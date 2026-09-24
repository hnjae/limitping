// SPDX-FileCopyrightText: 2026 wavever
// SPDX-FileCopyrightText: 2026 KIM Hyunjae
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package cli

import (
	"context"
	"errors"
	"io"

	"github.com/hnjae/limitping/internal/config"
)

// runContinueProxy is unsupported on Windows: the proxy relies on a Unix PTY and
// raw-terminal handling. The neutral logic in proxy.go still compiles so tests
// run everywhere.
func runContinueProxy(_ context.Context, _ io.Writer, _ string, _ []string, _ config.Config) error {
	return errors.New("limitping continue (interactive proxy) is not supported on Windows yet")
}
