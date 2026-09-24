// SPDX-FileCopyrightText: 2026 wavever
// SPDX-FileCopyrightText: 2026 KIM Hyunjae
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package cli

import "syscall"

// processAlive reports whether a process with the given pid is currently running.
// Signal 0 does the kernel's existence/permission check without delivering a
// signal: a nil error means it's ours and alive; EPERM means alive but owned by
// someone else (still running).
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
