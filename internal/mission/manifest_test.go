// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package mission

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestMissingFileReturnsEmpty(t *testing.T) {
	m, err := LoadManifest(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(m.Entries) != 0 {
		t.Fatalf("Entries = %v, want empty", m.Entries)
	}
}

func TestManifestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	m := NewManifest()
	m.Entries["db/types.xml"] = Entry{Origin: OriginPristine, SourceHash: "sha256:a", WrittenHash: "sha256:a"}
	m.Entries["mod_123/events.xml"] = Entry{Origin: OriginGenerated, SourceHash: "sha256:b", WrittenHash: "sha256:b"}

	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(loaded.Entries) != 2 {
		t.Fatalf("Entries = %v", loaded.Entries)
	}
	if loaded.Entries["db/types.xml"].SourceHash != "sha256:a" {
		t.Errorf("entry = %+v", loaded.Entries["db/types.xml"])
	}
}

func TestManifestSaveIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	m := NewManifest()
	m.Entries["z.xml"] = Entry{Origin: OriginPristine, SourceHash: "1", WrittenHash: "1"}
	m.Entries["a.xml"] = Entry{Origin: OriginPristine, SourceHash: "2", WrittenHash: "2"}

	p1 := filepath.Join(dir, "m1.json")
	p2 := filepath.Join(dir, "m2.json")
	if err := m.Save(p1); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := m.Save(p2); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b1, _ := os.ReadFile(p1)
	b2, _ := os.ReadFile(p2)
	if string(b1) != string(b2) {
		t.Fatalf("Save output is not deterministic:\n%s\nvs\n%s", b1, b2)
	}
}

func TestManifestSaveAtomicNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	m := NewManifest()
	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only the final manifest file, got %v", entries)
	}
}

func TestManifestPathsSorted(t *testing.T) {
	m := NewManifest()
	m.Entries["z"] = Entry{}
	m.Entries["a"] = Entry{}
	m.Entries["m"] = Entry{}
	paths := m.Paths()
	if paths[0] != "a" || paths[1] != "m" || paths[2] != "z" {
		t.Errorf("Paths() = %v", paths)
	}
}

func TestHashBytesStable(t *testing.T) {
	h1 := HashBytes([]byte("hello"))
	h2 := HashBytes([]byte("hello"))
	if h1 != h2 {
		t.Fatalf("hashes differ: %s vs %s", h1, h2)
	}
	if HashBytes([]byte("world")) == h1 {
		t.Fatal("different content produced the same hash")
	}
}

func TestHashFileMissingReturnsEmpty(t *testing.T) {
	h, err := HashFile(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if h != "" {
		t.Errorf("HashFile(missing) = %q, want empty", h)
	}
}

func TestLoadManifestInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestManifestSaveMissingParentDirErrors(t *testing.T) {
	m := NewManifest()
	path := filepath.Join(t.TempDir(), "nonexistent-subdir", "manifest.json")
	if err := m.Save(path); err == nil {
		t.Fatal("expected an error when the parent directory doesn't exist")
	}
}

func TestHashFileReadErrorOnDirectory(t *testing.T) {
	// Reading a directory as a file is a real (non-ENOENT) error, distinct
	// from "missing file".
	if _, err := HashFile(t.TempDir()); err == nil {
		t.Fatal("expected an error hashing a directory")
	}
}

func TestHashFileMatchesHashBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	content := []byte("some content")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	h, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if h != HashBytes(content) {
		t.Errorf("HashFile() = %q, want %q", h, HashBytes(content))
	}
}
