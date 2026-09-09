package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestLocalizedTextUsesChineseForChineseLocale(t *testing.T) {
	setLocale(t, "zh_CN.UTF-8")

	text := localizedText()
	if !strings.Contains(text.pingLong, "参数") {
		t.Fatalf("pingLong = %q, want Chinese help text", text.pingLong)
	}
	if !strings.Contains(text.statusShort, "用量") {
		t.Fatalf("statusShort = %q, want Chinese help text", text.statusShort)
	}
}

func TestLocalizedTextFallsBackToEnglish(t *testing.T) {
	setLocale(t, "C")

	text := localizedText()
	if !strings.Contains(text.pingLong, "Arguments") {
		t.Fatalf("pingLong = %q, want English help text", text.pingLong)
	}
	if !strings.Contains(text.statusShort, "usage") {
		t.Fatalf("statusShort = %q, want English help text", text.statusShort)
	}
}

func TestLocalizedTextHonorsLocalePrecedence(t *testing.T) {
	// POSIX: LC_ALL overrides LANG, so an explicit en_US wins over zh_CN.
	setLocale(t, "zh_CN.UTF-8")
	t.Setenv("LC_ALL", "en_US.UTF-8")

	text := localizedText()
	if !strings.Contains(text.statusShort, "usage") {
		t.Fatalf("statusShort = %q, want English when LC_ALL=en_US overrides LANG=zh_CN", text.statusShort)
	}
}

func TestRootCommandAliases(t *testing.T) {
	setLocale(t, "C")

	root := newRootCmd()
	cases := map[string]string{
		"p":      "ping",
		"sched":  "schedule",
		"s":      "status",
		"w":      "watch",
		"c":      "config",
		"cfg":    "config",
		"v":      "version",
		"ver":    "version",
		"up":     "upgrade",
		"update": "upgrade",
		"rm":     "uninstall",
		"remove": "uninstall",
	}

	for alias, want := range cases {
		cmd, _, err := root.Find([]string{alias})
		if err != nil {
			t.Fatalf("Find(%q) error = %v", alias, err)
		}
		if got := cmd.Name(); got != want {
			t.Fatalf("Find(%q) = %q, want %q", alias, got, want)
		}
	}

	nested := map[string]string{
		"i": "init",
		"p": "path",
	}
	for alias, want := range nested {
		cmd, _, err := root.Find([]string{"c", alias})
		if err != nil {
			t.Fatalf("Find(config %q) error = %v", alias, err)
		}
		if got := cmd.Name(); got != want {
			t.Fatalf("Find(config %q) = %q, want %q", alias, got, want)
		}
	}
}

func TestHelpFlagDescriptionIsLocalized(t *testing.T) {
	setLocale(t, "zh_CN.UTF-8")

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"ping", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "显示此命令的帮助") {
		t.Fatalf("help output = %q, want localized help flag", got)
	}
}

func TestRootHelpLocalizesDefaultCompletionCommand(t *testing.T) {
	setLocale(t, "zh_CN.UTF-8")

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "生成 shell 补全脚本") {
		t.Fatalf("help output = %q, want localized completion command", got)
	}
	if strings.Contains(got, "Generate the autocompletion script") {
		t.Fatalf("help output = %q, still contains default English completion text", got)
	}
}

func TestRootHelpPrintsCommandAliases(t *testing.T) {
	setLocale(t, "C")

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"ping, p",
		"status, s, stat",
		"version, v, ver",
		"upgrade, up, update",
		"uninstall, rm, remove",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("help output = %q, want command alias %q", got, want)
		}
	}
}

func TestConfigHelpPrintsSubcommandAliases(t *testing.T) {
	setLocale(t, "zh_CN.UTF-8")

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"config", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"init, i", "path, p"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help output = %q, want subcommand alias %q", got, want)
		}
	}
}

func setLocale(t *testing.T, locale string) {
	t.Helper()
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		t.Setenv(key, "")
	}
	t.Setenv("LANG", locale)
}

