// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package servermods

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bzed/dayz-server-operator/internal/moddeps"
	"github.com/bzed/dayz-server-operator/internal/product"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// fixture is a servermods/ tree with one mod holding one addon.
func fixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "servermods")
	src := filepath.Join(root, "demo", "src", "Demo")
	write(t, filepath.Join(src, PrefixFile), "Demo\n")
	write(t, filepath.Join(src, "config.cpp"), "class CfgPatches {};\n")
	write(t, filepath.Join(src, "scripts", "3_Game", "Demo", "A.c"), "class A {}\n")
	write(t, filepath.Join(src, "scripts", "5_Mission", "Demo", "B.c"), "class B {}\n")
	return root
}

func TestBuildProducesAValidLocalMod(t *testing.T) {
	root := fixture(t)
	out := filepath.Join(t.TempDir(), "out")
	res, err := Build(context.Background(), root, out)
	if err != nil || len(res) != 1 {
		t.Fatalf("Build: %v %v", res, err)
	}
	r := res[0]
	if r.Mod != "demo" || r.Addon != "Demo" || r.Prefix != "Demo" || len(r.SHA256) != 64 || filepath.Base(r.PBO) != "demo.pbo" {
		t.Fatalf("result = %+v", r)
	}
	// What dzo mod add would accept: addons/*.pbo with a prefix header.
	if err := product.ValidateMod(filepath.Join(out, "demo"), true); err != nil {
		t.Fatalf("ValidateMod: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "demo", "meta.cpp")); !strings.Contains(string(b), "publishedid = 0;") {
		t.Fatalf("meta.cpp = %q", b)
	}
	data, _ := os.ReadFile(r.PBO)
	p, err := moddeps.OpenPBO(data)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range p.Entries {
		names = append(names, strings.ReplaceAll(e.Name, `\`, "/"))
	}
	want := []string{"config.cpp", "scripts/3_Game/Demo/A.c", "scripts/5_Mission/Demo/B.c"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("entries = %v, want %v (sorted, no %s)", names, want, PrefixFile)
	}
}

func TestBuildIsReproducible(t *testing.T) {
	root := fixture(t)
	a, err := Build(context.Background(), root, filepath.Join(t.TempDir(), "a"))
	if err != nil {
		t.Fatal(err)
	}
	// A later mtime on a source file must not change the output.
	later := time.Now().Add(time.Hour)
	_ = os.Chtimes(filepath.Join(root, "demo", "src", "Demo", "config.cpp"), later, later)
	b, err := Build(context.Background(), root, filepath.Join(t.TempDir(), "b"))
	if err != nil {
		t.Fatal(err)
	}
	if a[0].SHA256 != b[0].SHA256 {
		t.Fatalf("hashes differ: %s vs %s", a[0].SHA256, b[0].SHA256)
	}
	// And a content change must.
	write(t, filepath.Join(root, "demo", "src", "Demo", "scripts", "3_Game", "Demo", "A.c"), "class A2 {}\n")
	c, _ := Build(context.Background(), root, filepath.Join(t.TempDir(), "c"))
	if c[0].SHA256 == a[0].SHA256 {
		t.Fatal("a changed script must change the hash")
	}
}

func TestBuildErrors(t *testing.T) {
	ctx := context.Background()
	out := t.TempDir()
	if _, err := Build(ctx, filepath.Join(t.TempDir(), "none"), out); err == nil || !strings.Contains(err.Error(), "no servermods") {
		t.Fatalf("empty root: %v", err)
	}
	empty := t.TempDir()
	if err := os.MkdirAll(filepath.Join(empty, "dzo-admin"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(ctx, empty, out); err == nil || !strings.Contains(err.Error(), "submodule") {
		t.Fatalf("unchecked submodule: %v", err)
	}
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "demo", "src", "Demo", PrefixFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(ctx, root, out); err == nil {
		t.Fatal("missing prefix file")
	}
	write(t, filepath.Join(root, "demo", "src", "Demo", PrefixFile), "  \n")
	if _, err := Build(ctx, root, out); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty prefix: %v", err)
	}
	only := t.TempDir()
	write(t, filepath.Join(only, PrefixFile), "X")
	if _, _, err := Pack(ctx, only, filepath.Join(out, "x.pbo")); err == nil || !strings.Contains(err.Error(), "no files") {
		t.Fatalf("no files: %v", err)
	}
	link := fixture(t)
	if err := os.Symlink("/etc/passwd", filepath.Join(link, "demo", "src", "Demo", "evil.c")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(ctx, link, out); err == nil || !strings.Contains(err.Error(), "regular") {
		t.Fatalf("symlink: %v", err)
	}
}
