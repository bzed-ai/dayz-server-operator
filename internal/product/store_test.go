// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package product

import (
	"path/filepath"
	"testing"
)

func TestProductStoreRoot(t *testing.T) {
	s := ProductStore("/var/lib/dzo/cache", "dayz-stable")
	want := filepath.Join("/var/lib/dzo/cache", "products", "dayz-stable")
	if s.Root != want {
		t.Errorf("Root = %q, want %q", s.Root, want)
	}
}

func TestModStoreRoot(t *testing.T) {
	s := ModStore("/var/lib/dzo/cache", 221100, 111111)
	want := filepath.Join("/var/lib/dzo/cache", "workshop", "221100", "111111")
	if s.Root != want {
		t.Errorf("Root = %q, want %q", s.Root, want)
	}
}

func TestModGenerationID(t *testing.T) {
	cases := []struct {
		timeUpdated int64
		retry       int
		want        string
	}{
		{1700000000, 0, "1700000000"},
		{1700000000, 1, "1700000000-r1"},
		{1700000000, 2, "1700000000-r2"},
	}
	for _, c := range cases {
		if got := ModGenerationID(c.timeUpdated, c.retry); got != c.want {
			t.Errorf("ModGenerationID(%d, %d) = %q, want %q", c.timeUpdated, c.retry, got, c.want)
		}
	}
}

func TestStoreIntegrationWithCache(t *testing.T) {
	root := t.TempDir()
	s := ModStore(root, 221100, 111111)
	if err := s.EnsureRoot(); err != nil {
		t.Fatalf("EnsureRoot: %v", err)
	}
	id := ModGenerationID(1700000000, 0)
	genDir, err := s.NewGeneration(id)
	if err != nil {
		t.Fatalf("NewGeneration: %v", err)
	}
	if err := s.SetCurrent(id); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	current, err := s.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current != id {
		t.Errorf("Current() = %q, want %q", current, id)
	}
	if genDir == "" {
		t.Error("NewGeneration returned an empty path")
	}
}

func TestLocalModStore(t *testing.T) {
	if got, want := LocalModStore("/c", "tools").Root, "/c/local/tools"; got != want {
		t.Errorf("Root = %q, want %q", got, want)
	}
}