func TestInvokedNameRecognizesOnlyTheAlias(t *testing.T) {
	cases := map[string]string{
		"/usr/local/bin/lmp":       BinaryAlias,
		"lmp.exe":                  BinaryAlias,
		"/usr/local/bin/limitping": "limitping",
		"/tmp/go-build/cli.test":   "limitping",
	}
	for argv0, want := range cases {
		t.Run(argv0, func(t *testing.T) {
			old := os.Args
			os.Args = []string{argv0}
			defer func() { os.Args = old }()
			if got := invokedName(); got != want {
				t.Fatalf("invokedName() = %q, want %q", got, want)
			}
		})
	}
}

func TestRootUsageEchoesTheInvokedName(t *testing.T) {
	setLocale(t, "C")
	old := os.Args
	os.Args = []string{"/usr/local/bin/" + BinaryAlias}
	defer func() { os.Args = old }()

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	if got := out.String(); !strings.Contains(got, BinaryAlias+" [command]") {
		t.Fatalf("help output = %q, want usage lines using %q", got, BinaryAlias)
	}
}

func TestHelpExamplesUseTheInvokedNameAndKeepTheProductName(t *testing.T) {
	setLocale(t, "C")
	old := os.Args
	os.Args = []string{"/usr/local/bin/" + BinaryAlias}
	defer func() { os.Args = old }()

	root := newRootCmd()
	watch, _, err := root.Find([]string{"watch"})
	if err != nil {
		t.Fatalf("find watch: %v", err)
	}
	if strings.Contains(watch.Long, "limitping watch") {
		t.Errorf("watch help still shows `limitping watch` examples under the alias:\n%s", watch.Long)
	}
	if !strings.Contains(watch.Long, BinaryAlias+" watch") {
		t.Errorf("watch help has no %q examples:\n%s", BinaryAlias+" watch", watch.Long)
	}
	// The product name is not an invocation, so prose keeps saying limitping.
	if !strings.Contains(root.Long, "limitping pings") {
		t.Errorf("root help rewrote the product name out of its prose:\n%s", root.Long)
	}
}

func TestRedeemIsReachableByItsShortAlias(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"r"})
	if err != nil {
		t.Fatalf("find %q: %v", "r", err)
	}
	if cmd.Name() != "redeem" {
		t.Fatalf("%q resolved to %q, want redeem", "r", cmd.Name())
	}
}

func TestWatchAndContinueHelpDocumentAutoRedeem(t *testing.T) {
	for _, text := range []cliText{enText, zhText} {
		if !strings.Contains(text.watchLong, "auto_redeem") {
			t.Error("watch help does not mention auto_redeem")
		}
		if !strings.Contains(text.continueLong, "auto_redeem") {
			t.Error("continue help does not mention auto_redeem")
		}
		if !strings.Contains(text.watchLong, "redeem") || !strings.Contains(text.continueLong, "redeem") {
			t.Error("watch/continue help does not point at the redeem command")
		}
	}
}

// Whichever name was typed, the root help must advertise the other one, so a
// user who only ever runs `limitping` still discovers `lmp` and vice versa.
func TestRootHelpAdvertisesBothBinaryNames(t *testing.T) {
	for _, argv0 := range []string{"/usr/local/bin/" + BinaryAlias, "/usr/local/bin/limitping"} {
		t.Run(argv0, func(t *testing.T) {
			setLocale(t, "C")
			old := os.Args
			os.Args = []string{argv0}
			defer func() { os.Args = old }()

			root := newRootCmd()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetArgs([]string{"--help"})
			if err := root.Execute(); err != nil {
				t.Fatalf("help: %v", err)
			}
			got := out.String()
			for _, want := range []string{BinaryAlias, "limitping"} {
				if !strings.Contains(got, want) {
					t.Fatalf("help output does not mention %q:\n%s", want, got)
				}
			}
			if !strings.Contains(got, "Aliases:") {
				t.Fatalf("help output has no Aliases section:\n%s", got)
			}
		})
	}
}
