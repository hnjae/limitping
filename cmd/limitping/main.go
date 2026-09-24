// SPDX-FileCopyrightText: 2026 wavever
// SPDX-FileCopyrightText: 2026 KIM Hyunjae
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command limitping keeps Claude Code / Codex rate-limit windows back-to-back
// by pinging each provider the moment its 5h window resets.
package main

import (
	"fmt"
	"os"

	"github.com/hnjae/limitping/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
