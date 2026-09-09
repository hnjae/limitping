package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/creack/pty"

	"github.com/wavever/CCLimitPing/internal/activity"
	"github.com/wavever/CCLimitPing/internal/auth"
	"github.com/wavever/CCLimitPing/internal/config"
	"github.com/wavever/CCLimitPing/internal/usage"
)

const (
	codexDefaultBaseURL = "https://chatgpt.com/backend-api"
	codexChatGPTPath    = "/wham/usage"
	codexResetPath      = "/wham/rate-limit-reset-credits"
	codexConsumePath    = "/wham/rate-limit-reset-credits/consume"
	codexAPIPath        = "/api/codex/usage"
	codexUserAgent      = "limitping"

	// codexRedeemCooldown throttles the automatic redemption path so a
	// once-a-minute poll loop cannot re-attempt a refused redemption every cycle.
	codexRedeemCooldown = 15 * time.Minute

	// The PTY needs a plausible size for the Codex TUI to lay out and render at
	// all; the exact numbers only have to be big enough to be a believable
	// terminal, since nothing reads the rendering back.
	codexPTYRows = 40
	codexPTYCols = 120

	codexTurnMinWait  = 4 * time.Second
	codexTurnQuiet    = 2500 * time.Millisecond
	codexTurnMaxWait  = 45 * time.Second
	codexExitGrace    = 5 * time.Second
	codexPollInterval = 200 * time.Millisecond
)

// Codex reads usage via the ChatGPT backend usage endpoint and triggers windows
// via the interactive, TTY-backed Codex CLI. Headless `codex exec` can consume
// tokens without anchoring the subscription-backed Codex window.
type Codex struct {
	cfg  config.ProviderConfig
	auth *auth.CodexAuth

	redeemMu   sync.Mutex
	lastRedeem time.Time // last automatic redemption attempt, for the cooldown
}

func NewCodex(cfg config.ProviderConfig) *Codex {
	return &Codex{
		cfg:  cfg,
		auth: auth.NewCodexAuth(),
	}
}

func (c *Codex) Name() string { return "codex" }

func (c *Codex) ActiveTask(ctx context.Context) (string, bool, error) {
	return codexActiveTask(ctx)
}

func (c *Codex) ReadUsage(ctx context.Context) (*usage.Usage, error) {
	body, r, err := readCodexUsage(ctx, c.auth)
	if err != nil {
		return nil, err
	}
	u := codexUsageToUsage(c.Name(), body, r)
	if credits, err := readCodexResetCredits(ctx, c.auth); err == nil {
		u.ResetCredits = credits
	} else if r.ResetCredits != nil {
		// The detail endpoint is private and may go away; the usage response
		// itself now embeds the available count, so keep at least that.
		u.ResetCredits = &usage.ResetCredits{AvailableCount: r.ResetCredits.AvailableCount}
	}
	return u, nil
}

func (c *Codex) Trigger(ctx context.Context, dryRun bool) (*TriggerResult, error) {
	return triggerCodex(ctx, c.cfg, dryRun)
}

// RedeemResetCredit spends the next available reset credit right now. Each call
// is a distinct attempt, so it carries a fresh idempotency key.
func (c *Codex) RedeemResetCredit(ctx context.Context) (string, error) {
	return c.consumeResetCredit(ctx, randomIdempotencyKey())
}

// AutoRedeemResetCredit spends a credit that is about to lapse, at most once per
// codexRedeemCooldown. The key is derived from the credit itself, so an attempt
// whose response was lost in flight is retried — after the cooldown — under the
// same key and cannot spend a second credit.
func (c *Codex) AutoRedeemResetCredit(ctx context.Context, u *usage.Usage) (string, error) {
	credit, ok := u.ResetCreditToRedeem(time.Now())
	if !ok {
		return "", nil
	}
	c.redeemMu.Lock()
	if time.Since(c.lastRedeem) < codexRedeemCooldown {
		c.redeemMu.Unlock()
		return "", nil
	}
	c.lastRedeem = time.Now()
	c.redeemMu.Unlock()
	return c.consumeResetCredit(ctx, creditIdempotencyKey(credit))
}

