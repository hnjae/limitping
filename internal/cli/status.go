// SPDX-FileCopyrightText: 2026 wavever
// SPDX-FileCopyrightText: 2026 KIM Hyunjae
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hnjae/limitping/internal/config"
	"github.com/hnjae/limitping/internal/provider"
	"github.com/hnjae/limitping/internal/spend"
	"github.com/hnjae/limitping/internal/usage"
)

// spendTimeout caps the local transcript scan. It runs alongside the usage
// fetch, so it normally costs nothing in wall time; the cap is there so a huge
// or unreadable transcript history cannot hold up the whole command.
const spendTimeout = 20 * time.Second
const creditTimeLayout = "Jan 02 15:04"

func newStatusCmd() *cobra.Command {
	var verbose bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"s", "stat"},
		Short:   "Show current 5h/weekly usage and reset countdowns without using quota",
		Long: `Show current 5h and weekly usage for every enabled provider. This command only reads usage data from zero-quota endpoints; it does not send a ping or consume model quota.

The 'today' line totals the tokens this machine's Claude Code / Codex sessions have used since local midnight, read from the transcripts those CLIs write to disk, and prices them at published API rates — what the day would have cost without the subscription. Work done from another machine or from the web app is not in those logs. Add -v for the per-model breakdown.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			providers := enabledProviders(cfg)
			if len(providers) == 0 {
				return fmt.Errorf("no providers enabled in config")
			}
			return runStatus(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), providers, verbose, jsonOut, cfg.UsageDisplay)
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "print the raw JSON response")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output usage as JSON instead of text")
	return cmd
}

func runStatus(ctx context.Context, out, progress io.Writer, providers []provider.Provider, verbose, jsonOut bool, display string) error {
	if progress == nil {
		progress = io.Discard
	}
	display = normalizeUsageDisplay(display)
	// In JSON mode keep stdout a single valid document: suppress the
	// "Fetching..." progress chatter that would otherwise interleave.
	if jsonOut {
		progress = io.Discard
	}
	failed := 0
	entries := make([]statusJSON, 0, len(providers))
	for _, p := range providers {
		// Started first and collected last: reading the day's transcripts is
		// pure local I/O, so it rides along with the network round trip instead
		// of adding to it.
		spendCh := make(chan *spend.Day, 1)
		go func() { spendCh <- todaySpend(ctx, p.Name()) }()

		fmt.Fprintf(progress, "Fetching %s usage...\n", p.Name())
		readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		u, err := p.ReadUsage(readCtx)
		cancel()
		day := <-spendCh
		if err != nil {
			failed++
			if jsonOut {
				entries = append(entries, statusJSON{Provider: p.Name(), Error: err.Error()})
				continue
			}
			fmt.Fprintf(out, "%-7s  error: %v\n", p.Name(), err)
			continue
		}
		if jsonOut {
			entries = append(entries, newStatusJSON(u, verbose, day))
			continue
		}
		printUsage(out, u, verbose, display, day)
	}
	if jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			return err
		}
	}
	if failed > 0 {
		return fmt.Errorf("status failed for %d provider(s)", failed)
	}
	return nil
}

// statusJSON is the stable, documented shape emitted by `status --json`. It is
// decoupled from usage.Usage so the internal model can evolve without breaking
// scripts that consume this output.
type statusJSON struct {
	Provider     string            `json:"provider"`
	Plan         string            `json:"plan,omitempty"`
	FiveHour     *windowJSON       `json:"five_hour,omitempty"`
	Weekly       *windowJSON       `json:"weekly,omitempty"`
	Credits      *creditsJSON      `json:"credits,omitempty"`
	ResetCredits *resetCreditsJSON `json:"reset_credits,omitempty"`
	Today        *todayJSON        `json:"today,omitempty"`
	LimitReached bool              `json:"limit_reached"`
	FetchedAt    string            `json:"fetched_at,omitempty"`
	Raw          json.RawMessage   `json:"raw,omitempty"`
	Error        string            `json:"error,omitempty"`
}

type windowJSON struct {
	UsedPercent      float64 `json:"used_percent"`
	RemainingPercent float64 `json:"remaining_percent"`
	Active           bool    `json:"active"`
	ResetsAt         string  `json:"resets_at,omitempty"`
	RemainingSeconds int     `json:"remaining_seconds"`
	WindowSeconds    int     `json:"window_seconds,omitempty"`
}

type creditsJSON struct {
	HasCredits bool   `json:"has_credits"`
	Unlimited  bool   `json:"unlimited"`
	Balance    string `json:"balance,omitempty"`
}

type resetCreditsJSON struct {
	AvailableCount int               `json:"available_count"`
	Credits        []resetCreditJSON `json:"credits,omitempty"`
}

type resetCreditJSON struct {
	Status     string `json:"status,omitempty"`
	GrantedAt  string `json:"granted_at,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	RedeemedAt string `json:"redeemed_at,omitempty"`
}

