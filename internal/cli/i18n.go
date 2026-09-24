// SPDX-FileCopyrightText: 2026 wavever
// SPDX-FileCopyrightText: 2026 KIM Hyunjae
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

type cliText struct {
	rootShort     string
	rootLong      string
	helpFlag      string
	usageTemplate string

	helpCommandShort string
	helpCommandLong  string
	helpUnknownTopic string

	completionShort      string
	completionLong       string
	completionNoDescFlag string
	completionShellShort string
	completionShellLong  string

	versionShort string

	statusShort       string
	statusLong        string
	statusVerboseFlag string
	statusJSONFlag    string
	statusFetchingFmt string

	// Text-mode usage rendering (status).
	statusErrorFmt            string // provider name, error
	statusFiveHourLineFmt     string // formatted window
	statusWeeklyLineFmt       string // formatted window
	statusNotEnforced         string
	statusWindowFmt           string // bar, pct, display word, countdown, clock
	statusWindowNoResetFmt    string // bar, pct, display word
	statusUsedWord            string
	statusRemainingWord       string
	statusCreditsUnlimited    string
	statusCreditsFmt          string // balance
	statusResetCreditsOneFmt  string // count (1)
	statusResetCreditsManyFmt string // count (>1)
	statusCreditAvailable     string
	statusCreditRedeemed      string
	statusCreditExpired       string
	statusCreditGrantedFmt    string // datetime
	statusCreditExpiresFmt    string // datetime
	statusCreditExpiresInFmt  string // remaining duration, appended to the expires part
	statusCreditRedeemedFmt   string // datetime
	statusCreditTimeLayout    string
	statusListSep             string
	statusNowWord             string

	// Today's local token consumption (status).
	statusTodayLineFmt      string // rendered token/cost summary
	statusTodayTokensFmt    string // token count
	statusTodayCostFmt      string // cost, appended to the token count
	statusTodayBreakdownFmt string // input, cache read, cache write, output (-v)
	statusTodayModelFmt     string // model, its token/cost summary (-v)
	statusTodayUnknownModel string

	pingShort       string
	pingLong        string
	pingDryRunFlag  string
	pingWouldRunFmt string // provider, command
	pingSendingFmt  string // provider, spinner frame, elapsed
	pingModelFmt    string // model, appended to a command that does not name one
	pingFailedFmt   string // provider, elapsed, error
	pingSuccessFmt  string // provider, elapsed, usage suffix

	watchShort             string
	watchLong              string
	watchDryRunFlag        string
	watchLiveFlag          string
	watchAlreadyRunningFmt string

	// `redeem` reset-credit strings.
	redeemShort         string
	redeemLong          string
	redeemDryRunFlag    string
	redeemNoneAvailable string
	redeemPlanFmt       string // expiry stamp, remaining lifetime
	redeemDryRunNote    string
	redeemOutcomeFmt    string // outcome sentence
	redeemDone          string
	redeemNothing       string
	redeemNoCredit      string
	redeemAlready       string
	redeemUnknownFmt    string // raw outcome code

	configShort     string
	configInitShort string
	configInitForce string
	configPathShort string
}

func localizedText() cliText { return enText }

