// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package product

import (
	"strings"
	"testing"
)

func TestValidateMod(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string][]byte
		local   bool
		wantErr string
	}{
		{"workshop ok", modFiles(false), false, ""},
		{"local ok", modFiles(true), true, ""},
		{"workshop without meta.cpp", map[string][]byte{"addons/a.pbo": buildPBO("p")}, false, "meta.cpp missing"},
		{"workshop without prefix is allowed", map[string][]byte{"meta.cpp": nil, "addons/a.pbo": buildPBO("")}, false, ""},
		{"workshop without addons is allowed", map[string][]byte{"meta.cpp": nil}, false, ""},
		{"local without prefix", map[string][]byte{"addons/a.pbo": buildPBO("")}, true, "no prefix"},
		{"local without pbo", map[string][]byte{"addons/readme.txt": nil}, true, "no .pbo"},
		{"corrupt pbo", map[string][]byte{"meta.cpp": nil, "addons/a.pbo": []byte("garbage")}, false, "a.pbo"},
		{"empty pbo", map[string][]byte{"meta.cpp": nil, "addons/a.pbo": {}}, false, "empty PBO"},
		{"upper-case extension", map[string][]byte{"meta.cpp": nil, "addons/A.PBO": buildPBO("p")}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, tt.files)
			err := ValidateMod(dir, tt.local)
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("ValidateMod() = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
