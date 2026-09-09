package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wavever/CCLimitPing/internal/update"
)

func TestPromptUpdateOffersThreeChoices(t *testing.T) {
	setLocale(t, "C")
	var out bytes.Buffer
	promptUpdate(&out, localizedText(), strings.NewReader("2\n"), "0.10.0")

	got := out.String()
	for _, want := range []string{
		"Update available!",
		update.ReleaseNotesURL,
		"1. Update now",
		"2. Skip",
		"3. Skip until next version",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "-> 0.10.0") {
		t.Fatalf("prompt does not name the new version:\n%s", got)
	}
}

// Skipping must leave no trace, so the same release is offered again next time.
func TestPromptUpdateSkipDoesNotDismiss(t *testing.T) {
	setLocale(t, "C")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for _, answer := range []string{"2\n", "\n", "nonsense\n", ""} {
		var out bytes.Buffer
		promptUpdate(&out, localizedText(), strings.NewReader(answer), "0.10.0")
		if got := update.Load().DismissedVersion; got != "" {
			t.Fatalf("answer %q dismissed %q, want nothing recorded", answer, got)
		}
	}
}

func TestPromptUpdateSkipUntilNextVersionRecordsTheDismissal(t *testing.T) {
	setLocale(t, "C")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var out bytes.Buffer
	promptUpdate(&out, localizedText(), strings.NewReader("3\n"), "0.10.0")

	if got := update.Load().DismissedVersion; got != "0.10.0" {
		t.Fatalf("DismissedVersion = %q, want 0.10.0", got)
	}
	// And that release is then silent, while the next one still speaks up.
	if got := update.Available(Version, "0.10.0", "0.10.0"); got != "" {
		t.Fatalf("Available after dismissal = %q, want silence", got)
	}
	if got := update.Available("0.9.0", "0.11.0", "0.10.0"); got != "0.11.0" {
		t.Fatalf("Available for the next release = %q, want 0.11.0", got)
	}
}

func TestPromptUpdateIsLocalized(t *testing.T) {
	setLocale(t, "zh_CN.UTF-8")
	var out bytes.Buffer
	promptUpdate(&out, localizedText(), strings.NewReader("2\n"), "0.10.0")

	if got := out.String(); !strings.Contains(got, "有新版本") || !strings.Contains(got, "3. 跳过此版本") {
		t.Fatalf("prompt is not localized:\n%s", got)
	}
}

// The notice writes to stdout ahead of the command's own output, so anything
// that is not a person at a terminal must never see it.
func TestUpdateNoticeStaysSilentWithoutATerminal(t *testing.T) {
	setLocale(t, "C")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := update.Save(update.State{LatestVersion: "99.0.0"}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	// os.Stdout under `go test` is a pipe, not a character device.
	updateNotice(t.Context(), &out, localizedText(), nil)
	if out.Len() != 0 {
		t.Fatalf("notice printed without a terminal:\n%s", out.String())
	}
}
