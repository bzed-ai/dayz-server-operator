// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package mission

import "testing"

func TestMatchGlobDoubleStarSuffix(t *testing.T) {
	cases := map[string]bool{
		"expansion":            true,
		"expansion/traders":    true,
		"expansion/a/b/c.json": true,
		"expansions/traders":   false,
		"other/expansion/x":    false,
	}
	for path, want := range cases {
		if got := MatchGlob("expansion/**", path); got != want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", "expansion/**", path, got, want)
		}
	}
}

func TestMatchGlobSingleSegmentWildcard(t *testing.T) {
	if !MatchGlob("custom_*/keys/*.bikey", "custom_loadout/keys/x.bikey") {
		t.Error("expected a match")
	}
	if MatchGlob("custom_*/keys/*.bikey", "custom_loadout/sub/keys/x.bikey") {
		t.Error("single * must not cross a path separator")
	}
}

func TestMatchGlobExact(t *testing.T) {
	if !MatchGlob("db/messages.xml", "db/messages.xml") {
		t.Error("expected an exact match")
	}
	if MatchGlob("db/messages.xml", "db/other.xml") {
		t.Error("expected no match")
	}
}

func TestMatchAny(t *testing.T) {
	patterns := []string{"expansion/**", "*.bak"}
	if !MatchAny(patterns, "expansion/x") {
		t.Error("expected a match on the first pattern")
	}
	if !MatchAny(patterns, "foo.bak") {
		t.Error("expected a match on the second pattern")
	}
	if MatchAny(patterns, "foo.xml") {
		t.Error("expected no match")
	}
}

func TestMatchAnyEmpty(t *testing.T) {
	if MatchAny(nil, "anything") {
		t.Error("no patterns should never match")
	}
}
