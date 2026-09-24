// SPDX-FileCopyrightText: 2026 wavever
// SPDX-FileCopyrightText: 2026 KIM Hyunjae
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package cli

import "syscall"

// Windows process status constants (from winbase.h).
const (
	processQueryLimitedInfo = 0x00001000
	stillActive             = 259
)

// processAlive reports whether a process with the given pid is currently running.
// Signal-based checks aren't available on Windows, so query the process exit code.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(processQueryLimitedInfo, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
