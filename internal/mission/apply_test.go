// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package mission

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestApplyWritesNewFiles(t *testing.T) {
	staging := t.TempDir()
	live := t.TempDir()
	filehistory := t.TempDir()
	writeTree(t, staging, map[string]string{"db/types.xml": "content-v1"})

	m := NewManifest()
	plan, err := Classify(staging, live, m, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	report, err := Apply(plan, staging, live, filehistory, m, Options{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(report.Written) != 1 {
		t.Fatalf("Written = %+v", report.Written)
	}
	if got := readString(t, filepath.Join(live, "db/types.xml")); got != "content-v1" {
		t.Errorf("live content = %q", got)
	}
	if m.Entries["db/types.xml"].SourceHash != HashBytes([]byte("content-v1")) {
		t.Errorf("manifest not updated: %+v", m.Entries["db/types.xml"])
	}
}

func TestApplyNewFileHasNothingToBackUp(t *testing.T) {
	staging := t.TempDir()
	live := t.TempDir()
	filehistory := t.TempDir()
	writeTree(t, staging, map[string]string{"db/types.xml": "v1"})

	m := NewManifest()
	plan, err := Classify(staging, live, m, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if _, err := Apply(plan, staging, live, filehistory, m, Options{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	entries, err := os.ReadDir(filehistory)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no filehistory entries for a brand-new file, got %v", entries)
	}
}

func TestApplyBacksUpDriftedFileBeforeOverwriting(t *testing.T) {
	staging := t.TempDir()
	live := t.TempDir()
	filehistory := t.TempDir()

	original := "v1"
	writeTree(t, staging, map[string]string{"db/types.xml": original})
	writeTree(t, live, map[string]string{"db/types.xml": "modified-by-something-else"})

	m := NewManifest()
	hash := HashBytes([]byte(original))
	m.Entries["db/types.xml"] = Entry{Origin: OriginPristine, SourceHash: hash, WrittenHash: hash}

	plan, err := Classify(staging, live, m, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	report, err := Apply(plan, staging, live, filehistory, m, Options{Now: func() time.Time { return time.Unix(1000000000, 0) }})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(report.Drifted) != 1 {
		t.Fatalf("Drifted = %+v", report.Drifted)
	}

	// The drifted content must be preserved in filehistory before being overwritten.
	tsDirs, err := os.ReadDir(filehistory)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(tsDirs) != 1 {
		t.Fatalf("expected exactly one filehistory generation, got %v", tsDirs)
	}
	backedUp := readString(t, filepath.Join(filehistory, tsDirs[0].Name(), "db/types.xml"))
	if backedUp != "modified-by-something-else" {
		t.Errorf("backed-up content = %q, want the pre-overwrite drifted content", backedUp)
	}

	// And the live file now has the staging content.
	if got := readString(t, filepath.Join(live, "db/types.xml")); got != original {
		t.Errorf("live content = %q, want %q", got, original)
	}
}

func TestApplyObsoleteNeverTouchesDisk(t *testing.T) {
	staging := t.TempDir() // no longer ships this file
	live := t.TempDir()
	filehistory := t.TempDir()
	writeTree(t, live, map[string]string{"mod_999/types.xml": "still here"})

	m := NewManifest()
	m.Entries["mod_999/types.xml"] = Entry{Origin: OriginGenerated, SourceHash: "h", WrittenHash: "h"}

	plan, err := Classify(staging, live, m, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	report, err := Apply(plan, staging, live, filehistory, m, Options{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(report.Obsolete) != 1 {
		t.Fatalf("Obsolete = %+v", report.Obsolete)
	}
	// The file itself is untouched on disk.
	if got := readString(t, filepath.Join(live, "mod_999/types.xml")); got != "still here" {
		t.Errorf("live content = %q, obsolete files must never be modified/deleted", got)
	}
	// But it's gone from the manifest (no longer "managed").
	if _, ok := m.Entries["mod_999/types.xml"]; ok {
		t.Error("obsolete entry should have been dropped from the manifest")
	}
}

func TestApplyUnchangedDoesNotRewriteFile(t *testing.T) {
	staging := t.TempDir()
	live := t.TempDir()
	filehistory := t.TempDir()
	content := "stable"
	writeTree(t, staging, map[string]string{"db/types.xml": content})
	writeTree(t, live, map[string]string{"db/types.xml": content})

	hash := HashBytes([]byte(content))
	m := NewManifest()
	m.Entries["db/types.xml"] = Entry{Origin: OriginPristine, SourceHash: hash, WrittenHash: hash}

	livePath := filepath.Join(live, "db/types.xml")
	before, err := os.Stat(livePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	plan, err := Classify(staging, live, m, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	report, err := Apply(plan, staging, live, filehistory, m, Options{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(report.Written) != 0 {
		t.Fatalf("Written = %+v, want none", report.Written)
	}

	after, err := os.Stat(livePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("an unchanged file must not be rewritten (mtime changed)")
	}
}

func TestApplyNeverWritesOutsidePlan(t *testing.T) {
	// A foreign file living in "live" (e.g. storage_1/*, or anything not
	// in the manifest/staging) must survive Apply completely untouched,
	// even when it shares a directory with managed files.
	staging := t.TempDir()
	live := t.TempDir()
	filehistory := t.TempDir()
	writeTree(t, staging, map[string]string{"db/types.xml": "new content"})
	writeTree(t, live, map[string]string{
		"storage_1/player.bin": "foreign game data",
	})

	plan, err := Classify(staging, live, NewManifest(), nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if _, err := Apply(plan, staging, live, filehistory, NewManifest(), Options{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := readString(t, filepath.Join(live, "storage_1/player.bin")); got != "foreign game data" {
		t.Fatalf("foreign file was modified: %q", got)
	}
}

func TestApplyPrunesOldFileHistory(t *testing.T) {
	staging := t.TempDir()
	live := t.TempDir()
	filehistory := t.TempDir()

	m := NewManifest()
	content := "v0"
	writeTree(t, staging, map[string]string{"db/types.xml": content})
	writeTree(t, live, map[string]string{"db/types.xml": content})
	hash := HashBytes([]byte(content))
	m.Entries["db/types.xml"] = Entry{Origin: OriginPristine, SourceHash: hash, WrittenHash: hash}

	// Cause 3 drift-and-overwrite cycles at 3 distinct timestamps, keeping
	// only the newest 2.
	base := time.Unix(2000000000, 0)
	for i := 0; i < 3; i++ {
		writeTree(t, live, map[string]string{"db/types.xml": "drifted"}) // force drift each round
		plan, err := Classify(staging, live, m, nil)
		if err != nil {
			t.Fatalf("Classify: %v", err)
		}
		now := base.Add(time.Duration(i) * time.Hour)
		if _, err := Apply(plan, staging, live, filehistory, m, Options{KeepFileHistory: 2, Now: func() time.Time { return now }}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}

	entries, err := os.ReadDir(filehistory)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 retained filehistory generations, got %d: %v", len(entries), entries)
	}
}

func TestApplyMissingStagingFileErrors(t *testing.T) {
	staging := t.TempDir()
	live := t.TempDir()
	filehistory := t.TempDir()

	// Craft a plan entry that doesn't actually exist in staging, to
	// exercise the error path (Classify itself would never produce this).
	plan := Plan{Entries: []PlanEntry{{Path: "missing.xml", Action: ActionNew, Origin: OriginPristine, StagingHash: "x"}}}
	if _, err := Apply(plan, staging, live, filehistory, NewManifest(), Options{}); err == nil {
		t.Fatal("expected an error when the staging file doesn't exist")
	}
}

func TestApplyWriteFailsWhenLiveDirBlockedByFile(t *testing.T) {
	staging := t.TempDir()
	live := t.TempDir()
	filehistory := t.TempDir()
	writeTree(t, staging, map[string]string{"db/types.xml": "v1"})

	plan, err := Classify(staging, live, NewManifest(), nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	// Only now block "live/db" with a plain file, after Classify has
	// already produced its plan against the (still-empty) live dir - this
	// isolates the failure to Apply's write step.
	if err := os.WriteFile(filepath.Join(live, "db"), []byte("blocker"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	if _, err := Apply(plan, staging, live, filehistory, NewManifest(), Options{}); err == nil {
		t.Fatal("expected an error when the live directory path is blocked by a file")
	}
}

func TestApplyEmptyPlanIsNoOp(t *testing.T) {
	staging := t.TempDir()
	live := t.TempDir()
	filehistory := t.TempDir()
	report, err := Apply(Plan{}, staging, live, filehistory, NewManifest(), Options{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(report.Written) != 0 || len(report.Obsolete) != 0 {
		t.Fatalf("report = %+v", report)
	}
	entries, err := os.ReadDir(filehistory)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Error("an empty plan must not create a filehistory generation")
	}
}
