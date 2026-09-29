// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package mission

import (
	"context"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo creates a repo with one commit holding files and returns its path
// (usable as a git "URL").
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeTree(t, dir, files)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "x"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

type tree struct{ pristine, fallback, live, manifest, history string }

func newTree(t *testing.T) tree {
	root := t.TempDir()
	return tree{
		pristine: filepath.Join(root, "servermpmissions", "m"), fallback: filepath.Join(root, "fallback"),
		live: filepath.Join(root, "mpmissions", "m"), manifest: filepath.Join(root, "mpmissions", ".dzo-manifest.json"),
		history: filepath.Join(root, "filehistory"),
	}
}

func (tr tree) render(t *testing.T, dry bool, unmanaged ...string) (Plan, Report) {
	t.Helper()
	fallback := tr.fallback
	if _, err := os.Stat(fallback); err != nil {
		fallback = "" // most tests have no fallback mission
	}
	plan, report, err := Render(RenderInput{
		PristineDir: tr.pristine, FallbackDir: fallback, LiveDir: tr.live, ManifestPath: tr.manifest,
		FileHistoryDir: tr.history, Unmanaged: unmanaged, DryRun: dry,
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return plan, report
}

func TestRenderInitThenIncremental(t *testing.T) {
	tr := newTree(t)
	writeTree(t, tr.pristine, map[string]string{"db/types.xml": "v1", "init.c": "c1"})
	writeTree(t, tr.fallback, map[string]string{"cfgweather.xml": "weather", "init.c": "fallback-init", "db/types.xml": "not-a-base-file"})

	if plan, _ := tr.render(t, true); len(plan.Changed()) != 3 {
		t.Fatalf("dry run plan = %+v", plan)
	}
	if _, err := os.Stat(tr.live); !os.IsNotExist(err) {
		t.Fatal("dry run must not create the live mission")
	}

	_, rep := tr.render(t, false)
	if len(rep.Written) != 3 {
		t.Fatalf("init wrote %d files, want 3 (types.xml, init.c, cfgweather.xml from fallback)", len(rep.Written))
	}
	if got := readString(t, filepath.Join(tr.live, "init.c")); got != "c1" {
		t.Errorf("init.c = %q: pristine must win over the fallback", got)
	}

	// The game creates foreign files; pristine gains one, changes one.
	writeTree(t, tr.live, map[string]string{"storage_1/data.bin": "player data", "expansion/x.json": "foreign"})
	writeTree(t, tr.pristine, map[string]string{"db/types.xml": "v2", "new.xml": "n"})
	_, rep = tr.render(t, false)
	if len(rep.Written) != 2 {
		t.Fatalf("second render wrote %+v, want types.xml update + new.xml", rep.Written)
	}
	for f, want := range map[string]string{"storage_1/data.bin": "player data", "expansion/x.json": "foreign", "db/types.xml": "v2"} {
		if got := readString(t, filepath.Join(tr.live, f)); got != want {
			t.Errorf("%s = %q, want %q", f, got, want)
		}
	}
	if plan, _ := tr.render(t, true); !plan.Empty() {
		t.Errorf("a third render must be a no-op, plan = %+v", plan)
	}
}

func TestRenderDriftIsBackedUp(t *testing.T) {
	tr := newTree(t)
	writeTree(t, tr.pristine, map[string]string{"cfgweather.xml": "p1"})
	tr.render(t, false)
	writeTree(t, tr.live, map[string]string{"cfgweather.xml": "admin edit"})
	writeTree(t, tr.pristine, map[string]string{"cfgweather.xml": "p2"})
	_, rep := tr.render(t, false)
	if len(rep.Drifted) != 1 {
		t.Fatalf("Drifted = %+v", rep.Drifted)
	}
	var backup string
	_ = filepath.WalkDir(tr.history, func(p string, d fs.DirEntry, _ error) error {
		if !d.IsDir() && strings.HasSuffix(p, "cfgweather.xml") {
			backup = readString(t, p)
		}
		return nil
	})
	if backup != "admin edit" {
		t.Errorf("filehistory backup = %q, want the drifted admin edit", backup)
	}
}

func TestRenderErrors(t *testing.T) {
	tr := newTree(t)
	if _, _, err := Render(RenderInput{PristineDir: tr.pristine, LiveDir: tr.live, ManifestPath: tr.manifest}); err == nil {
		t.Error("missing pristine dir must fail")
	}
	writeTree(t, tr.pristine, map[string]string{"a": "1"})
	if _, _, err := Render(RenderInput{PristineDir: tr.pristine, FallbackDir: tr.fallback, LiveDir: tr.live, ManifestPath: tr.manifest}); err == nil {
		t.Error("missing fallback dir must fail")
	}
	if err := os.MkdirAll(filepath.Dir(tr.manifest), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tr.manifest, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Render(RenderInput{PristineDir: tr.pristine, LiveDir: tr.live, ManifestPath: tr.manifest}); err == nil {
		t.Error("corrupt manifest must fail")
	}
	if _, err := os.Stat(tr.live); !os.IsNotExist(err) {
		t.Error("a failed render must not touch the live mission")
	}
}

func TestRenderFallbackUnreadable(t *testing.T) {
	tr := newTree(t)
	writeTree(t, tr.pristine, map[string]string{"a": "1"})
	writeTree(t, tr.fallback, map[string]string{"cfgweather.xml/x": "a dir where a file is expected"})
	if _, _, err := Render(RenderInput{PristineDir: tr.pristine, FallbackDir: tr.fallback, LiveDir: tr.live, ManifestPath: tr.manifest}); err == nil {
		t.Error("an unreadable fallback file must fail")
	}
}

// snapshot maps every file under root to its content.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			out[filepath.ToSlash(rel)] = readString(t, p)
		}
		return nil
	})
	return out
}

