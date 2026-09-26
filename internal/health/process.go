// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package health implements dzo's in-container health probes (§C9):
// HealthStartupCmd waits for the server to come up, HealthCmd checks it is
// still alive. Both run inside the game container (the static dzo binary
// is bind-mounted read-only) and probe 127.0.0.1 in the container's own
// network/PID namespace, so they work under both network modes.
package health

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultProcessNames are the names dzo looks for when checking that the
// game server process is running. "DayZServer" is the executable name.
// "enfMain" is noted in the plan (§C22) as the name the Enfusion engine's
// main thread/process shows under in a real boot test - kept here as a
// fallback until that is confirmed against a live server (needs S1/S9
// verification); matching on either name means dzo works whichever turns
// out to be accurate, or both.
var DefaultProcessNames = []string{"DayZServer", "enfMain"}

// ProcessRunning reports whether any process under procRoot (normally
// "/proc") matches one of names, by /proc/<pid>/comm or the basename of
// argv[0] in /proc/<pid>/cmdline. procRoot is a parameter (rather than a
// hardcoded "/proc") so this is unit-testable with a fabricated directory
// tree, without a real DayZServer binary.
func ProcessRunning(procRoot string, names []string) (bool, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return false, fmt.Errorf("health: read %s: %w", procRoot, err)
	}
	wanted := make(map[string]bool, len(names))
	for _, n := range names {
		wanted[n] = true
	}

	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue // not a PID directory
		}
		pidDir := filepath.Join(procRoot, e.Name())

		if comm, err := os.ReadFile(filepath.Join(pidDir, "comm")); err == nil { //nolint:gosec // pidDir is procRoot + a numeric directory name from ReadDir, not external input
			if wanted[strings.TrimSpace(string(comm))] {
				return true, nil
			}
		}
		if cmdline, err := os.ReadFile(filepath.Join(pidDir, "cmdline")); err == nil { //nolint:gosec // see above
			argv0, _, _ := strings.Cut(string(cmdline), "\x00")
			if wanted[filepath.Base(argv0)] {
				return true, nil
			}
		}
	}
	return false, nil
}
