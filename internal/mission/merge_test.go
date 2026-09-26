// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bzed-ai/dayz-server-operator/internal/ce"
)

func TestCopyPristineCopiesTreeAndDirs(t *testing.T) {
	pristine := t.TempDir()
	staging := t.TempDir()
	writeTree(t, pristine, map[string]string{
		"db/types.xml": "types content",
		"init.c":       "init content",
		"env/x.xml":    "env content",
	})

	if err := CopyPristine(pristine, staging); err != nil {
		t.Fatalf("CopyPristine: %v", err)
	}
	for path, want := range map[string]string{
		"db/types.xml": "types content",
		"init.c":       "init content",
		"env/x.xml":    "env content",
	} {
		if got := readString(t, filepath.Join(staging, path)); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
}

func TestCopyPristineMissingSourceErrors(t *testing.T) {
	if err := CopyPristine(filepath.Join(t.TempDir(), "nope"), t.TempDir()); err == nil {
		t.Fatal("expected an error for a missing pristine dir")
	}
}

func TestWriteFileCreatesParentDirs(t *testing.T) {
	staging := t.TempDir()
	if err := WriteFile(staging, "mod_123/types.xml", []byte("hi")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if got := readString(t, filepath.Join(staging, "mod_123/types.xml")); got != "hi" {
		t.Errorf("content = %q", got)
	}
}

func TestMergeJSONFileCreatesWhenMissing(t *testing.T) {
	staging := t.TempDir()
	if err := MergeJSONFile(staging, "cfggameplay.json", []byte(`{"a": 1}`), nil); err != nil {
		t.Fatalf("MergeJSONFile: %v", err)
	}
	got := readString(t, filepath.Join(staging, "cfggameplay.json"))
	if !strings.Contains(got, `"a": 1`) {
		t.Errorf("content = %q", got)
	}
}

func TestMergeJSONFileMergesWithExisting(t *testing.T) {
	staging := t.TempDir()
	if err := WriteFile(staging, "cfggameplay.json", []byte(`{"WorldsData": {"objectSpawnersArr": ["a.json"]}}`)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	overlay := []byte(`{"WorldsData": {"objectSpawnersArr": ["b.json"]}}`)
	if err := MergeJSONFile(staging, "cfggameplay.json", overlay, []string{"objectSpawnersArr"}); err != nil {
		t.Fatalf("MergeJSONFile: %v", err)
	}
	got := readString(t, filepath.Join(staging, "cfggameplay.json"))
	if !strings.Contains(got, "a.json") || !strings.Contains(got, "b.json") {
		t.Errorf("expected both entries appended, got %q", got)
	}
}

func TestMergeJSONFileInvalidOverlayErrors(t *testing.T) {
	staging := t.TempDir()
	if err := MergeJSONFile(staging, "x.json", []byte("not json"), nil); err == nil {
		t.Fatal("expected an error for invalid overlay JSON")
	}
}

func TestMergeXMLFileCreatesRootWhenMissing(t *testing.T) {
	staging := t.TempDir()
	overlay := []byte(`<eventgroupdef><eventgroup name="A"/></eventgroupdef>`)
	if err := MergeXMLFile(staging, "cfgeventgroups.xml", "eventgroupdef", overlay, "name"); err != nil {
		t.Fatalf("MergeXMLFile: %v", err)
	}
	got := readString(t, filepath.Join(staging, "cfgeventgroups.xml"))
	if !strings.Contains(got, `name="A"`) {
		t.Errorf("content = %q", got)
	}
}

func TestMergeXMLFileMergesWithExisting(t *testing.T) {
	staging := t.TempDir()
	if err := WriteFile(staging, "cfgeventgroups.xml", []byte(`<eventgroupdef><eventgroup name="A"/></eventgroupdef>`)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	overlay := []byte(`<eventgroupdef><eventgroup name="B"/></eventgroupdef>`)
	if err := MergeXMLFile(staging, "cfgeventgroups.xml", "eventgroupdef", overlay, "name"); err != nil {
		t.Fatalf("MergeXMLFile: %v", err)
	}
	got := readString(t, filepath.Join(staging, "cfgeventgroups.xml"))
	if !strings.Contains(got, `name="A"`) || !strings.Contains(got, `name="B"`) {
		t.Errorf("expected both A and B present, got %q", got)
	}
}

func TestMergeXMLFileInvalidOverlayErrors(t *testing.T) {
	staging := t.TempDir()
	if err := MergeXMLFile(staging, "x.xml", "root", []byte("<not valid xml"), "name"); err == nil {
		t.Fatal("expected an error for invalid overlay XML")
	}
}

func TestMergeXMLFileInvalidExistingErrors(t *testing.T) {
	staging := t.TempDir()
	if err := WriteFile(staging, "x.xml", []byte("<broken")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := MergeXMLFile(staging, "x.xml", "root", []byte("<root/>"), "name"); err == nil {
		t.Fatal("expected an error for invalid existing staging XML")
	}
}

func TestRegisterCEFolderCreatesEconomyRoot(t *testing.T) {
	staging := t.TempDir()
	files := []ce.CEFile{{Name: "types.xml", Type: "types"}}
	if err := RegisterCEFolder(staging, "cfgeconomycore.xml", "mod_123", files); err != nil {
		t.Fatalf("RegisterCEFolder: %v", err)
	}
	got := readString(t, filepath.Join(staging, "cfgeconomycore.xml"))
	if !strings.Contains(got, `folder="mod_123"`) || !strings.Contains(got, `name="types.xml"`) {
		t.Errorf("content = %q", got)
	}
}

func TestRegisterCEFolderAppendsInOrder(t *testing.T) {
	staging := t.TempDir()
	if err := RegisterCEFolder(staging, "cfgeconomycore.xml", "mod_1", nil); err != nil {
		t.Fatalf("RegisterCEFolder: %v", err)
	}
	if err := RegisterCEFolder(staging, "cfgeconomycore.xml", "mod_2", nil); err != nil {
		t.Fatalf("RegisterCEFolder: %v", err)
	}
	got := readString(t, filepath.Join(staging, "cfgeconomycore.xml"))
	idx1 := strings.Index(got, "mod_1")
	idx2 := strings.Index(got, "mod_2")
	if idx1 < 0 || idx2 < 0 || idx1 > idx2 {
		t.Errorf("expected mod_1 registered before mod_2, got %q", got)
	}
}

func TestRegisterCEFolderInvalidExistingErrors(t *testing.T) {
	staging := t.TempDir()
	if err := WriteFile(staging, "cfgeconomycore.xml", []byte("<broken")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := RegisterCEFolder(staging, "cfgeconomycore.xml", "mod_1", nil); err == nil {
		t.Fatal("expected an error for invalid existing cfgeconomycore.xml")
	}
}

func TestRegisterCEFolderWrongRootErrors(t *testing.T) {
	staging := t.TempDir()
	if err := WriteFile(staging, "cfgeconomycore.xml", []byte("<notEconomy/>")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := RegisterCEFolder(staging, "cfgeconomycore.xml", "mod_1", nil); err == nil {
		t.Fatal("expected an error for a non-<economycore> root")
	}
}

func TestMergeJSONFileWriteErrorPropagates(t *testing.T) {
	staging := t.TempDir()
	// "blocked" exists as a file, so writing "blocked/x.json" must fail
	// at the mkdir step inside writeFileAtomic.
	if err := os.WriteFile(filepath.Join(staging, "blocked"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	if err := MergeJSONFile(staging, "blocked/x.json", []byte("{}"), nil); err == nil {
		t.Fatal("expected an error when the target path is blocked")
	}
}
