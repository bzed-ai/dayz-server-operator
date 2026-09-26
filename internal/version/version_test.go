// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package version

import "testing"

func TestString(t *testing.T) {
	Version, Commit, Date = "1.2.3", "abcdef", "2026-01-01"
	t.Cleanup(func() { Version, Commit, Date = "dev", "unknown", "unknown" })

	got := String()
	want := "1.2.3 (abcdef, 2026-01-01)"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
