// SPDX-FileCopyrightText: 2026 wavever
// SPDX-FileCopyrightText: 2026 KIM Hyunjae
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hnjae/limitping/internal/provider"
	"github.com/hnjae/limitping/internal/usage"
)

// A ping's own output cannot answer the question it is run to answer: a ping is
// too small to move the used percentage, so the window state has to be read
// back from the provider afterwards.
func TestRunPingsReportsTheWindowStateAfterward(t *testing.T) {
	p := fakeStatusProvider{
		name: "codex",
		usage: &usage.Usage{
			Provider:  "codex",
			Plan:      "plus",
			FiveHour:  usage.Window{ResetsAt: time.Now().Add(5 * time.Hour), WindowSeconds: 18000},
			FetchedAt: time.Now(),
		},
	}

	var out bytes.Buffer
	if err := runPings(context.Background(), &out, []provider.Provider{p}, false, false, "used"); err != nil {
		t.Fatalf("runPings() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "codex ok") {
		t.Fatalf("output = %q, want the provider command", got)
	}
	if !strings.Contains(got, "codex (plus)") || !strings.Contains(got, "5h") {
		t.Fatalf("output = %q, want the window state after the ping", got)
	}
}

// A dry run sends nothing, so there is no new state to read back — and reading
// it would suggest the window was touched.
func TestRunPingsDryRunReportsNoWindowState(t *testing.T) {
	read := 0
	p := fakeStatusProvider{
		name:   "codex",
		usage:  &usage.Usage{Provider: "codex"},
		onRead: func() { read++ },
	}

	var out bytes.Buffer
	if err := runPings(context.Background(), &out, []provider.Provider{p}, true, false, "used"); err != nil {
		t.Fatalf("runPings() error = %v", err)
	}
	if read != 0 {
		t.Fatalf("usage reads = %d, want none for a dry run", read)
	}
	if !strings.Contains(out.String(), "codex ok") {
		t.Fatalf("output = %q, want the dry-run command", out.String())
	}
}

func TestCommandLineNamesTheModelOnlyWhenTheCommandDoesNot(t *testing.T) {

	cases := []struct {
		name      string
		res       provider.TriggerResult
		wantModel bool
	}{{
		name:      "model absent from the command is appended",
		res:       provider.TriggerResult{Command: "codex -c model_reasoning_effort=low ok", Model: "gpt-5.6-sol"},
		wantModel: true,
	}, {
		name: "model already in the command is not repeated",
		res:  provider.TriggerResult{Command: "codex -m gpt-5.6-luna ok", Model: "gpt-5.6-luna"},
	}, {
		name: "unresolvable model adds nothing",
		res:  provider.TriggerResult{Command: "claude ."},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := commandLine(&c.res)
			if c.wantModel {
				if !strings.HasPrefix(got, c.res.Command) || !strings.Contains(got, c.res.Model) {
					t.Fatalf("commandLine() = %q, want command and selected model", got)
				}
			} else if got != c.res.Command {
				t.Fatalf("commandLine() = %q, want the provider command unchanged", got)
			}
		})
	}
}
