// SPDX-FileCopyrightText: 2026 wavever
// SPDX-FileCopyrightText: 2026 KIM Hyunjae
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/hnjae/limitping/internal/config"
	"github.com/hnjae/limitping/internal/scheduler"
)

func newWatchCmd() *cobra.Command {
	var dryRun bool
	var live bool
	cmd := &cobra.Command{
		Use:     "watch [provider]",
		Aliases: []string{"w"},
		Short:   "Run the foreground daemon and ping each provider when its 5h window resets",
		Long: `Run the foreground daemon. When a provider's 5h window resets, limitping sends the minimal message to start the next window.

Arguments:
  provider  Optional. One of: claude, codex, all.
            Defaults to all, which watches every enabled provider.

Codex reset credits: set auto_redeem = true under [codex] in the config and watch also spends a banked reset credit that is about to lapse — within 24h when there is usage worth reclaiming, or in its final hour. Off by default because redeeming is irreversible; 'limitping redeem' spends one by hand.

Examples:
  limitping watch
  limitping w claude
  limitping watch --live
  limitping watch --dry-run`,
		Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"claude", "codex", "all"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			name := "all"
			if len(args) > 0 {
				name = args[0]
			}
			targets, err := selectTargets(cfg, name)
			if err != nil {
				return err
			}
			release, err := acquireWatchLock(name, dryRun)
			if err != nil {
				return err
			}
			defer release()

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			s := scheduler.New(cfg, targets, dryRun, live, cmd.OutOrStdout())
			s.Run(ctx)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "log when pings would fire without sending them")
	cmd.Flags().BoolVar(&live, "live", false, "show a live heartbeat/status line while watching (uses more power)")
	return cmd
}
