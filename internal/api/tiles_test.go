// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTiles(t *testing.T) {
	e := newEnv(t)
	root := t.TempDir()
	set := filepath.Join(root, "m", "abcdef0123456789")
	if err := os.MkdirAll(filepath.Join(set, "0", "0"), 0o750); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(set, "metadata.json"), []byte(`{"map":"m"}`), 0o600)
	_ = os.WriteFile(filepath.Join(set, "0", "0", "0.jpg"), []byte("JPEG"), 0o600)
	if err := os.Symlink("abcdef0123456789", filepath.Join(root, "m", "current")); err != nil {
		t.Fatal(err)
	}
	tok := e.token(RoleViewer, nil, false)

	if code, _ := e.do("GET", "/api/v1/tiles/m/metadata.json", tok, ""); code != 404 {
		t.Fatalf("without a tiles directory every tile is 404, got %d", code)
	}
	e.srv.TilesDir = root
	if code, _ := e.do("GET", "/api/v1/tiles/m/metadata.json", "", ""); code != 401 {
		t.Fatalf("tiles need a token, got %d", code)
	}
	code, b := e.do("GET", "/api/v1/tiles/m/metadata.json", tok, "")
	if code != 200 || !strings.Contains(string(b), `"map":"m"`) {
		t.Fatalf("metadata: %d %s", code, b)
	}
	code, b = e.do("GET", "/api/v1/tiles/m/abcdef0123456789/0/0/0.jpg", tok, "")
	if code != 200 || string(b) != "JPEG" {
		t.Fatalf("tile: %d %s", code, b)
	}
	for _, p := range []string{
		"/api/v1/tiles/m/abcdef0123456789/0/0/1.jpg",    // not built
		"/api/v1/tiles/other/metadata.json",             // unknown map
		"/api/v1/tiles/m/current/0/0/0.jpg",             // only the hash is a valid tile set name
		"/api/v1/tiles/m/abcdef0123456789/0/0/0.png",    // wrong extension
		"/api/v1/tiles/m/abcdef0123456789/0/0/../0.jpg", // traversal
		"/api/v1/tiles/M/metadata.json",                 // map names are lower case
		"/api/v1/tiles/m/metadata.json/x",
	} {
		if code, _ := e.do("GET", p, tok, ""); code != 404 {
			t.Errorf("%s: want 404, got %d", p, code)
		}
	}
}
