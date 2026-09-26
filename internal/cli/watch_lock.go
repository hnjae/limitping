// SPDX-FileCopyrightText: 2026 wavever
// SPDX-FileCopyrightText: 2026 KIM Hyunjae
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hnjae/limitping/internal/config"
)

const watchLockName = "watch.lock"

type watchLockState struct {
	PID       int       `json:"pid"`
	Provider  string    `json:"provider"`
	DryRun    bool      `json:"dry_run"`
	StartedAt time.Time `json:"started_at"`
}

func acquireWatchLock(provider string, dryRun bool) (func(), error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, watchLockName)

	for {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			st := watchLockState{
				PID:       os.Getpid(),
				Provider:  provider,
				DryRun:    dryRun,
				StartedAt: time.Now(),
			}
			data, jerr := json.MarshalIndent(st, "", "  ")
			if jerr == nil {
				_, jerr = f.Write(append(data, '\n'))
			}
			cerr := f.Close()
			if jerr != nil {
				_ = os.Remove(path)
				return nil, jerr
			}
			if cerr != nil {
				_ = os.Remove(path)
				return nil, cerr
			}
			return func() { releaseWatchLock(path, st.PID) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}

		st, ok := readWatchLockPath(path)
		if ok && processAlive(st.PID) {
			return nil, watchAlreadyRunningError(st)
		}
		_ = os.Remove(path)
	}
}

func readWatchLockPath(path string) (watchLockState, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return watchLockState{}, false
	}
	var st watchLockState
	if err := json.Unmarshal(data, &st); err != nil || st.PID <= 0 {
		return watchLockState{}, false
	}
	return st, true
}

func releaseWatchLock(path string, pid int) {
	st, ok := readWatchLockPath(path)
	if ok && st.PID != pid {
		return
	}
	_ = os.Remove(path)
}

func watchAlreadyRunningError(st watchLockState) error {
	started := st.StartedAt.Format("2006-01-02 15:04:05")
	return fmt.Errorf("watch already running (pid %d, provider %s%s, started %s)", st.PID, st.Provider, dryRunNote(st.DryRun), started)
}

// dryRunNote renders the watch mode in an already-running error.
func dryRunNote(dryRun bool) string {
	if dryRun {
		return " (dry-run)"
	}
	return ""
}