// TestRenderTouchesOnlyManagedFiles is the §C6 property: over random
// pristine/live trees and repeated renders, a render only ever writes paths
// that are in the manifest or new in pristine, and never touches storage_*
// or unmanaged paths, or any file dzo does not own.
func TestRenderTouchesOnlyManagedFiles(t *testing.T) {
	names := []string{"a.xml", "b.json", "db/types.xml", "env/z.xml", "mod_1/x.xml", "expansion/e.json", "storage_1/p.bin", "storage_2/db/q", "deep/er/f"}
	for seed := int64(0); seed < 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		tr := newTree(t)
		unmanaged := []string{"expansion/**"}
		for round := 0; round < 4; round++ {
			pristine := map[string]string{}
			for _, n := range names {
				if rng.Intn(2) == 0 {
					pristine[n] = fmt.Sprintf("p-%d-%d", seed, rng.Intn(3))
				}
			}
			if err := os.RemoveAll(tr.pristine); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(tr.pristine, 0o750); err != nil {
				t.Fatal(err)
			}
			writeTree(t, tr.pristine, pristine)
			// the game and admins write files dzo does not own
			foreign := map[string]string{
				"storage_1/p.bin": fmt.Sprintf("s-%d-%d", seed, round), "expansion/e.json": "foreign", fmt.Sprintf("mine-%d.txt", round): "x",
			}
			writeTree(t, tr.live, foreign)

			before := snapshot(t, tr.live)
			m, err := LoadManifest(tr.manifest)
			if err != nil {
				t.Fatal(err)
			}
			owned := map[string]bool{}
			for _, p := range m.Paths() {
				owned[p] = true
			}
			tr.render(t, false, unmanaged...)
			after := snapshot(t, tr.live)

			for path, was := range before {
				if after[path] == was {
					continue
				}
				top, _, _ := strings.Cut(path, "/")
				_, inPristine := pristine[path]
				switch {
				case strings.HasPrefix(top, "storage_"), MatchAny(unmanaged, path):
					t.Fatalf("seed %d round %d: %s must never change, was %q now %q", seed, round, path, was, after[path])
				case !owned[path] && !inPristine:
					t.Fatalf("seed %d round %d: foreign file %s changed", seed, round, path)
				}
			}
			for path := range after {
				top, _, _ := strings.Cut(path, "/")
				if _, existed := before[path]; existed {
					continue
				}
				if _, ok := pristine[path]; !ok || strings.HasPrefix(top, "storage_") || MatchAny(unmanaged, path) {
					t.Fatalf("seed %d round %d: render created %s, which pristine does not ship (or is excluded)", seed, round, path)
				}
			}
		}
	}
}

func TestFetchPristine(t *testing.T) {
	repo := gitRepo(t, map[string]string{"empty.map/init.c": "c", "empty.map/db/types.xml": "t", "other/x": "y"})
	dest := filepath.Join(t.TempDir(), "servermpmissions", "empty.map")
	if err := FetchPristine(context.Background(), repo, "main", "empty.*", dest); err != nil {
		t.Fatalf("FetchPristine: %v", err)
	}
	if got := snapshot(t, dest); len(got) != 2 || got["db/types.xml"] != "t" {
		t.Errorf("pristine = %v", got)
	}
	// a refetch replaces the tree
	writeTree(t, dest, map[string]string{"stale": "x"})
	if err := FetchPristine(context.Background(), repo, "main", "empty.map", dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "stale")); !os.IsNotExist(err) {
		t.Error("a refetch must replace the old pristine tree")
	}
}

func TestFetchPristineErrors(t *testing.T) {
	repo := gitRepo(t, map[string]string{"a/f": "1", "b/f": "2", "file": "x"})
	dest := filepath.Join(t.TempDir(), "p")
	writeTree(t, dest, map[string]string{"keep": "me"})
	tests := []struct{ name, url, ref, path string }{
		{"bad ref", repo, "nope", "a"},
		{"bad url", "/nonexistent/repo", "main", "a"},
		{"glob matches two", repo, "main", "?"},
		{"no match", repo, "main", "zzz"},
		{"not a dir", repo, "main", "file"},
		{"bad glob", repo, "main", "["},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := FetchPristine(context.Background(), tt.url, tt.ref, tt.path, dest); err == nil {
				t.Fatal("want an error")
			}
			if readString(t, filepath.Join(dest, "keep")) != "me" {
				t.Error("a failed fetch must leave the old pristine mission alone")
			}
		})
	}
	// dest's parent cannot be created below a regular file
	blocker := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := FetchPristine(context.Background(), repo, "main", "a", filepath.Join(blocker, "x", "p")); err == nil {
		t.Error("want an error for an uncreatable destination")
	}
}
