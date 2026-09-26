// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissionDiffAndApply(t *testing.T) {
	staging := t.TempDir()
	live := t.TempDir()
	filehistory := t.TempDir()
	manifest := filepath.Join(t.TempDir(), "manifest.json")

	if err := os.MkdirAll(filepath.Join(staging, "db"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staging, "db/types.xml"), []byte("v1"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := runCmd(t, "mission", "diff", staging, live, "--manifest", manifest)
	if err != nil {
		t.Fatalf("mission diff: %v", err)
	}
	if !strings.Contains(out, "db/types.xml") || !strings.Contains(out, "new") {
		t.Errorf("diff output = %q", out)
	}

	out, err = runCmd(t, "mission", "apply", staging, live, "--manifest", manifest, "--filehistory", filehistory)
	if err != nil {
		t.Fatalf("mission apply: %v", err)
	}
	if !strings.Contains(out, "written: 1") {
		t.Errorf("apply output = %q", out)
	}

	data, err := os.ReadFile(filepath.Join(live, "db/types.xml"))
	if err != nil {
		t.Fatalf("read live file: %v", err)
	}
	if string(data) != "v1" {
		t.Errorf("live content = %q", data)
	}

	// Second apply with nothing changed should report no writes.
	out, err = runCmd(t, "mission", "diff", staging, live, "--manifest", manifest)
	if err != nil {
		t.Fatalf("mission diff (2nd): %v", err)
	}
	if !strings.Contains(out, "no changes") {
		t.Errorf("expected no changes on the second diff, got %q", out)
	}
}

func TestMissionDiffRequiresManifestFlag(t *testing.T) {
	if _, err := runCmd(t, "mission", "diff", t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("expected an error for missing --manifest")
	}
}

func TestMissionApplyRequiresFilehistoryFlag(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	if _, err := runCmd(t, "mission", "apply", t.TempDir(), t.TempDir(), "--manifest", manifest); err == nil {
		t.Fatal("expected an error for missing --filehistory")
	}
}

func TestMissionDiffWithUnmanagedGlobs(t *testing.T) {
	staging := t.TempDir()
	live := t.TempDir()
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.MkdirAll(filepath.Join(staging, "expansion"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staging, "expansion/x.json"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := runCmd(t, "mission", "diff", staging, live, "--manifest", manifest, "--unmanaged", "expansion/**")
	if err != nil {
		t.Fatalf("mission diff: %v", err)
	}
	if strings.Contains(out, "expansion") {
		t.Errorf("expansion/* should be excluded via --unmanaged, got %q", out)
	}
}