// consumeResetCredit redeems one banked reset credit. The credit id is
// deliberately omitted: the backend then picks the next available credit — the
// same one the policy targets — so we don't depend on an id field this private
// endpoint doesn't document.
func (c *Codex) consumeResetCredit(ctx context.Context, idempotencyKey string) (string, error) {
	payload, err := json.Marshal(map[string]string{"idempotency_key": idempotencyKey})
	if err != nil {
		return "", err
	}
	accountID, _ := c.auth.AccountID(ctx)
	body, err := fetchWithAuth(ctx, c.auth, func(token string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexConsumeURL(), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", codexUserAgent)
		req.Header.Set("OpenAI-Beta", "codex-1")
		req.Header.Set("originator", "Codex Desktop")
		if accountID != "" {
			req.Header.Set("ChatGPT-Account-Id", accountID)
		}
		return req, nil
	})
	if err != nil {
		return "", fmt.Errorf("codex reset credit consume: %w", err)
	}
	var r struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("codex reset credit consume: parsing response: %w", err)
	}
	if r.Code == "" {
		return "", fmt.Errorf("codex reset credit consume: no outcome in response: %s", truncate(body, 200))
	}
	return normalizeRedeemOutcome(r.Code), nil
}

// normalizeRedeemOutcome folds the two spellings of the same outcomes into the
// snake_case form we report: the private endpoint answers in snake_case, while
// Codex's app-server protocol spells them in camelCase. Unknown codes pass
// through untouched rather than being reported as a success.
func normalizeRedeemOutcome(code string) string {
	switch code {
	case "nothingToReset":
		return RedeemNothingToReset
	case "noCredit":
		return RedeemNoCredit
	case "alreadyRedeemed":
		return RedeemAlreadyRedeemed
	default:
		return code
	}
}

func randomIdempotencyKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("limitping-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// creditIdempotencyKey derives a stable key from the credit being spent, so the
// same credit always maps to the same logical attempt.
func creditIdempotencyKey(c usage.ResetCredit) string {
	sum := sha256.Sum256([]byte("limitping-reset-credit|" + c.ExpiresAt.UTC().Format(time.RFC3339)))
	return hex.EncodeToString(sum[:16])
}

func codexActiveTask(_ context.Context) (string, bool, error) {
	// Active-session detection relies entirely on the Codex CLI hooks (see
	// `limitping hooks install`). Without them we don't guess from the process
	// list; the scheduler just pings.
	if !activity.Enabled("codex") {
		return "", false, nil
	}
	return activity.Active("codex")
}

type codexWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int     `json:"limit_window_seconds"`
	ResetAt            int64   `json:"reset_at"`
}

// The windows are pointers because the backend nulls out a window when that
// limit is not currently enforced. OpenAI did exactly that between 2026-07-12
// and (at the latest) 2026-09-09, when the 5h limit was gone and primary_window
// carried the weekly one — hence codexWindowsFromRateLimit classifying by
// length rather than by position.
type codexRateLimit struct {
	Allowed      bool         `json:"allowed"`
	LimitReached bool         `json:"limit_reached"`
	Primary      *codexWindow `json:"primary_window"`
	Secondary    *codexWindow `json:"secondary_window"`
}

type codexCredits struct {
	HasCredits bool   `json:"has_credits"`
	Unlimited  bool   `json:"unlimited"`
	Balance    string `json:"balance"`
}

type codexUsageResp struct {
	PlanType     string                   `json:"plan_type"`
	RateLimit    codexRateLimit           `json:"rate_limit"`
	Credits      *codexCredits            `json:"credits"`
	ResetCredits *codexInlineResetCredits `json:"rate_limit_reset_credits"`
}

// codexInlineResetCredits is the reset-credit count embedded in the usage
// response itself; a fallback when the detail endpoint is unavailable.
type codexInlineResetCredits struct {
	AvailableCount int `json:"available_count"`
}

type codexResetCreditsResp struct {
	AvailableCount *int               `json:"available_count"`
	Credits        []codexResetCredit `json:"credits"`
}

type codexResetCredit struct {
	Status     string `json:"status"`
	GrantedAt  string `json:"granted_at"`
	ExpiresAt  string `json:"expires_at"`
	RedeemedAt string `json:"redeemed_at"`
}

