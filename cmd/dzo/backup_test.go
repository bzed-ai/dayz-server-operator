// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bzed-ai/dayz-server-operator/internal/btrfs"
)

// btrfsData returns a data directory on btrfs (DZO_BTRFS_TESTDIR, or the
// user's cache directory), or skips. Subvolumes need btrfs.Delete to go.
func btrfsData(t *testing.T) string {
	t.Helper()
	base := os.Getenv("DZO_BTRFS_TESTDIR")
	if base == "" {
		c, err := os.UserCacheDir()
		if err != nil {
			t.Skip("no cache dir")
		}
		base = c
	}
	if err := os.MkdirAll(base, 0o750); err != nil {
		t.Skip(err)
	}
	if ok, err := btrfs.IsBtrfs(base); err != nil || !ok {
		t.Skipf("%s is not on btrfs; set DZO_BTRFS_TESTDIR", base)
	}
	data, err := os.MkdirTemp(base, "dzo-cli-test-")
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() {
		for _, sub := range []string{"snapshots/x", "instances"} {
			es, _ := os.ReadDir(filepath.Join(data, sub))
			for _, e := range es {
				_ = btrfs.Delete(filepath.Join(data, sub, e.Name()))
			}
		}
		_ = os.RemoveAll(data)
	})
	return data
}

func TestBackupAndRestoreThroughTheCLI(t *testing.T) {
	data := btrfsData(t)
	cfg, root, _ := renderSetupIn(t, data)
	if out, err := runCmd(t, "backup", "create", "x", "--config", cfg); err == nil {
		t.Fatalf("an instance that was never rendered is not a subvolume yet: %s", out)
	}
	if out, err := runCmd(t, "instance", "render", "x", "--config", cfg); err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	if ok, err := btrfs.IsSubvolume(root); err != nil || !ok {
		t.Fatalf("render must create the instance as a subvolume: %v %v", ok, err)
	}
	live := filepath.Join(root, "mpmissions", "empty.m")
	storage := filepath.Join(live, "storage_1", "persistence")
	writeFile(t, storage, "player data v1")

	out, err := runCmd(t, "backup", "create", "x", "--config", cfg)
	if err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	id := strings.Fields(out)[0]
	if !strings.HasSuffix(id, "-manual") {
		t.Fatalf("snapshot id = %q", id)
	}
	if out, err := runCmd(t, "backup", "list", "x", "--config", cfg); err != nil || !strings.Contains(out, id) || !strings.Contains(out, "complete") {
		t.Fatalf("list: %v\n%s", err, out)
	}
	writeFile(t, storage, "player data v2, changed")
	if out, err := runCmd(t, "backup", "diff", "x", id, "--config", cfg); err != nil || !strings.Contains(out, "~ mpmissions/empty.m/storage_1/persistence") {
		t.Fatalf("diff: %v\n%s", err, out)
	}
	if _, err := runCmd(t, "backup", "pin", "x", id, "--config", cfg); err != nil {
		t.Fatal(err)
	}
	if out, _ := runCmd(t, "backup", "list", "x", "--config", cfg); !strings.Contains(out, "pinned") {
		t.Errorf("pinned snapshots are marked: %s", out)
	}
	if _, err := runCmd(t, "backup", "unpin", "x", id, "--config", cfg); err != nil {
		t.Fatal(err)
	}
	if out, err := runCmd(t, "backup", "prune", "--dry-run", "--config", cfg); err != nil || strings.Contains(out, "would prune") {
		t.Errorf("one snapshot is within the policy: %v\n%s", err, out)
	}

	// a path restore, then a full restore, with the instance not running (no systemd here)
	lcFlags := []string{"--command", "false"} // `false is-active` fails: the unit is not active
	args := append([]string{"restore", "x", id, "--path", "mpmissions/empty.m/storage_1", "--config", cfg}, lcFlags...)
	if out, err := runCmd(t, args...); err != nil || !strings.Contains(out, "restored") {
		t.Fatalf("restore --path: %v\n%s", err, out)
	}
	if b, _ := os.ReadFile(storage); string(b) != "player data v1" {
		t.Errorf("the path must be back: %q", b)
	}
	writeFile(t, filepath.Join(root, "marker"), "live")
	args = append([]string{"restore", "x", id, "--config", cfg}, lcFlags...)
	if out, err := runCmd(t, args...); err != nil {
		t.Fatalf("restore: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "marker")); !os.IsNotExist(err) {
		t.Error("a full restore brings back the backup")
	}
	if ok, _ := btrfs.IsSubvolume(root); !ok {
		t.Error("the restored instance is a subvolume")
	}
	out, _ = runCmd(t, "backup", "list", "x", "--config", cfg)
	for _, reason := range []string{"pre_restore", "replaced"} {
		if !strings.Contains(out, reason) {
			t.Errorf("a restore leaves a %s snapshot:\n%s", reason, out)
		}
	}
	if _, err := runCmd(t, "backup", "create", "x", "--reason", "bogus", "--config", cfg); err == nil {
		t.Error("an unknown reason must fail")
	}
}

func TestRenderTakesTheSnapshotsThePolicyAsks(t *testing.T) {
	data := btrfsData(t)
	cfg, _, _ := renderSetupIn(t, data)
	site := filepath.Join(data, "site", "instances", "x", "instance.yaml")
	b, _ := os.ReadFile(site)
	writeFile(t, site, string(b)+"backup: {before: [render, mission_update]}\n")

	out, err := runCmd(t, "instance", "render", "x", "--config", cfg)
	if err != nil || strings.Contains(out, "snapshot ") {
		t.Fatalf("the first render has nothing to protect: %v\n%s", err, out)
	}
	if out, err = runCmd(t, "instance", "render", "x", "--config", cfg); err != nil || strings.Count(out, "snapshot ") != 1 {
		t.Fatalf("a later render takes the render snapshot: %v\n%s", err, out)
	}
	if out, err = runCmd(t, "instance", "render", "x", "--config", cfg, "--update-pristine"); err != nil || strings.Count(out, "snapshot ") != 2 {
		t.Fatalf("--update-pristine adds the mission_update snapshot: %v\n%s", err, out)
	}
	if out, err = runCmd(t, "instance", "render", "x", "--config", cfg, "--dry-run"); err != nil || strings.Contains(out, "snapshot ") {
		t.Fatalf("a dry run changes nothing: %v\n%s", err, out)
	}
}

func TestBackupNeedsBtrfs(t *testing.T) {
	cfg, _, _ := renderSetup(t) // t.TempDir may or may not be btrfs
	if ok, _ := btrfs.IsBtrfs(filepath.Dir(cfg)); ok {
		t.Skip("the temp dir is on btrfs")
	}
	if out, err := runCmd(t, "instance", "render", "x", "--config", cfg); err != nil {
		t.Fatalf("a render works without btrfs: %v\n%s", err, out)
	}
	if _, err := runCmd(t, "backup", "create", "x", "--config", cfg); err == nil || !strings.Contains(err.Error(), "btrfs") {
		t.Errorf("a backup without btrfs must say so: %v", err)
	}
}
