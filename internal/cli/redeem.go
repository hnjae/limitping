// SPDX-FileCopyrightText: 2026 wavever
// SPDX-FileCopyrightText: 2026 KIM Hyunjae
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/hnjae/limitping/internal/config"
	"github.com/hnjae/limitping/internal/provider"
	"github.com/hnjae/limitping/internal/usage"
)

// newRedeemCmd spends a banked Codex reset credit. It is a separate, explicit
// command because redeeming is irreversible: `status` only ever reports the
// credits, and the automatic path (auto_redeem) is opt-in.
func newRedeemCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "redeem",
		Aliases: []string{"r"},
		Short:   "Spend a banked Codex rate-limit reset credit now",
		Long: `Consume one of the Codex reset credits shown by 'limitping status', resetting the rate-limit windows it is eligible for.

Redeeming is irreversible. The backend decides which credit to spend and refuses with "nothing to reset" when no window is currently eligible, so a credit is never burned for nothing.

Set auto_redeem = true under [codex] in the config to let 'watch' spend a credit on its own once it is close to expiring (within 24h with real usage to reclaim, or in its final hour).

Examples:
  limitping redeem --dry-run
  limitping redeem`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return runRedeem(cmd.Context(), cmd.OutOrStdout(), provider.NewCodex(cfg.Codex), dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show which credit would be spent without consuming it")
	return cmd
}

func runRedeem(ctx context.Context, out io.Writer, p *provider.Codex, dryRun bool) error {
	readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	u, err := p.ReadUsage(readCtx)
	cancel()
	if err != nil {
		return err
	}
	credit, ok := nextRedeemableCredit(u, time.Now())
	if !ok {
		return fmt.Errorf("%s", "no reset credits available to redeem")
	}

	// The expiry is unknown when only the count survived (see below).
	if !credit.ExpiresAt.IsZero() {
		expires := credit.ExpiresAt.Local()
		fmt.Fprintf(out, "codex   redeeming 1 reset credit (expires %s, in %s)\n",
			expires.Format(creditTimeLayout)+" "+fmtZone(expires),
			fmtDurDays(time.Until(credit.ExpiresAt)))
	}
	if dryRun {
		fmt.Fprint(out, "dry run: nothing was consumed\n")
		return nil
	}

	outcome, err := p.RedeemResetCredit(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "codex   %s\n", redeemOutcomeText(outcome))
	return nil
}

// nextRedeemableCredit returns the soonest-expiring credit that is still
// spendable, ignoring the auto-redeem timing policy: an explicit `redeem` is
// the user asking for it now.
func nextRedeemableCredit(u *usage.Usage, now time.Time) (usage.ResetCredit, bool) {
	if u.ResetCredits == nil {
		return usage.ResetCredit{}, false
	}
	var target usage.ResetCredit
	found := false
	for _, c := range u.ResetCredits.Credits {
		if !c.Redeemable(now) {
			continue
		}
		if !found || c.ExpiresAt.Before(target.ExpiresAt) {
			target, found = c, true
		}
	}
	// The detail endpoint is private and may go away, leaving only the count
	// from the usage response; trust that rather than refusing to redeem.
	if !found && u.ResetCredits.AvailableCount > 0 && len(u.ResetCredits.Credits) == 0 {
		return usage.ResetCredit{}, true
	}
	return target, found
}

func redeemOutcomeText(outcome string) string {
	switch outcome {
	case provider.RedeemReset:
		return "redeemed — the eligible rate-limit windows were reset"
	case provider.RedeemNothingToReset:
		return "no rate-limit window is currently eligible for a reset; the credit was not spent"
	case provider.RedeemNoCredit:
		return "the account has no reset credits available"
	case provider.RedeemAlreadyRedeemed:
		return "this redemption already completed earlier"
	default:
		return fmt.Sprintf("unexpected outcome from the backend: %s", outcome)
	}
}
