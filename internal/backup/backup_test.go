// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bzed/dayz-server-operator/internal/btrfs"
)

// fakeFS copies directories where btrfs would snapshot them.
type fakeFS struct {
	free         uint64
	failSnapshot bool
	failDelete   map[string]int // path suffix -> how many times Delete fails
	deleted      []string
}

func (f *fakeFS) Snapshot(src, dst string, _ bool) error {
	if f.failSnapshot {
		return errors.New("snapshot failed")
	}
	if _, err := os.Lstat(dst); err == nil {
		return errors.New("exists")
	}
	return copyTree(src, dst)
}
func (f *fakeFS) SetReadOnly(string, bool) error { return nil }
func (f *fakeFS) Delete(p string) error {
	for suffix, n := range f.failDelete {
		if strings.HasSuffix(p, suffix) && n > 0 {
			f.failDelete[suffix]--
			return errors.New("busy")
		}
	}
	f.deleted = append(f.deleted, filepath.Base(p))
	return os.RemoveAll(p)
}
func (f *fakeFS) FreeBytes(string) (uint64, error) { return f.free, nil }

type env struct {
	t   *testing.T
	m   *Manager
	fs  *fakeFS
	now time.Time
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	e := &env{t: t, fs: &fakeFS{free: 1 << 40}, now: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	e.m = &Manager{
		Instance: "hashima", InstanceDir: filepath.Join(root, "instances", "hashima"), Root: filepath.Join(root, "snapshots", "hashima"),
		Policy: Policy{Keep: 20, MinKeep: 1}, FS: e.fs, Now: func() time.Time { return e.now },
	}
	write(t, filepath.Join(e.m.InstanceDir, "mpmissions", "m", "storage_1", "data"), "v1")
	write(t, filepath.Join(e.m.InstanceDir, "profiles", "log"), "l")
	return e
}

func (e *env) create(reason Reason) Snapshot {
	e.t.Helper()
	e.now = e.now.Add(time.Minute)
	res, err := e.m.Create(context.Background(), reason)
	if err != nil {
		e.t.Fatalf("create %s: %v", reason, err)
	}
	return res.Snapshot
}

func TestCreateRecordsASnapshot(t *testing.T) {
	e := newEnv(t)
	s := e.create(Manual)
	if s.State != Complete || s.Reason != Manual || !strings.HasSuffix(s.ID, "-manual") || !strings.HasPrefix(s.ID, "20261001-120100") {
		t.Fatalf("snapshot = %+v", s)
	}
	if read(filepath.Join(e.m.Root, s.ID, "mpmissions", "m", "storage_1", "data")) != "v1" {
		t.Error("the snapshot must hold the instance's files")
	}
	write(t, filepath.Join(e.m.InstanceDir, "mpmissions", "m", "storage_1", "data"), "v2")
	if read(filepath.Join(e.m.Root, s.ID, "mpmissions", "m", "storage_1", "data")) != "v1" {
		t.Error("a snapshot must not follow the instance")
	}
	ss, err := e.m.List()
	if err != nil || len(ss) != 1 || ss[0].ID != s.ID {
		t.Fatalf("list = %+v %v", ss, err)
	}
	// the same second, the same reason: a different id
	e.m.Now = func() time.Time { return e.now }
	r2, err := e.m.Create(context.Background(), Manual)
	if err != nil || r2.Snapshot.ID == s.ID || !strings.HasSuffix(r2.Snapshot.ID, "-manual-2") {
		t.Errorf("second id = %q %v", r2.Snapshot.ID, err)
	}
	if _, err := e.m.Create(context.Background(), "bogus"); err == nil {
		t.Error("an unknown reason must be refused")
	}
}

func TestFailedCreateLeavesNothingBehind(t *testing.T) {
	e := newEnv(t)
	e.fs.failSnapshot = true
	if _, err := e.m.Create(context.Background(), Manual); err == nil {
		t.Fatal("must fail")
	}
	if ss, _ := e.m.List(); len(ss) != 0 {
		t.Errorf("a failed snapshot must not be in the index: %+v", ss)
	}
}

func TestRetentionRunsAfterACreate(t *testing.T) {
	e := newEnv(t)
	e.m.Policy = Policy{Keep: 2, MinKeep: 1}
	first := e.create(Manual)
	if err := e.m.Pin(first.ID, true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		e.create(Scheduled)
	}
	ss, _ := e.m.List()
	if len(ss) != 2 { // keep is the total: the pinned one and the newest
		var ids []string
		for _, s := range ss {
			ids = append(ids, s.ID)
		}
		t.Errorf("want the pinned and the newest, got %v", ids)
	}
	if _, err := os.Stat(filepath.Join(e.m.Root, first.ID)); err != nil {
		t.Error("the pinned snapshot must stay")
	}
	if len(e.fs.deleted) != 3 {
		t.Errorf("deleted %v", e.fs.deleted)
	}
}

func TestPruneDryRunAndInterruptedWork(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 4; i++ {
		e.create(Manual)
	}
	e.m.Policy = Policy{Keep: 2, MinKeep: 1}
	would, err := e.m.Prune(true)
	if err != nil || len(would) != 2 || len(e.fs.deleted) != 0 {
		t.Fatalf("dry run: %v %v deleted %v", would, err, e.fs.deleted)
	}

	// a deletion that fails stays "deleting" and is finished by the next run
	e.fs.failDelete = map[string]int{would[0].ID: 1}
	if _, err := e.m.Prune(false); err == nil {
		t.Fatal("a failing delete must be reported")
	}
	raw, _ := e.m.load() // without the recovery that every operation starts with
	var states []State
	for _, s := range raw {
		states = append(states, s.State)
	}
	if len(raw) != 3 || !contains(states, Deleting) {
		t.Fatalf("after a failed delete the entry stays, marked deleting: %+v", raw)
	}
	if ss, _ := e.m.List(); len(ss) != 2 { // the next operation finishes the deletion
		t.Errorf("the resumed deletion must finish: %+v", ss)
	}

	// an interrupted creation is cleaned up
	write(t, filepath.Join(e.m.Root, "20261001-130000-manual", "half"), "x")
	ss, _ := e.m.List()
	ss = append(ss, Snapshot{ID: "20261001-130000-manual", Reason: Manual, State: Creating, Created: e.now})
	if err := e.m.save(ss); err != nil {
		t.Fatal(err)
	}
	if ss, _ := e.m.List(); len(ss) != 2 {
		t.Errorf("a creating entry must be dropped: %+v", ss)
	}
	if _, err := os.Stat(filepath.Join(e.m.Root, "20261001-130000-manual")); !os.IsNotExist(err) {
		t.Error("the half-made snapshot must be deleted")
	}
}

func contains(ss []State, s State) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func TestMissingSnapshotsAreMarked(t *testing.T) {
	e := newEnv(t)
	s := e.create(Manual)
	if err := os.RemoveAll(filepath.Join(e.m.Root, s.ID)); err != nil {
		t.Fatal(err)
	}
	got, err := e.m.Get(s.ID)
	if err != nil || got.State != Missing {
		t.Errorf("a snapshot whose directory is gone is missing: %+v %v", got, err)
	}
	if _, err := e.m.Path(s.ID); err == nil {
		t.Error("a missing snapshot has no path")
	}
	if _, err := e.m.Get("nope"); err == nil {
		t.Error("unknown id")
	}
}

func TestOrphansAreReportedNotDeleted(t *testing.T) {
	e := newEnv(t)
	e.create(Manual)
	write(t, filepath.Join(e.m.Root, "20260101-000000-manual", "f"), "x")
	write(t, filepath.Join(e.m.Root, "stranger", "f"), "x")
	o, err := e.m.Orphans()
	if err != nil || len(o) != 2 {
		t.Fatalf("orphans = %v %v", o, err)
	}
	if _, err := e.m.Prune(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.m.Root, "stranger")); err != nil {
		t.Error("prune must never touch what is not in the index")
	}
	if err := e.m.AdoptOrphans([]string{"20260101-000000-manual"}); err != nil {
		t.Fatal(err)
	}
	if s, err := e.m.Get("20260101-000000-manual"); err != nil || s.Created.Year() != 2026 || s.Created.Month() != 1 {
		t.Errorf("adopted = %+v %v", s, err)
	}
	if err := e.m.DeleteOrphans([]string{"stranger"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.m.Root, "stranger")); !os.IsNotExist(err) {
		t.Error("the orphan must be deleted when asked")
	}
	if err := e.m.DeleteOrphans([]string{e.create(Manual).ID}); err == nil {
		t.Error("an indexed snapshot is not an orphan")
	}
	if err := e.m.AdoptOrphans([]string{"nothing"}); err == nil {
		t.Error("adopting a name that is not an orphan must fail")
	}
}

