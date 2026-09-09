package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/wavever/CCLimitPing/internal/update"
)

// The update notice runs before a command's own output, the way the Codex CLI
// does it, so a new release is seen rather than scrolled past. It is offered
// only on an interactive terminal and only from commands a person is watching:
// `hook` must stay fast and silent, `watch`/`bg start` may have no terminal at
// all, and `--json` output has to stay machine-readable.

var updateHTTPClient = http.DefaultClient

// updateNotice asks about a newer release, if there is one. Every failure path
// is silent: a version check must never get in the way of the command the user
// actually ran.
func updateNotice(ctx context.Context, out io.Writer, text cliText, in *os.File) {
	if !isTerminal(os.Stdout) || !isTerminal(in) || !isReleaseVersion() {
		return
	}
	latest := update.Latest(ctx, updateHTTPClient)
	next := update.Available(version(), latest, update.Load().DismissedVersion)
	if next == "" {
		return
	}
	promptUpdate(out, text, in, next)
}

// promptUpdate renders the notice and applies the choice. An unrecognized or
// empty answer means "skip", the option that changes nothing.
func promptUpdate(out io.Writer, text cliText, in io.Reader, next string) {
	fmt.Fprintf(out, text.updateAvailableFmt, update.Normalize(version()), next)
	fmt.Fprintf(out, text.updateNotesFmt, update.ReleaseNotesURL)
	fmt.Fprintf(out, text.updateOptionUpgrade, invokedName()+" upgrade")
	fmt.Fprint(out, text.updateOptionSkip)
	fmt.Fprint(out, text.updateOptionSkipVersion)
	fmt.Fprint(out, text.updateChoosePrompt)

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(out)
		return
	}
	switch strings.TrimSpace(line) {
	case "1":
		fmt.Fprintln(out)
		if err := runUpgrade(context.Background(), out, out); err != nil {
			fmt.Fprintf(out, text.updateFailedFmt, err)
		}
	case "3":
		if err := update.Dismiss(next); err != nil {
			fmt.Fprintf(out, text.updateFailedFmt, err)
		} else {
			fmt.Fprintf(out, text.updateDismissedFmt, next)
		}
	}
	fmt.Fprintln(out)
}
