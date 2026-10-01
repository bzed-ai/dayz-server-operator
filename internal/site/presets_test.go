// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package site

import (
	"os"
	"path/filepath"
	"testing"
)

// The presets the package ships must load as a site's integrations/maps.
func TestShippedMapPresetsLoad(t *testing.T) {
	site := t.TempDir()
	dst := filepath.Join(site, "integrations", "maps")
	if err := os.MkdirAll(dst, 0o750); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob("../../contrib/presets/maps/*.yaml")
	if len(files) != 3 {
		t.Fatalf("want 3 presets, found %v", files)
	}
	for _, f := range files {
		b, err := os.ReadFile(f) //nolint:gosec // test fixture
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, filepath.Base(f)), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := LoadTree(site)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := tree.Maps["vanilla-enoch"]
	if !ok || p.Git == "" || p.Ref == "" || p.Path != "dayzOffline.enoch" {
		t.Fatalf("presets = %+v", tree.Maps)
	}
}