// todayJSON is the local day's token consumption, read from the provider CLI's
// own transcripts. cost_usd is what those tokens would cost at API rates;
// cost_complete is false when a model that ran had no published rates, which
// makes cost_usd a lower bound.
type todayJSON struct {
	Date                string           `json:"date"`
	InputTokens         int              `json:"input_tokens"`
	CacheReadTokens     int              `json:"cache_read_tokens"`
	CacheCreationTokens int              `json:"cache_creation_tokens"`
	OutputTokens        int              `json:"output_tokens"`
	TotalTokens         int              `json:"total_tokens"`
	CostUSD             float64          `json:"cost_usd"`
	CostComplete        bool             `json:"cost_complete"`
	Models              []todayModelJSON `json:"models,omitempty"`
}

type todayModelJSON struct {
	Model       string  `json:"model,omitempty"`
	TotalTokens int     `json:"total_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}

func newStatusJSON(u *usage.Usage, verbose bool, day *spend.Day) statusJSON {
	s := statusJSON{
		Provider:     u.Provider,
		Plan:         u.Plan,
		LimitReached: u.LimitReached,
	}
	// A missing window means the provider does not currently enforce that
	// limit; drop the key rather than emit an all-zero window.
	if !u.FiveHour.Missing() {
		s.FiveHour = newWindowJSON(u.FiveHour)
	}
	if !u.Weekly.Missing() {
		s.Weekly = newWindowJSON(u.Weekly)
	}
	if !u.FetchedAt.IsZero() {
		s.FetchedAt = u.FetchedAt.Format(time.RFC3339)
	}
	if u.Credits != nil {
		s.Credits = &creditsJSON{
			HasCredits: u.Credits.HasCredits,
			Unlimited:  u.Credits.Unlimited,
			Balance:    u.Credits.Balance,
		}
	}
	if u.ResetCredits != nil {
		s.ResetCredits = newResetCreditsJSON(u.ResetCredits)
	}
	s.Today = newTodayJSON(day)
	if verbose && json.Valid(u.Raw) {
		s.Raw = json.RawMessage(u.Raw)
	}
	return s
}

func newWindowJSON(w usage.Window) *windowJSON {
	j := &windowJSON{
		UsedPercent:      w.UsedPercent,
		RemainingPercent: remainingPercent(w.UsedPercent),
		Active:           w.Active(),
		RemainingSeconds: int(w.Remaining().Seconds()),
		WindowSeconds:    w.WindowSeconds,
	}
	if !w.ResetsAt.IsZero() {
		j.ResetsAt = w.ResetsAt.Format(time.RFC3339)
	}
	return j
}

func newResetCreditsJSON(rc *usage.ResetCredits) *resetCreditsJSON {
	out := &resetCreditsJSON{
		AvailableCount: rc.AvailableCount,
		Credits:        make([]resetCreditJSON, 0, len(rc.Credits)),
	}
	for _, c := range rc.Credits {
		out.Credits = append(out.Credits, resetCreditJSON{
			Status:     c.Status,
			GrantedAt:  timeJSON(c.GrantedAt),
			ExpiresAt:  timeJSON(c.ExpiresAt),
			RedeemedAt: timeJSON(c.RedeemedAt),
		})
	}
	return out
}

// newTodayJSON renders the day's spend, or nothing at all when the provider's
// CLI has never run on this machine — an absent key says "no local data", which
// zeros would misreport as "nothing was spent".
func newTodayJSON(day *spend.Day) *todayJSON {
	if day == nil || !day.Available {
		return nil
	}
	out := &todayJSON{
		Date:                day.Date.Format("2006-01-02"),
		InputTokens:         day.Tokens.Input,
		CacheReadTokens:     day.Tokens.CacheRead,
		CacheCreationTokens: day.Tokens.CacheWrite,
		OutputTokens:        day.Tokens.Output,
		TotalTokens:         day.Tokens.Total(),
		CostUSD:             roundUSD(day.CostUSD),
		CostComplete:        day.Priced,
	}
	for _, m := range day.Models {
		out.Models = append(out.Models, todayModelJSON{
			Model:       m.Model,
			TotalTokens: m.Tokens.Total(),
			CostUSD:     roundUSD(m.CostUSD),
		})
	}
	return out
}

// roundUSD trims the float noise (0.30000000000000004) that summing per-model
// costs leaves behind, at a precision finer than any real per-day total needs.
func roundUSD(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

func timeJSON(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func printUsage(out io.Writer, u *usage.Usage, verbose bool, display string, day *spend.Day) {
	display = normalizeUsageDisplay(display)
	plan := u.Plan
	if plan != "" {
		plan = " (" + plan + ")"
	}
	fmt.Fprintf(out, "%s%s\n", u.Provider, plan)
	fmt.Fprintf(out, "  5h     %s\n", fmtWindow(u.FiveHour, display))
	fmt.Fprintf(out, "  weekly %s\n", fmtWindow(u.Weekly, display))
	printToday(out, day, verbose)
	if u.Credits != nil && (u.Credits.HasCredits || u.Credits.Unlimited) {
		if u.Credits.Unlimited {
			fmt.Fprint(out, "  credits unlimited\n")
		} else {
			fmt.Fprintf(out, "  credits %s\n", u.Credits.Balance)
		}
	}
	printResetCredits(out, u.ResetCredits)
	if verbose {
		fmt.Fprintf(out, "  raw: %s\n", string(u.Raw))
	}
	fmt.Fprintln(out)
}

// todaySpend reads what the provider's local CLI sessions consumed today. It is
// a best-effort extra: a transcript that cannot be read costs the line, never
// the status command.
func todaySpend(ctx context.Context, name string) *spend.Day {
	ctx, cancel := context.WithTimeout(ctx, spendTimeout)
	defer cancel()
	day, err := spend.Today(ctx, name)
	if err != nil && day.Empty() {
		return nil
	}
	return &day
}

// printToday renders the day's token consumption and what it would have cost at
// API rates — the usage endpoints report percentages only, so this is the one
// place a subscription's actual consumption becomes a number. Nothing is
// printed for a provider whose CLI has never run on this machine: silence is
// honest there, while "0 tok" would claim a quiet day.
func printToday(out io.Writer, day *spend.Day, verbose bool) {
	if day == nil || !day.Available {
		return
	}
	fmt.Fprintf(out, "  today  %s\n", fmtSpend(day.Tokens.Total(), day.CostUSD))
	if !verbose || day.Empty() {
		return
	}
	fmt.Fprintf(out, "         in %s · cache %s read / %s write · out %s\n",
		humanTokens(day.Tokens.Input), humanTokens(day.Tokens.CacheRead),
		humanTokens(day.Tokens.CacheWrite), humanTokens(day.Tokens.Output))
	for _, m := range day.Models {
		name := m.Model
		if name == "" {
			name = "unknown model"
		}
		fmt.Fprintf(out, "         %-26s %s\n", name, fmtSpend(m.Tokens.Total(), m.CostUSD))
	}
}

// fmtSpend renders "47.9M tok  ≈ $38.15", dropping the cost when the model's
// rates are unknown (an unpublished or brand-new model).
func fmtSpend(tokens int, costUSD float64) string {
	s := fmt.Sprintf("%s tok", humanTokens(tokens))
	if costUSD > 0 {
		s += fmt.Sprintf("  ≈ $%s", fmtUSD(costUSD))
	}
	return s
}

// humanTokens keeps the status line scannable: exact below 10k, where the digits
// are still readable, and abbreviated above it. `--json` carries exact counts.
func humanTokens(n int) string {
	switch {
	case n < 10_000:
		return humanInt(n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	case n < 1_000_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	default:
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	}
}

// fmtUSD shows cents, falling back to four decimals for the sums too small to
// register in them.
func fmtUSD(v float64) string {
	if v < 0.01 {
		return fmt.Sprintf("%.4f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func printResetCredits(out io.Writer, rc *usage.ResetCredits) {
	if rc == nil || (rc.AvailableCount == 0 && len(rc.Credits) == 0) {
		return
	}
	countFmt := "  reset credits %d resets available\n"
	if rc.AvailableCount == 1 {
		countFmt = "  reset credits %d reset available\n"
	}
	fmt.Fprintf(out, countFmt, rc.AvailableCount)
	for _, c := range rc.Credits {
		line := resetCreditLine(c)
		if line != "" {
			fmt.Fprintf(out, "    - %s\n", line)
		}
	}
}

func resetCreditLine(c usage.ResetCredit) string {
	status := c.Status
	if status == "" {
		switch {
		case !c.RedeemedAt.IsZero():
			status = "redeemed"
		case !c.ExpiresAt.IsZero() && time.Now().After(c.ExpiresAt):
			status = "expired"
		default:
			status = "available"
		}
	}
	parts := []string{status}
	if !c.GrantedAt.IsZero() {
		parts = append(parts, fmt.Sprintf("granted %s", c.GrantedAt.Local().Format(creditTimeLayout)))
	}
	if !c.ExpiresAt.IsZero() {
		// The zone is stated on the expiry — the one date on this line that is a
		// deadline to act on — and carries the line's other stamps with it.
		expires := c.ExpiresAt.Local()
		part := fmt.Sprintf("expires %s", expires.Format(creditTimeLayout)+" "+fmtZone(expires))
		// Remaining lifetime, so an unredeemed credit about to lapse is
		// visible at a glance. Meaningless once redeemed or expired.
		if remaining := time.Until(c.ExpiresAt); remaining > 0 && c.RedeemedAt.IsZero() {
			part += fmt.Sprintf(" (in %s)", fmtDurDays(remaining))
		}
		parts = append(parts, part)
	}
	if !c.RedeemedAt.IsZero() {
		parts = append(parts, fmt.Sprintf("redeemed %s", c.RedeemedAt.Local().Format(creditTimeLayout)))
	}
	return strings.Join(parts, ", ")
}

func fmtWindow(w usage.Window, display string) string {
	if w.Missing() {
		return "not currently enforced"
	}
	display = normalizeUsageDisplay(display)
	pct := displayedPercent(w, display)
	bar := usageBar(pct)
	word := "used"
	if display == "remaining" {
		word = "remaining"
	}
	if w.ResetsAt.IsZero() {
		return fmt.Sprintf("%s %5.1f%% %-9s (no active window)", bar, pct, word)
	}
	return fmt.Sprintf("%s %5.1f%% %-9s resets in %-8s (%s)",
		bar, pct, word, fmtDur(w.Remaining()), fmtClock(w.ResetsAt))
}

// fmtClock renders the reset wall-clock time and its UTC offset.
func fmtClock(t time.Time) string {
	lt := t.Local()
	return lt.Format("Mon 15:04") + " " + fmtZone(lt)
}

// fmtZone renders t's UTC offset (UTC+8, UTC-5:30, UTC). The offset is used
// rather than the zone abbreviation because abbreviations are ambiguous — CST
// is both China Standard Time and US Central Standard Time.
func fmtZone(t time.Time) string {
	_, offset := t.Zone()
	if offset == 0 {
		return "UTC"
	}
	sign := "+"
	if offset < 0 {
		sign, offset = "-", -offset
	}
	if minutes := offset % 3600 / 60; minutes != 0 {
		return fmt.Sprintf("UTC%s%d:%02d", sign, offset/3600, minutes)
	}
	return fmt.Sprintf("UTC%s%d", sign, offset/3600)
}

func normalizeUsageDisplay(display string) string {
	if display == "remaining" {
		return "remaining"
	}
	return "used"
}

func displayedPercent(w usage.Window, display string) float64 {
	if normalizeUsageDisplay(display) == "remaining" {
		return remainingPercent(w.UsedPercent)
	}
	return w.UsedPercent
}

func remainingPercent(used float64) float64 {
	remaining := 100 - used
	if remaining < 0 {
		return 0
	}
	if remaining > 100 {
		return 100
	}
	return remaining
}

func usageBar(pct float64) string {
	const width = 10
	filled := int(pct/100*width + 0.5)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	b := make([]rune, width)
	for i := range b {
		if i < filled {
			b[i] = '█'
		} else {
			b[i] = '░'
		}
	}
	return "[" + string(b) + "]"
}

// fmtDurDays renders long spans with a day component (e.g. 11d16h) — reset
// credits live for 30 days, where pure hours would be unreadable — and falls
// back to fmtDur below one day.
func fmtDurDays(d time.Duration) string {
	if d < 24*time.Hour {
		return fmtDur(d)
	}
	days := int(d / (24 * time.Hour))
	hours := int(d % (24 * time.Hour) / time.Hour)
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, hours)
}

func fmtDur(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	d = d.Round(time.Minute)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
