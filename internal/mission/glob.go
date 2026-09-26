// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package mission

import (
	"path/filepath"
	"strings"
)

// MatchGlob reports whether path matches pattern, where pattern is a
// slash-separated glob supporting "**" as a whole segment (matching zero
// or more path segments, e.g. "expansion/**" matches "expansion" and
// everything under it) in addition to filepath.Match's single-segment
// "*"/"?"/character-class syntax. This is what instance.yaml's
// mission.unmanaged globs use (§C3 example: ["expansion/**"]).
func MatchGlob(pattern, path string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(path, "/"))
}

func matchSegments(pat, path []string) bool {
	if len(pat) == 0 {
		return len(path) == 0
	}
	if pat[0] == "**" {
		if matchSegments(pat[1:], path) {
			return true
		}
		return len(path) > 0 && matchSegments(pat, path[1:])
	}
	if len(path) == 0 {
		return false
	}
	ok, err := filepath.Match(pat[0], path[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pat[1:], path[1:])
}

// MatchAny reports whether path matches any of patterns.
func MatchAny(patterns []string, path string) bool {
	for _, p := range patterns {
		if MatchGlob(p, path) {
			return true
		}
	}
	return false
}