func readCodexUsage(ctx context.Context, auth *auth.CodexAuth) ([]byte, codexUsageResp, error) {
	var r codexUsageResp
	accountID, _ := auth.AccountID(ctx)
	body, err := fetchWithAuth(ctx, auth, func(token string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, codexUsageURL(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", codexUserAgent)
		if accountID != "" {
			req.Header.Set("ChatGPT-Account-Id", accountID)
		}
		return req, nil
	})
	if err != nil {
		return nil, r, err
	}

	if err := json.Unmarshal(body, &r); err != nil {
		return nil, r, fmt.Errorf("codex usage: parsing response: %w", err)
	}
	return body, r, nil
}

func readCodexResetCredits(ctx context.Context, auth *auth.CodexAuth) (*usage.ResetCredits, error) {
	var r codexResetCreditsResp
	accountID, _ := auth.AccountID(ctx)
	body, err := fetchWithAuth(ctx, auth, func(token string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, codexResetCreditsURL(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", codexUserAgent)
		req.Header.Set("OpenAI-Beta", "codex-1")
		req.Header.Set("originator", "Codex Desktop")
		if accountID != "" {
			req.Header.Set("ChatGPT-Account-Id", accountID)
		}
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("codex reset credits: parsing response: %w", err)
	}
	return codexResetCreditsToUsage(r), nil
}

func codexUsageToUsage(provider string, body []byte, r codexUsageResp) *usage.Usage {
	fiveHour, weekly := codexWindowsFromRateLimit(r.RateLimit)
	u := &usage.Usage{
		Provider:     provider,
		Plan:         r.PlanType,
		FetchedAt:    time.Now(),
		Raw:          body,
		LimitReached: r.RateLimit.LimitReached,
		FiveHour:     fiveHour,
		Weekly:       weekly,
	}
	if r.Credits != nil {
		u.Credits = &usage.Credits{
			HasCredits: r.Credits.HasCredits,
			Unlimited:  r.Credits.Unlimited,
			Balance:    r.Credits.Balance,
		}
	}
	return u
}

func codexResetCreditsToUsage(r codexResetCreditsResp) *usage.ResetCredits {
	credits := make([]usage.ResetCredit, 0, len(r.Credits))
	for _, c := range r.Credits {
		credits = append(credits, usage.ResetCredit{
			Status:     c.Status,
			GrantedAt:  parseCodexResetTime(c.GrantedAt),
			ExpiresAt:  parseCodexResetTime(c.ExpiresAt),
			RedeemedAt: parseCodexResetTime(c.RedeemedAt),
		})
	}
	count := len(credits)
	if r.AvailableCount != nil {
		count = *r.AvailableCount
	}
	return &usage.ResetCredits{
		AvailableCount: count,
		Credits:        credits,
	}
}

func parseCodexResetTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

func codexUsageURL() string {
	base := codexDefaultBaseURL
	if contents, err := os.ReadFile(codexConfigPath()); err == nil {
		if configured := parseCodexBaseURL(string(contents)); configured != "" {
			base = configured
		}
	}
	return codexUsageURLFromBase(base)
}

func codexResetCreditsURL() string {
	base := codexDefaultBaseURL
	if contents, err := os.ReadFile(codexConfigPath()); err == nil {
		if configured := parseCodexBaseURL(string(contents)); configured != "" {
			base = configured
		}
	}
	return codexResetCreditsURLFromBase(base)
}

func codexUsageURLFromBase(base string) string {
	normalized := normalizeCodexBaseURL(base)
	path := codexAPIPath
	if strings.Contains(normalized, "/backend-api") {
		path = codexChatGPTPath
	}
	endpoint := normalized + path
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return codexDefaultBaseURL + codexChatGPTPath
	}
	return endpoint
}

func codexResetCreditsURLFromBase(base string) string {
	return codexResetURLFromBase(base, codexResetPath)
}

func codexConsumeURL() string {
	base := codexDefaultBaseURL
	if contents, err := os.ReadFile(codexConfigPath()); err == nil {
		if configured := parseCodexBaseURL(string(contents)); configured != "" {
			base = configured
		}
	}
	return codexResetURLFromBase(base, codexConsumePath)
}

// codexResetURLFromBase builds a reset-credit endpoint. These live only on the
// ChatGPT backend, so a base pointing elsewhere falls back to the default.
func codexResetURLFromBase(base, path string) string {
	normalized := normalizeCodexBaseURL(base)
	if !strings.Contains(normalized, "/backend-api") {
		normalized = codexDefaultBaseURL
	}
	endpoint := normalized + path
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return codexDefaultBaseURL + path
	}
	return endpoint
}

func normalizeCodexBaseURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = codexDefaultBaseURL
	}
	base = strings.TrimRight(base, "/")
	if (strings.HasPrefix(base, "https://chatgpt.com") || strings.HasPrefix(base, "https://chat.openai.com")) &&
		!strings.Contains(base, "/backend-api") {
		base += "/backend-api"
	}
	return base
}

