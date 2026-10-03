// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package version holds build-time version information stamped via -ldflags.
package version

// Set via -ldflags "-X github.com/bzed/dayz-server-operator/internal/version.Version=..." at build time.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// String returns a human-readable "version (commit, date)" summary.
func String() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
