// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewGenerationCreatesDir(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "store"))
	path, err := s.NewGeneration("123")
	if err != nil {
		t.Fatalf("NewGeneration: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected a directory at %s", path)
	}
}

func TestNewGenerationRefusesDuplicate(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.NewGeneration("123"); err != nil {
		t.Fatalf("NewGeneration: %v", err)
	}
	if _, err := s.NewGeneration("123"); err == nil {
		t.Fatal("expected an error creating the same generation twice")
	}
}

func TestNewGenerationRejectsReservedNames(t *testing.T) {
	s := New(t.TempDir())
	for _, name := range []string{"", "current", ".dzo-lock"} {
		if _, err := s.NewGeneration(name); err == nil {
			t.Errorf("expected an error for reserved name %q", name)
		}
	}
}

func TestCurrentEmptyByDefault(t *testing.T) {
	s := New(t.TempDir())
	cur, err := s.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if cur != "" {
		t.Errorf("Current() = %q, want empty", cur)
	}
}

func TestSetCurrentAndCurrent(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.NewGeneration("100"); err != nil {
		t.Fatalf("NewGeneration: %v", err)
	}
	if err := s.SetCurrent("100"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	cur, err := s.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if cur != "100" {
		t.Errorf("Current() = %q, want 100", cur)
	}
}

func TestSetCurrentSwitchesAtomically(t *testing.T) {
	s := New(t.TempDir())
	for _, id := range []string{"100", "200"} {
		if _, err := s.NewGeneration(id); err != nil {
			t.Fatalf("NewGeneration(%s): %v", id, err)
		}
	}
	if err := s.SetCurrent("100"); err != nil {
		t.Fatalf("SetCurrent(100): %v", err)
	}
	if err := s.SetCurrent("200"); err != nil {
		t.Fatalf("SetCurrent(200): %v", err)
	}
	cur, _ := s.Current()
	if cur != "200" {
		t.Errorf("Current() = %q, want 200", cur)
	}
}

func TestSetCurrentRejectsMissingGeneration(t *testing.T) {
	s := New(t.TempDir())
	if err := s.SetCurrent("nope"); err == nil {
		t.Fatal("expected an error for a nonexistent generation")
	}
}

func TestGenerationsSortedByAge(t *testing.T) {
	s := New(t.TempDir())
	for _, id := range []string{"a", "b", "c"} {
		if _, err := s.NewGeneration(id); err != nil {
			t.Fatalf("NewGeneration(%s): %v", id, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	gens, err := s.Generations()
	if err != nil {
		t.Fatalf("Generations: %v", err)
	}
	want := []string{"a", "b", "c"}
	if !equalStrings(gens, want) {
		t.Fatalf("Generations() = %v, want %v", gens, want)
	}
}

func TestGenerationsEmptyStore(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "does-not-exist-yet"))
	gens, err := s.Generations()
	if err != nil {
		t.Fatalf("Generations: %v", err)
	}
	if len(gens) != 0 {
		t.Fatalf("Generations() = %v, want empty", gens)
	}
}

func TestRemoveDeletesGeneration(t *testing.T) {
	s := New(t.TempDir())
	path, err := s.NewGeneration("100")
	if err != nil {
		t.Fatalf("NewGeneration: %v", err)
	}
	if err := s.Remove("100"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("generation directory should be gone, stat err = %v", err)
	}
}

func TestRemoveRefusesCurrent(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.NewGeneration("100"); err != nil {
		t.Fatalf("NewGeneration: %v", err)
	}
	if err := s.SetCurrent("100"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	if err := s.Remove("100"); err == nil {
		t.Fatal("expected an error removing the current generation")
	}
}

func TestRemoveRejectsReservedNames(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Remove("current"); err == nil {
		t.Fatal("expected an error removing the reserved 'current' name")
	}
}

func TestGCKeepsMostRecentAndCurrent(t *testing.T) {
	s := New(t.TempDir())
	ids := []string{"g1", "g2", "g3", "g4", "g5"}
	for _, id := range ids {
		if _, err := s.NewGeneration(id); err != nil {
			t.Fatalf("NewGeneration(%s): %v", id, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	// g2 is old but marked current, and must survive GC even though it
	// falls outside the "keep 2 most recent" window.
	if err := s.SetCurrent("g2"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}

	removed, err := s.GC(2)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	wantRemoved := []string{"g3", "g1"}
	if !equalStrings(sortedCopy(removed), sortedCopy(wantRemoved)) {
		t.Fatalf("GC removed = %v, want %v", removed, wantRemoved)
	}

	remaining, err := s.Generations()
	if err != nil {
		t.Fatalf("Generations: %v", err)
	}
	wantRemaining := []string{"g2", "g4", "g5"}
	if !equalStrings(sortedCopy(remaining), sortedCopy(wantRemaining)) {
		t.Fatalf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestGCRejectsNonPositiveKeep(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.GC(0); err == nil {
		t.Fatal("expected an error for keep=0")
	}
}

func TestGCOnEmptyStore(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "empty"))
	removed, err := s.GC(3)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none", removed)
	}
}

func TestCopyGeneration(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o640); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o750); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "nested.txt"), []byte("world"), 0o640); err != nil {
		t.Fatalf("write nested fixture: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.MkdirAll(dst, 0o750); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	if err := CopyGeneration(src, dst); err != nil {
		t.Fatalf("CopyGeneration: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil || string(data) != "hello" {
		t.Fatalf("file.txt = %q, %v", data, err)
	}
	data, err = os.ReadFile(filepath.Join(dst, "sub", "nested.txt"))
	if err != nil || string(data) != "world" {
		t.Fatalf("sub/nested.txt = %q, %v", data, err)
	}
}

func TestCopyGenerationMissingSource(t *testing.T) {
	if err := CopyGeneration("/nonexistent/source/dir", t.TempDir()); err == nil {
		t.Fatal("expected an error for a missing source directory")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func TestFlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	unlock, err := Flock(path)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	unlock2, err := Flock(path) // free again after unlock
	if err != nil {
		t.Fatal(err)
	}
	unlock2()
	if _, err := Flock(filepath.Join(t.TempDir(), "no", "such", "dir", "lock")); err == nil {
		t.Error("an uncreatable lock file must fail")
	}
}