func parseCodexBaseURL(contents string) string {
	var cfg struct {
		ChatGPTBaseURL string `toml:"chatgpt_base_url"`
	}
	if _, err := toml.Decode(contents, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.ChatGPTBaseURL)
}

// codexPingModel resolves the model to pass to `codex -m`. An unset config
// means "spend as little as possible", not "use my Codex working model": the
// cheapest catalogued model is picked explicitly. Empty means the catalog could
// not answer, and the CLI is left to choose as before.
func codexPingModel(configured string) string {
	if configured != "" {
		return configured
	}
	return codexCheapestModel()
}

// codexCLIConfiguredModel is the model set in the Codex CLI's own config, used
// only to report what an un-pinned ping will run on. It is never chosen: a
// working model is typically a much more expensive tier than a ping needs.
func codexCLIConfiguredModel() string {
	contents, err := os.ReadFile(codexConfigPath())
	if err != nil {
		return ""
	}
	return parseCodexModel(string(contents))
}

func parseCodexModel(contents string) string {
	var cfg struct {
		Model string `toml:"model"`
	}
	if _, err := toml.Decode(contents, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Model)
}

func codexConfigPath() string {
	if h := os.Getenv("CODEX_HOME"); h != "" {
		return filepath.Join(h, "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".codex", "config.toml")
	}
	return filepath.Join(home, ".codex", "config.toml")
}

func codexModelsCachePath() string {
	return filepath.Join(filepath.Dir(codexConfigPath()), "models_cache.json")
}

// checkCodexModel rejects a configured model the Codex CLI's own catalog no
// longer lists. OpenAI retires Codex models every few months and `codex -m`
// is not validated locally, so without this a stale config fails as an opaque
// server-side error at window rollover — precisely when nobody is watching.
// The catalog is a private Codex file: any problem reading it skips the check
// rather than blocking a ping that would otherwise have worked.
func checkCodexModel(model string) error {
	if model == "" {
		return nil // resolved from the catalog, so never stale
	}
	catalog := codexModelCatalog()
	if len(catalog) == 0 {
		return nil
	}
	var available []string
	for _, m := range catalog {
		if m.Slug == model {
			return nil
		}
		if m.Listed {
			available = append(available, m.Slug)
		}
	}
	if len(available) == 0 {
		for _, m := range catalog {
			available = append(available, m.Slug)
		}
	}
	return fmt.Errorf("codex model %q is no longer in the Codex model catalog (%s); available: %s — update model under [codex] in limitping's config, or set it to \"\" to let limitping pick the cheapest one",
		model, codexModelsCachePath(), strings.Join(available, ", "))
}

// codexCatalogModel is one entry of the Codex CLI's cached model catalog.
// Listed reflects visibility: the catalog hides internal models such as
// codex-auto-review, which are valid to pass but must never be chosen or
// suggested on the user's behalf.
type codexCatalogModel struct {
	Slug        string
	Description string
	Priority    int
	Listed      bool
}

// codexModelCatalog reads the models the Codex CLI last cached for this
// account. It returns nil when the cache is missing or unparseable.
func codexModelCatalog() []codexCatalogModel {
	data, err := os.ReadFile(codexModelsCachePath())
	if err != nil {
		return nil
	}
	var cache struct {
		Models []struct {
			Slug        string `json:"slug"`
			Description string `json:"description"`
			Visibility  string `json:"visibility"`
			Priority    int    `json:"priority"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}
	models := make([]codexCatalogModel, 0, len(cache.Models))
	for _, m := range cache.Models {
		if m.Slug == "" {
			continue
		}
		models = append(models, codexCatalogModel{
			Slug:        m.Slug,
			Description: m.Description,
			Priority:    m.Priority,
			Listed:      m.Visibility == "list",
		})
	}
	return models
}

// codexBudgetMarkers are the words OpenAI uses for its low-cost tier, matched
// against a model's description and slug. The catalog carries no price field,
// so this wording is the only cost signal it exposes ("Fast and affordable
// agentic coding model"), alongside the "mini"-style naming used for budget
// variants.
var codexBudgetMarkers = []string{"affordable", "cheap", "low cost", "low-cost", "mini"}

// codexCheapestModel picks the model a ping should use when none is configured.
// A ping only has to be a billable request — the model does not matter — so it
// should land on the cheapest one the plan offers rather than on whatever the
// user set as their working model in the Codex CLI, which is typically a far
// more expensive tier.
//
// Ties break toward the largest priority, i.e. the entry Codex itself ranks
// furthest from its flagship. Returns "" when the catalog is unreadable or
// nothing is recognizably the budget tier: guessing a model on price wording
// that no longer exists would be worse than letting the CLI decide.
func codexCheapestModel() string {
	best := codexCatalogModel{Priority: -1}
	for _, m := range codexModelCatalog() {
		if !m.Listed || !codexIsBudgetModel(m) {
			continue
		}
		// Largest priority wins; slug breaks an exact tie so the choice is
		// stable across catalog orderings.
		if m.Priority > best.Priority || (m.Priority == best.Priority && m.Slug < best.Slug) {
			best = m
		}
	}
	return best.Slug
}

func codexIsBudgetModel(m codexCatalogModel) bool {
	haystack := strings.ToLower(m.Description + " " + m.Slug)
	for _, marker := range codexBudgetMarkers {
		if strings.Contains(haystack, marker) {
			return true
		}
	}
	return false
}

// codexWindowsFromRateLimit classifies the windows by length rather than
// position. Historically primary was the 5h window and secondary the weekly
// one, but with the 5h limit removed the weekly window is the (only) primary,
// so position no longer identifies a window. A window a couple of days or
// longer is the weekly one; anything shorter is the 5h one. A limit whose
// window is absent stays the zero Window (usage.Window.Missing).
func codexWindowsFromRateLimit(rl codexRateLimit) (fiveHour, weekly usage.Window) {
	const weeklyMinSeconds = 2 * 24 * 60 * 60
	for _, w := range []*codexWindow{rl.Primary, rl.Secondary} {
		if w == nil {
			continue
		}
		if w.LimitWindowSeconds >= weeklyMinSeconds {
			weekly = codexWindowToUsage(*w)
		} else {
			fiveHour = codexWindowToUsage(*w)
		}
	}
	return fiveHour, weekly
}

func codexWindowToUsage(w codexWindow) usage.Window {
	var resetsAt time.Time
	if w.ResetAt > 0 {
		resetsAt = time.Unix(w.ResetAt, 0)
	}
	return usage.Window{
		UsedPercent:   w.UsedPercent,
		ResetsAt:      resetsAt,
		WindowSeconds: w.LimitWindowSeconds,
	}
}

func triggerCodex(ctx context.Context, cfg config.ProviderConfig, dryRun bool) (*TriggerResult, error) {
	prompt := cfg.Prompt
	if prompt == "" {
		prompt = "ok"
	}
	// A ping is a synthetic session, so the user's Codex hooks have no business
	// running for it — and leaving them on is not merely untidy: an unreviewed
	// hook makes the TUI open on a blocking "Hooks need review" prompt that no
	// keystroke of ours answers, so the ping silently never submits anything.
	args := []string{"--disable", "hooks"}
	if cfg.ReasoningEffort != "" {
		args = append(args, "-c", "model_reasoning_effort="+cfg.ReasoningEffort)
	}
	model := codexPingModel(cfg.Model)
	if model != "" {
		args = append(args, "-m", model)
	}
	args = append(args, codexInteractiveArgs(cfg.ExtraArgs)...)
	args = append(args, prompt)
	reported := model
	if reported == "" {
		// Nothing was pinned, so the CLI picks; report its choice rather than
		// leaving the ping silent about what it spent quota on.
		reported = codexCLIConfiguredModel()
	}
	res := &TriggerResult{
		Command: "codex " + shellJoin(args),
		Model:   reported,
	}
	// Checked before the dry-run return too: a dry run that prints a command
	// which cannot succeed is worse than no dry run.
	if err := checkCodexModel(cfg.Model); err != nil {
		return res, err
	}
	if dryRun {
		return res, nil
	}

	cmd := exec.CommandContext(ctx, "codex", args...)
	// pty.Start would hand the child a 0x0 terminal. Claude Code tolerates that,
	// but the Codex TUI draws nothing into a zero-sized viewport: it emits its
	// terminal-capability queries and then sits there forever, so the prompt is
	// never submitted, no request is dispatched, and the window never starts —
	// while the session still exits cleanly and reads as a successful ping.
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: codexPTYRows, Cols: codexPTYCols})
	if err != nil {
		return res, fmt.Errorf("codex interactive failed to start: %w", err)
	}
	defer ptmx.Close()

	output := &limitedBuffer{limit: 4096}
	go func() {
		_, _ = io.Copy(output, ptmx)
	}()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	if terminal, err := codexAwait(ctx, cmd, ptmx, output, done, codexTurnMaxWait,
		func(idle, elapsed time.Duration) bool {
			return elapsed >= codexTurnMinWait && idle >= codexTurnQuiet
		}); terminal {
		return res, err
	}

	return res, codexInteractiveStop(ctx, cmd, ptmx, done, output)
}

func codexAwait(ctx context.Context, cmd *exec.Cmd, ptmx *os.File, output *limitedBuffer, done <-chan error, maxWait time.Duration, ready func(idle, elapsed time.Duration) bool) (bool, error) {
	start := time.Now()
	deadline := time.After(maxWait)
	ticker := time.NewTicker(codexPollInterval)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return true, codexInteractiveErr(err, output)
		case <-ctx.Done():
			return true, codexInteractiveCancel(ctx, cmd, ptmx, done, output)
		case <-deadline:
			return false, nil
		case <-ticker.C:
			changed := output.changedAt()
			if !changed.IsZero() && ready(time.Since(changed), time.Since(start)) {
				return false, nil
			}
		}
	}
}

func codexInteractiveStop(ctx context.Context, cmd *exec.Cmd, ptmx *os.File, done <-chan error, output *limitedBuffer) error {
	deadline := time.After(codexExitGrace)
	ticker := time.NewTicker(codexExitGrace / 2)
	defer ticker.Stop()

	for sent := false; ; {
		if !sent {
			_, _ = ptmx.Write([]byte{0x03})
			sent = true
		}
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return codexInteractiveCancel(ctx, cmd, ptmx, done, output)
		case <-ticker.C:
			_, _ = ptmx.Write([]byte{0x03})
		case <-deadline:
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return nil
		}
	}
}

func codexInteractiveErr(err error, output *limitedBuffer) error {
	if err == nil {
		return nil
	}
	tail := truncate(output.Bytes(), 300)
	if tail == "" {
		return fmt.Errorf("codex interactive failed: %w", err)
	}
	return fmt.Errorf("codex interactive failed: %w: %s", err, tail)
}

func codexInteractiveCancel(ctx context.Context, cmd *exec.Cmd, ptmx *os.File, done <-chan error, output *limitedBuffer) error {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	_ = ptmx.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
	}
	tail := truncate(output.Bytes(), 300)
	if tail == "" {
		return fmt.Errorf("codex interactive cancelled: %w", ctx.Err())
	}
	return fmt.Errorf("codex interactive cancelled: %w: %s", ctx.Err(), tail)
}

func codexInteractiveArgs(extra []string) []string {
	out := make([]string, 0, len(extra))
	for i := 0; i < len(extra); i++ {
		arg := extra[i]
		flag, inlineValue := splitFlagValue(arg)
		if codexInteractiveUnsupportedValueArg(flag) {
			if !inlineValue && i+1 < len(extra) {
				i++
			}
			continue
		}
		if codexInteractiveUnsupportedArg(flag) {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func codexInteractiveUnsupportedArg(flag string) bool {
	switch flag {
	case "--skip-git-repo-check", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--json":
		return true
	default:
		return false
	}
}

func codexInteractiveUnsupportedValueArg(flag string) bool {
	switch flag {
	case "--output-schema", "--output-last-message", "--color", "-o":
		return true
	default:
		return false
	}
}
