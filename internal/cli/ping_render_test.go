package cli

import (
	"strings"
	"testing"

	"github.com/wavever/CCLimitPing/internal/provider"
)

func TestCommandLineNamesTheModelOnlyWhenTheCommandDoesNot(t *testing.T) {
	setLocale(t, "C")
	text := localizedText()

	cases := []struct {
		name string
		res  provider.TriggerResult
		want string
	}{{
		name: "model absent from the command is appended",
		res:  provider.TriggerResult{Command: "codex -c model_reasoning_effort=low ok", Model: "gpt-5.6-sol"},
		want: "codex -c model_reasoning_effort=low ok  (model: gpt-5.6-sol)",
	}, {
		name: "model already in the command is not repeated",
		res:  provider.TriggerResult{Command: "codex -m gpt-5.6-luna ok", Model: "gpt-5.6-luna"},
		want: "codex -m gpt-5.6-luna ok",
	}, {
		name: "unresolvable model adds nothing",
		res:  provider.TriggerResult{Command: "claude ."},
		want: "claude .",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := commandLine(text, &c.res); got != c.want {
				t.Fatalf("commandLine() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestCommandLineIsLocalized(t *testing.T) {
	setLocale(t, "zh_CN.UTF-8")
	res := provider.TriggerResult{Command: "codex ok", Model: "gpt-5.6-sol"}
	if got := commandLine(localizedText(), &res); !strings.Contains(got, "模型: gpt-5.6-sol") {
		t.Fatalf("commandLine() = %q, want a localized model label", got)
	}
}
