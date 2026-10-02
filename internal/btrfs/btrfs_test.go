// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package btrfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// testDir returns a scratch directory on a btrfs filesystem, or skips: set
// DZO_BTRFS_TESTDIR to a directory on btrfs that the test user may write to.
// Without it the user's cache directory is tried.
func testDir(t *testing.T) string {
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
	if ok, err := IsBtrfs(base); err != nil || !ok {
		t.Skipf("%s is not on btrfs; set DZO_BTRFS_TESTDIR", base)
	}
	dir, err := os.MkdirTemp(base, "dzo-btrfs-test-")
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() {
		// snapshots and subvolumes need Delete, RemoveAll cannot remove a subvolume with the read-only flag
		es, _ := os.ReadDir(dir)
		for _, e := range es {
			_ = Delete(filepath.Join(dir, e.Name()))
		}
		_ = os.RemoveAll(dir)
	})
	return dir
}

func TestSubvolumeSnapshotAndDelete(t *testing.T) {
	dir := testDir(t)
	sub := filepath.Join(dir, "inst")
	if err := CreateSubvolume(sub); err != nil {
		t.Fatalf("create: %v", err)
	}
	if ok, err := IsSubvolume(sub); err != nil || !ok {
		t.Fatalf("a created subvolume must be one: %v %v", ok, err)
	}
	plain := filepath.Join(dir, "plain")
	if err := os.Mkdir(plain, 0o750); err != nil {
		t.Fatal(err)
	}
	if ok, _ := IsSubvolume(plain); ok {
		t.Error("a plain directory is not a subvolume")
	}
	if err := os.MkdirAll(filepath.Join(sub, "mpmissions", "m", "storage_1"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "mpmissions", "m", "storage_1", "data"), []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}

	snap := filepath.Join(dir, "20261002-120000-manual")
	if err := Snapshot(sub, snap, true); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if ro, err := ReadOnly(snap); err != nil || !ro {
		t.Fatalf("a read-only snapshot must be read-only: %v %v", ro, err)
	}
	if b, _ := os.ReadFile(filepath.Join(snap, "mpmissions", "m", "storage_1", "data")); string(b) != "v1" {
		t.Errorf("snapshot content = %q", b)
	}
	if err := os.WriteFile(filepath.Join(snap, "x"), []byte("x"), 0o600); !errors.Is(err, unix.EROFS) {
		t.Errorf("writing into a read-only snapshot must fail with EROFS, got %v", err)
	}
	// the live data moves on, the snapshot does not
	if err := os.WriteFile(filepath.Join(sub, "mpmissions", "m", "storage_1", "data"), []byte("v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(snap, "mpmissions", "m", "storage_1", "data")); string(b) != "v1" {
		t.Errorf("a snapshot must not follow the live data: %q", b)
	}

	// a writable snapshot of the backup is how a restore brings it back
	restored := filepath.Join(dir, "restored")
	if err := Snapshot(snap, restored, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(restored, "x"), []byte("x"), 0o600); err != nil {
		t.Errorf("a restored instance must be writable: %v", err)
	}

	for _, p := range []string{snap, restored, sub} {
		if err := Delete(p); err != nil {
			t.Fatalf("delete %s: %v", p, err)
		}
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists after Delete", p)
		}
	}
}

func TestSetReadOnlyRoundTrip(t *testing.T) {
	dir := testDir(t)
	sub := filepath.Join(dir, "s")
	if err := CreateSubvolume(sub); err != nil {
		t.Fatal(err)
	}
	if err := SetReadOnly(sub, true); err != nil {
		t.Fatal(err)
	}
	if ro, _ := ReadOnly(sub); !ro {
		t.Error("read-only flag not set")
	}
	if err := SetReadOnly(sub, false); err != nil {
		t.Fatal(err)
	}
	if ro, _ := ReadOnly(sub); ro {
		t.Error("read-only flag not cleared")
	}
}

func TestErrors(t *testing.T) {
	dir := testDir(t)
	if err := Snapshot(filepath.Join(dir, "missing"), filepath.Join(dir, "x"), true); err == nil {
		t.Error("a snapshot of a missing source must fail")
	}
	plain := filepath.Join(dir, "plain")
	_ = os.Mkdir(plain, 0o750)
	if err := Snapshot(plain, filepath.Join(dir, "x"), true); err == nil {
		t.Error("a plain directory cannot be snapshotted")
	}
	sub := filepath.Join(dir, "s")
	if err := CreateSubvolume(sub); err != nil {
		t.Fatal(err)
	}
	if err := CreateSubvolume(sub); err == nil {
		t.Error("creating a subvolume that exists must fail")
	}
	for _, bad := range []string{"", "a/b", strings.Repeat("x", 5000)} {
		if err := CreateSubvolume(filepath.Join(dir, bad)); bad != "" && err == nil {
			t.Errorf("name %q must be refused", bad)
		}
	}
	if err := Delete(filepath.Join(dir, "nothing")); err == nil {
		t.Error("deleting a missing path must fail")
	}
	if ok, err := IsBtrfs("/proc"); err != nil || ok {
		t.Errorf("/proc is not btrfs: %v %v", ok, err)
	}
	if _, err := IsBtrfs(filepath.Join(dir, "nope")); err == nil {
		t.Error("a missing path is an error")
	}
}

func TestSameFilesystemAndMountOptions(t *testing.T) {
	dir := testDir(t)
	if ok, err := SameFilesystem(dir, filepath.Join(dir)); err != nil || !ok {
		t.Errorf("a directory is on its own filesystem: %v %v", ok, err)
	}
	if ok, err := SameFilesystem(dir, "/proc"); err != nil || ok {
		t.Errorf("/proc is another filesystem: %v %v", ok, err)
	}
	opts, err := MountOptions(dir)
	if err != nil || !strings.Contains(opts, "btrfs") && !strings.Contains(opts, "rw") {
		t.Errorf("mount options of a btrfs dir: %q %v", opts, err)
	}
	_ = UserSubvolRmAllowed(dir) // either is fine; it must not panic
}