func TestSafePathRefusesAnythingButAChild(t *testing.T) {
	e := newEnv(t)
	for _, bad := range []string{"", "..", ".", "a/b", "../x", indexFile, lockFile} {
		if _, err := e.m.safePath(bad); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
	if err := os.MkdirAll(e.m.Root, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(e.m.InstanceDir, filepath.Join(e.m.Root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.safePath("link"); err == nil {
		t.Error("a symlink must be refused: deleting it must never reach the instance")
	}
}

func TestFreeSpaceIsMadeBeforeASnapshot(t *testing.T) {
	e := newEnv(t)
	e.m.Policy = Policy{Keep: 20, MinKeep: 2, MinFreeBytes: 100}
	for i := 0; i < 4; i++ {
		e.create(Manual)
	}
	e.fs.free = 10 // too little
	res, err := e.m.Create(context.Background(), Manual)
	if err != nil {
		t.Fatal(err)
	}
	ss, _ := e.m.List()
	// room is made down to min_keep 2, then the new one is taken; free space never improves in this fake
	if len(ss) != 3 || len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "nothing may be pruned") {
		t.Errorf("snapshots %d, warnings %v", len(ss), res.Warnings)
	}
}

func TestPostBackupHook(t *testing.T) {
	e := newEnv(t)
	var got string
	e.m.PostBackup = func(_ context.Context, p string) error { got = p; return errors.New("restic failed") }
	e.now = e.now.Add(time.Minute)
	res, err := e.m.Create(context.Background(), Manual)
	if err != nil {
		t.Fatalf("a failing hook must not fail the backup: %v", err)
	}
	if got != filepath.Join(e.m.Root, res.Snapshot.ID) || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "post_backup hook: restic failed") {
		t.Errorf("hook got %q, warnings %v", got, res.Warnings)
	}
}

func TestRestoreFull(t *testing.T) {
	e := newEnv(t)
	s := e.create(Manual)
	write(t, filepath.Join(e.m.InstanceDir, "mpmissions", "m", "storage_1", "data"), "v2-newer")
	write(t, filepath.Join(e.m.InstanceDir, "extra"), "only live")

	var order []string
	e.now = e.now.Add(time.Hour)
	res, err := e.m.Restore(context.Background(), s.ID, RestoreOptions{
		Stop:  func(context.Context) error { order = append(order, "stop"); return nil },
		Start: func(context.Context) error { order = append(order, "start"); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "stop,start" {
		t.Errorf("order = %v", order)
	}
	if read(filepath.Join(e.m.InstanceDir, "mpmissions", "m", "storage_1", "data")) != "v1" {
		t.Error("the backup must be the live instance now")
	}
	if _, err := os.Stat(filepath.Join(e.m.InstanceDir, "extra")); !os.IsNotExist(err) {
		t.Error("what was not in the backup is gone from the live instance")
	}
	ss, _ := e.m.List()
	reasons := map[Reason]bool{}
	for _, x := range ss {
		reasons[x.Reason] = true
	}
	if !reasons[PreRestore] || !reasons[Replaced] || res.Snapshot.Reason != PreRestore {
		t.Errorf("a restore keeps a safety snapshot and the replaced data: %+v", ss)
	}
	for _, x := range ss {
		if x.Reason == Replaced && read(filepath.Join(e.m.Root, x.ID, "extra")) != "only live" {
			t.Error("the replaced snapshot must hold what was live before")
		}
	}
	if aside, _ := filepath.Glob(filepath.Join(filepath.Dir(e.m.InstanceDir), ".hashima.replaced-*")); len(aside) != 0 {
		t.Errorf("the aside copy must have moved into the snapshots: %v", aside)
	}
}

func TestRestoreFullFailureLeavesTheInstanceAlone(t *testing.T) {
	e := newEnv(t)
	s := e.create(Manual)
	write(t, filepath.Join(e.m.InstanceDir, "marker"), "live")
	calls := 0
	e.m.FS = &countingFS{fakeFS: e.fs, failOn: 2, calls: &calls} // the safety snapshot works, the restore does not
	_, err := e.m.Restore(context.Background(), s.ID, RestoreOptions{})
	if err == nil || !strings.Contains(err.Error(), "the instance is as it was") {
		t.Fatalf("err = %v", err)
	}
	if read(filepath.Join(e.m.InstanceDir, "marker")) != "live" {
		t.Error("the instance must be where it was")
	}
	// the safety snapshot failing stops the restore before anything happens
	e.fs.failSnapshot = true
	e.m.FS = e.fs
	if _, err := e.m.Restore(context.Background(), s.ID, RestoreOptions{}); err == nil || !strings.Contains(err.Error(), "nothing was restored") {
		t.Errorf("err = %v", err)
	}
	if _, err := e.m.Restore(context.Background(), "nope", RestoreOptions{}); err == nil {
		t.Error("unknown snapshot")
	}
	stopErr := errors.New("won't stop")
	e.fs.failSnapshot = false
	if _, err := e.m.Restore(context.Background(), s.ID, RestoreOptions{Stop: func(context.Context) error { return stopErr }}); !errors.Is(err, stopErr) {
		t.Errorf("a failing stop must stop the restore: %v", err)
	}
}

// countingFS fails the failOn-th Snapshot call.
type countingFS struct {
	*fakeFS
	failOn int
	calls  *int
}

func (c *countingFS) Snapshot(src, dst string, ro bool) error {
	*c.calls++
	if *c.calls == c.failOn {
		return errors.New("no space left")
	}
	return c.fakeFS.Snapshot(src, dst, ro)
}

func TestRestorePath(t *testing.T) {
	e := newEnv(t)
	s := e.create(Manual)
	write(t, filepath.Join(e.m.InstanceDir, "mpmissions", "m", "storage_1", "data"), "v2")
	write(t, filepath.Join(e.m.InstanceDir, "mpmissions", "m", "storage_1", "newfile"), "n")
	write(t, filepath.Join(e.m.InstanceDir, "profiles", "log"), "keep me")
	if _, err := e.m.Restore(context.Background(), s.ID, RestoreOptions{Path: "mpmissions/m/storage_1"}); err != nil {
		t.Fatal(err)
	}
	if read(filepath.Join(e.m.InstanceDir, "mpmissions", "m", "storage_1", "data")) != "v1" {
		t.Error("the path must be restored")
	}
	if _, err := os.Stat(filepath.Join(e.m.InstanceDir, "mpmissions", "m", "storage_1", "newfile")); !os.IsNotExist(err) {
		t.Error("the path is replaced as a whole")
	}
	if read(filepath.Join(e.m.InstanceDir, "profiles", "log")) != "keep me" {
		t.Error("everything else stays")
	}
	if old, _ := filepath.Glob(filepath.Join(e.m.InstanceDir, "mpmissions", "m", "*.dzo-restore-old")); len(old) != 0 {
		t.Errorf("leftover %v", old)
	}
	for _, bad := range []string{"../other", "/etc", "..", ".", "a/../../b"} {
		if _, err := e.m.Restore(context.Background(), s.ID, RestoreOptions{Path: bad}); err == nil {
			t.Errorf("path %q must be refused", bad)
		}
	}
	if _, err := e.m.Restore(context.Background(), s.ID, RestoreOptions{Path: "not/in/backup"}); err == nil {
		t.Error("a path that is not in the snapshot must be refused before anything happens")
	}
	if ss, _ := e.m.List(); countReason(ss, PreRestore) != 1 {
		t.Errorf("the refused restores must not take safety snapshots: %+v", ss)
	}
}

func countReason(ss []Snapshot, r Reason) int {
	n := 0
	for _, s := range ss {
		if s.Reason == r {
			n++
		}
	}
	return n
}

func TestDiff(t *testing.T) {
	e := newEnv(t)
	a := e.create(Manual)
	write(t, filepath.Join(e.m.InstanceDir, "mpmissions", "m", "storage_1", "data"), "v2-longer")
	write(t, filepath.Join(e.m.InstanceDir, "new"), "n")
	if err := os.Remove(filepath.Join(e.m.InstanceDir, "profiles", "log")); err != nil {
		t.Fatal(err)
	}
	got, err := e.m.DiffWith(a.ID, "live")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"mpmissions/m/storage_1/data": "~", "new": "+", "profiles/log": "-"}
	if len(got) != 3 {
		t.Fatalf("diff = %v", got)
	}
	for _, c := range got {
		if want[c.Path] != c.Kind {
			t.Errorf("%+v", c)
		}
	}
	b := e.create(Manual)
	if same, _ := e.m.DiffWith(b.ID, ""); len(same) != 0 {
		t.Errorf("a fresh snapshot has no differences to live: %v", same)
	}
	if _, err := e.m.DiffWith("nope", ""); err == nil {
		t.Error("unknown snapshot")
	}
}

// On a real btrfs filesystem (the user's cache directory, or DZO_BTRFS_TESTDIR).
func TestOnRealBtrfs(t *testing.T) {
	base := os.Getenv("DZO_BTRFS_TESTDIR")
	if base == "" {
		c, err := os.UserCacheDir()
		if err != nil {
			t.Skip("no cache dir")
		}
		base = c
	}
	if ok, err := btrfs.IsBtrfs(base); err != nil || !ok {
		t.Skipf("%s is not on btrfs; set DZO_BTRFS_TESTDIR", base)
	}
	dir, err := os.MkdirTemp(base, "dzo-backup-test-")
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() {
		es, _ := os.ReadDir(filepath.Join(dir, "snapshots", "x"))
		for _, e := range es {
			_ = btrfs.Delete(filepath.Join(dir, "snapshots", "x", e.Name()))
		}
		es, _ = os.ReadDir(filepath.Join(dir, "instances"))
		for _, e := range es {
			_ = btrfs.Delete(filepath.Join(dir, "instances", e.Name()))
		}
		_ = os.RemoveAll(dir)
	})
	if err := os.MkdirAll(filepath.Join(dir, "instances"), 0o750); err != nil {
		t.Fatal(err)
	}
	inst := filepath.Join(dir, "instances", "x")
	if err := btrfs.CreateSubvolume(inst); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(inst, "mpmissions", "m", "storage_1", "data"), "v1")

	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	m := &Manager{Instance: "x", InstanceDir: inst, Root: filepath.Join(dir, "snapshots", "x"), FS: Btrfs{},
		Policy: Policy{Keep: 2, MinKeep: 1}, Now: func() time.Time { clock = clock.Add(time.Minute); return clock }}
	snaps := []Snapshot{}
	for i := 0; i < 3; i++ {
		res, err := m.Create(context.Background(), Manual)
		if err != nil {
			t.Fatal(err)
		}
		snaps = append(snaps, res.Snapshot)
	}
	if ss, _ := m.List(); len(ss) != 2 {
		t.Fatalf("retention on real snapshots: %+v", ss)
	}
	if ro, err := btrfs.ReadOnly(filepath.Join(m.Root, snaps[2].ID)); err != nil || !ro {
		t.Errorf("a backup must be read-only: %v %v", ro, err)
	}

	write(t, filepath.Join(inst, "mpmissions", "m", "storage_1", "data"), "v2")
	if _, err := m.Restore(context.Background(), snaps[2].ID, RestoreOptions{Path: "mpmissions/m/storage_1"}); err != nil {
		t.Fatal(err)
	}
	if read(filepath.Join(inst, "mpmissions", "m", "storage_1", "data")) != "v1" {
		t.Error("partial restore from a real snapshot")
	}
	write(t, filepath.Join(inst, "marker"), "live")
	if _, err := m.Restore(context.Background(), snaps[2].ID, RestoreOptions{}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := btrfs.IsSubvolume(inst); !ok {
		t.Error("the restored instance must be a subvolume")
	}
	if _, err := os.Stat(filepath.Join(inst, "marker")); !os.IsNotExist(err) {
		t.Error("full restore brings back the backup, not the live data")
	}
	if err := os.WriteFile(filepath.Join(inst, "writable"), []byte("x"), 0o600); err != nil {
		t.Errorf("the restored instance must be writable: %v", err)
	}
	var replaced Snapshot
	ss, _ := m.List()
	for _, s := range ss {
		if s.Reason == Replaced {
			replaced = s
		}
	}
	if replaced.ID == "" || read(filepath.Join(m.Root, replaced.ID, "marker")) != "live" {
		t.Errorf("the replaced data must be kept as a snapshot: %+v", ss)
	}
	if ro, _ := btrfs.ReadOnly(filepath.Join(m.Root, replaced.ID)); !ro {
		t.Error("the replaced snapshot is read-only")
	}
}

func TestRestoreSourceSurvivesTheRetention(t *testing.T) {
	e := newEnv(t)
	e.m.Policy = Policy{Keep: 2, MinKeep: 1}
	oldest := e.create(Manual)
	e.create(Manual)
	e.create(Manual) // retention has already removed the oldest
	ss, _ := e.m.List()
	if len(ss) != 2 || ss[0].ID == oldest.ID {
		t.Fatalf("setup: %+v", ss)
	}
	src := ss[0] // the older of the two: the first the retention would delete next
	if _, err := e.m.Restore(context.Background(), src.ID, RestoreOptions{}); err != nil {
		t.Fatalf("restoring the snapshot the retention would delete next must work: %v", err)
	}
	// the pin is lifted afterwards
	if got, err := e.m.Get(src.ID); err == nil && got.Pinned {
		t.Error("the restore must not leave the snapshot pinned")
	}
}
