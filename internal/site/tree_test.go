// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

const goodInstance = `name: a
product: dayz-stable
map: m
mission_source: {preset: vanilla}
ports: {game: 2302}
network: host
mods: [{local: tools, server: true}]
`

func TestLoadTree(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "site.yaml", "image: img\nlocal_mods:\n  tools: {path: /srv/tools}\n")
	write(t, dir, "integrations/maps/vanilla.yaml", "git: g\nref: r\npath: p\n")
	write(t, dir, "instances/a/instance.yaml", goodInstance)
	tree, err := LoadTree(dir)
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	if tree.Site.Image != "img" || tree.Maps["vanilla"].Ref != "r" || len(tree.InstanceNames()) != 1 {
		t.Errorf("tree = %+v", tree)
	}
	if _, err := tree.Instance("a"); err != nil {
		t.Error(err)
	}
	if _, err := tree.Instance("zz"); err == nil {
		t.Error("want an error for an unknown instance")
	}
}

func TestLoadTreeEmptyAndMissingSiteYAML(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadTree(dir); err != nil {
		t.Fatalf("no site.yaml: %v", err)
	}
	write(t, dir, "site.yaml", "")
	if _, err := LoadTree(dir); err != nil {
		t.Fatalf("empty site.yaml: %v", err)
	}
}

func TestLoadTreeErrors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"unknown key", map[string]string{"site.yaml": "imagee: x\n"}, "imagee"},
		{"bad local mod", map[string]string{"site.yaml": "local_mods: {x: {url: u}}\n"}, "sha256 is required"},
		{"local mod both", map[string]string{"site.yaml": "local_mods: {x: {url: u, sha256: s, path: /p}}\n"}, "exactly one"},
		{"local mod rel path", map[string]string{"site.yaml": "local_mods: {x: {path: rel}}\n"}, "absolute"},
		{"bad preset", map[string]string{"integrations/maps/v.yaml": "gitt: x\n"}, "gitt"},
		{"name mismatch", map[string]string{"instances/b/instance.yaml": goodInstance}, "does not match directory"},
		{"invalid instance", map[string]string{"instances/a/instance.yaml": "name: a\n"}, "product is required"},
		{"bad instance yaml", map[string]string{"instances/a/instance.yaml": "name: [\n"}, "parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for rel, c := range tt.files {
				write(t, dir, rel, c)
			}
			if _, err := LoadTree(dir); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadTree() = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestLoadTreeUnreadableSiteYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "site.yaml"), 0o750); err != nil { // a directory, not a file
		t.Fatal(err)
	}
	if _, err := LoadTree(dir); err == nil {
		t.Fatal("want a read error")
	}
}

func TestAddMod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "instances", "a", "instance.yaml")
	write(t, dir, "instances/a/instance.yaml", "# my server\nname: a\nproduct: dayz-stable   # stable\nmap: m\nmission_source: {preset: vanilla}\nports: {game: 2302}\nnetwork: host\n")

	if err := AddMod(path, ModRef{ID: 1559212036}); err != nil {
		t.Fatalf("AddMod: %v", err)
	}
	if err := AddMod(path, ModRef{Local: "tools", Server: true}); err != nil {
		t.Fatalf("AddMod local: %v", err)
	}
	got, _ := os.ReadFile(path)
	for _, want := range []string{"# my server", "# stable", "- {id: 1559212036}", "- {local: tools, server: true}"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("file lacks %q:\n%s", want, got)
		}
	}
	// the result still loads
	write(t, dir, "integrations/maps/vanilla.yaml", "git: g\nref: r\npath: p\n")
	tree, err := LoadTree(dir)
	if err != nil || len(tree.Instances["a"].Mods) != 2 {
		t.Fatalf("LoadTree = %v, %v", tree, err)
	}
	if err := AddMod(path, ModRef{ID: 1559212036}); err == nil || !strings.Contains(err.Error(), "already lists") {
		t.Errorf("duplicate AddMod = %v", err)
	}
}

func TestAddModErrors(t *testing.T) {
	dir := t.TempDir()
	if err := AddMod(filepath.Join(dir, "missing.yaml"), ModRef{ID: 1}); err == nil {
		t.Error("missing file must fail")
	}
	if err := AddMod(filepath.Join(dir, "x.yaml"), ModRef{}); err == nil {
		t.Error("an invalid ref must fail")
	}
	write(t, dir, "list.yaml", "- a\n")
	if err := AddMod(filepath.Join(dir, "list.yaml"), ModRef{ID: 1}); err == nil || !strings.Contains(err.Error(), "not a YAML mapping") {
		t.Errorf("err = %v", err)
	}
	write(t, dir, "bad.yaml", "a: [\n")
	if err := AddMod(filepath.Join(dir, "bad.yaml"), ModRef{ID: 1}); err == nil {
		t.Error("broken YAML must fail")
	}
}