var enText = cliText{
	rootShort: "Keep Claude Code / Codex rate-limit windows back-to-back",
	rootLong:  "limitping pings your AI coding provider the moment its 5h rate-limit window resets, so the next window starts immediately and stays aligned. Usage is read via zero-quota endpoints; pings go through the official CLIs.",
	helpFlag:  "help for this command",
	usageTemplate: `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .NameAndAliases 24}} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .NameAndAliases 24}} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .NameAndAliases 24}} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`,

	helpCommandShort: "Help about any command",
	helpCommandLong:  "Help provides help for any command in the application.\nType limitping help [command] for full details.",
	helpUnknownTopic: "Unknown help topic",

	completionShort:      "Generate shell completion scripts",
	completionLong:       "Generate shell completion scripts for limitping.\n\nRun `limitping completion [bash|zsh|fish|powershell] --help` for shell-specific usage.",
	completionNoDescFlag: "disable completion descriptions",
	completionShellShort: "Generate the %s completion script",
	completionShellLong:  "Generate the %s completion script for limitping.",

	versionShort: "Print the version",

	statusShort: "Show current 5h/weekly usage and reset countdowns without using quota",
	statusLong: `Show current 5h and weekly usage for every enabled provider. This command only reads usage data from zero-quota endpoints; it does not send a ping or consume model quota.

The 'today' line totals the tokens this machine's Claude Code / Codex sessions have used since local midnight, read from the transcripts those CLIs write to disk, and prices them at published API rates — what the day would have cost without the subscription. Work done from another machine or from the web app is not in those logs. Add -v for the per-model breakdown.`,
	statusVerboseFlag: "print the raw JSON response",
	statusJSONFlag:    "output usage as JSON instead of text",
	statusFetchingFmt: "Fetching %s usage...\n",

	statusErrorFmt:            "%-7s  error: %v\n",
	statusFiveHourLineFmt:     "  5h     %s\n",
	statusWeeklyLineFmt:       "  weekly %s\n",
	statusNotEnforced:         "not currently enforced",
	statusWindowFmt:           "%s %5.1f%% %-9s resets in %-8s (%s)",
	statusWindowNoResetFmt:    "%s %5.1f%% %-9s (no active window)",
	statusUsedWord:            "used",
	statusRemainingWord:       "remaining",
	statusCreditsUnlimited:    "  credits unlimited\n",
	statusCreditsFmt:          "  credits %s\n",
	statusResetCreditsOneFmt:  "  reset credits %d reset available\n",
	statusResetCreditsManyFmt: "  reset credits %d resets available\n",
	statusCreditAvailable:     "available",
	statusCreditRedeemed:      "redeemed",
	statusCreditExpired:       "expired",
	statusCreditGrantedFmt:    "granted %s",
	statusCreditExpiresFmt:    "expires %s",
	statusCreditExpiresInFmt:  " (in %s)",
	statusCreditRedeemedFmt:   "redeemed %s",
	statusCreditTimeLayout:    "Jan 02 15:04",
	statusListSep:             ", ",
	statusNowWord:             "now",

	statusTodayLineFmt:      "  today  %s\n",
	statusTodayTokensFmt:    "%s tok",
	statusTodayCostFmt:      "  ≈ $%s",
	statusTodayBreakdownFmt: "         in %s · cache %s read / %s write · out %s\n",
	statusTodayModelFmt:     "         %-26s %s\n",
	statusTodayUnknownModel: "unknown model",

	pingShort: "Trigger a provider window now with a minimal message",
	pingLong: `Trigger a rate-limit window immediately by sending the minimal message for the selected provider.

Arguments:
  provider  Optional. One of: claude, codex, all.
            Defaults to all, which pings every enabled provider.

Examples:
  limitping ping
  limitping p claude
  limitping ping codex --dry-run`,
	pingDryRunFlag:  "print the command without sending",
	pingWouldRunFmt: "%-7s would run: %s\n",
	pingSendingFmt:  "\r%-7s %c sending… %s",
	pingModelFmt:    "  (model: %s)",
	pingFailedFmt:   "%-7s ✗ failed after %s: %v\n",
	pingSuccessFmt:  "%-7s ✓ pinged (%s%s)\n",

	watchShort: "Run the foreground daemon and ping each provider when its 5h window resets",
	watchLong: `Run the foreground daemon. When a provider's 5h window resets, limitping sends the minimal message to start the next window.

Arguments:
  provider  Optional. One of: claude, codex, all.
            Defaults to all, which watches every enabled provider.

Codex reset credits: set auto_redeem = true under [codex] in the config and watch also spends a banked reset credit that is about to lapse — within 24h when there is usage worth reclaiming, or in its final hour. Off by default because redeeming is irreversible; 'limitping redeem' spends one by hand.

Examples:
  limitping watch
  limitping w claude
  limitping watch --live
  limitping watch --dry-run`,
	watchDryRunFlag:        "log when pings would fire without sending them",
	watchLiveFlag:          "show a live heartbeat/status line while watching (uses more power)",
	watchAlreadyRunningFmt: "watch already running (pid %d, provider %s%s, started %s); stop it before starting another watcher",

	redeemShort: "Spend a banked Codex rate-limit reset credit now",
	redeemLong: `Consume one of the Codex reset credits shown by 'limitping status', resetting the rate-limit windows it is eligible for.

Redeeming is irreversible. The backend decides which credit to spend and refuses with "nothing to reset" when no window is currently eligible, so a credit is never burned for nothing.

Set auto_redeem = true under [codex] in the config to let 'watch' spend a credit on its own once it is close to expiring (within 24h with real usage to reclaim, or in its final hour).

Examples:
  limitping redeem --dry-run
  limitping redeem`,
	redeemDryRunFlag:    "show which credit would be spent without consuming it",
	redeemNoneAvailable: "no reset credits available to redeem",
	redeemPlanFmt:       "codex   redeeming 1 reset credit (expires %s, in %s)\n",
	redeemDryRunNote:    "dry run: nothing was consumed\n",
	redeemOutcomeFmt:    "codex   %s\n",
	redeemDone:          "redeemed — the eligible rate-limit windows were reset",
	redeemNothing:       "no rate-limit window is currently eligible for a reset; the credit was not spent",
	redeemNoCredit:      "the account has no reset credits available",
	redeemAlready:       "this redemption already completed earlier",
	redeemUnknownFmt:    "unexpected outcome from the backend: %s",

	configShort:     "Manage the configuration file",
	configInitShort: "Write a default config file",
	configInitForce: "overwrite an existing config",
	configPathShort: "Print the config file path",
}
