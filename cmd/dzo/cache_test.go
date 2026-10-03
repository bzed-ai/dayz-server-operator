// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"strings"
	"testing"

	"github.com/bzed/dayz-server-operator/internal/cache"
)

func TestCacheListAndGC(t *testing.T) {
	dir := t.TempDir()
	s := cache.New(dir)
	for _, id := range []string{"1", "2", "3"} {
		if _, err := s.NewGeneration(id); err != nil {
			t.Fatalf("NewGeneration(%s): %v", id, err)
		}
	}
	if err := s.SetCurrent("2"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}

	out, err := runCmd(t, "cache", "list", dir)
	if err != nil {
		t.Fatalf("cache list: %v", err)
	}
	if !strings.Contains(out, "* 2") {
		t.Errorf("list output should mark 2 as current:\n%s", out)
	}

	out, err = runCmd(t, "cache", "gc", dir, "--keep", "1")
	if err != nil {
		t.Fatalf("cache gc: %v", err)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("gc output = %q", out)
	}
}

func TestCacheListMissingStore(t *testing.T) {
	out, err := runCmd(t, "cache", "list", "/nonexistent/store")
	if err != nil {
		t.Fatalf("cache list on an empty store should not error: %v", err)
	}
	if out != "" {
		t.Errorf("expected no output for an empty store, got %q", out)
	}
}
