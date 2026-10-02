// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package backup is §C20: backups are read-only btrfs snapshots of an
// instance's subvolume, kept at <snapshots>/<instance>/<id>, recorded in an
// index, pruned by a retention policy and restorable. It only ever acts on
// snapshots in its index, and only on direct children of its snapshot root.
package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/bzed-ai/dayz-server-operator/internal/btrfs"
)

// FS is what the manager needs from the filesystem; Btrfs is the real one.
type FS interface {
	Snapshot(src, dst string, readonly bool) error
	SetReadOnly(path string, ro bool) error
	Delete(path string) error
	FreeBytes(path string) (uint64, error)
}

// Btrfs is the FS on a btrfs filesystem.
type Btrfs struct{}

// Snapshot implements FS.
func (Btrfs) Snapshot(src, dst string, ro bool) error { return btrfs.Snapshot(src, dst, ro) }

// SetReadOnly implements FS.
func (Btrfs) SetReadOnly(p string, ro bool) error { return btrfs.SetReadOnly(p, ro) }

// Delete implements FS.
func (Btrfs) Delete(p string) error { return btrfs.Delete(p) }

// FreeBytes implements FS.
func (Btrfs) FreeBytes(p string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(p, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil //nolint:gosec // the block size is positive
}

const (
	indexFile = "index.json"
	lockFile  = ".lock"
	idFormat  = "20060102-150405"
)

// Manager manages the snapshots of one instance.
type Manager struct {
	Instance    string
	InstanceDir string // the instance's subvolume
	Root        string // <paths.snapshots>/<instance>
	Policy      Policy
	FS          FS
	Now         func() time.Time
	// PostBackup, if set, is called with the path of every new snapshot (the
	// post_backup hook); its failure is a warning, never a failure of the backup.
	PostBackup func(ctx context.Context, path string) error
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}

type index struct {
	Snapshots []Snapshot `json:"snapshots"`
}

func (m *Manager) path(id string) string { return filepath.Join(m.Root, id) }

// safePath returns the directory of a snapshot id after checking that it is a
// direct child of the root and not a symlink.
func (m *Manager) safePath(id string) (string, error) {
	if id == "" || id != filepath.Base(id) || id == "." || id == ".." || id == indexFile || id == lockFile {
		return "", fmt.Errorf("backup: %q is not a snapshot id", id)
	}
	p := m.path(id)
	if fi, err := os.Lstat(p); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("backup: %s is a symlink", p)
	}
	return p, nil
}

func (m *Manager) load() ([]Snapshot, error) {
	b, err := os.ReadFile(filepath.Join(m.Root, indexFile)) //nolint:gosec // dzo's own snapshot root
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ix index
	if err := json.Unmarshal(b, &ix); err != nil {
		return nil, fmt.Errorf("backup: %s: %w", filepath.Join(m.Root, indexFile), err)
	}
	return ix.Snapshots, nil
}

// ReadIndex reads the index of a snapshot root without locking it or recovering
// anything: for monitoring, which must never write.
func ReadIndex(root string) ([]Snapshot, error) {
	return (&Manager{Root: root}).load()
}

func (m *Manager) save(ss []Snapshot) error {
	sort.SliceStable(ss, func(i, j int) bool { return ss[i].Created.Before(ss[j].Created) })
	b, err := json.MarshalIndent(index{Snapshots: ss}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(m.Root, ".index-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // gone after the rename
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(m.Root, indexFile))
}

// locked runs f with the snapshot root locked, so that a scheduled snapshot,
// the daily prune and a manual command never interleave.
func (m *Manager) locked(f func(ss *[]Snapshot) error) error {
	if err := os.MkdirAll(m.Root, 0o750); err != nil {
		return err
	}
	lf, err := os.OpenFile(filepath.Join(m.Root, lockFile), os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // our root
	if err != nil {
		return err
	}
	defer func() { _ = lf.Close() }()
	if err := unix.Flock(int(lf.Fd()), unix.LOCK_EX); err != nil { //nolint:gosec // a file descriptor
		return err
	}
	ss, err := m.load()
	if err != nil {
		return err
	}
	if err := m.recover(&ss); err != nil {
		return err
	}
	return f(&ss)
}

// recover finishes what an interrupted run left: a snapshot that was being
// created is removed, one that was being deleted is deleted, and one whose
// directory is gone is marked missing.
func (m *Manager) recover(ss *[]Snapshot) error {
	var out []Snapshot
	var errs []string
	for _, s := range *ss {
		p, err := m.safePath(s.ID)
		if err != nil {
			errs = append(errs, err.Error())
			out = append(out, s)
			continue
		}
		_, statErr := os.Lstat(p)
		exists := statErr == nil
		switch s.State {
		case Creating, Deleting:
			if exists {
				if err := m.FS.Delete(p); err != nil {
					errs = append(errs, err.Error())
					out = append(out, s)
					continue
				}
			}
			// the entry is dropped: a half-made snapshot is worthless, a half-deleted one is gone
		case Complete:
			if !exists {
				s.State = Missing
			}
			out = append(out, s)
		case Missing:
			if exists {
				s.State = Complete
			}
			out = append(out, s)
		default:
			out = append(out, s)
		}
	}
	*ss = out
	if err := m.save(*ss); err != nil {
		return err
	}
	if len(errs) > 0 {
		return fmt.Errorf("backup: recovery: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (m *Manager) newID(reason Reason, ss []Snapshot) string {
	base := m.now().Format(idFormat) + "-" + string(reason)
	id := base
	for n := 2; ; n++ {
		taken := false
		for _, s := range ss {
			taken = taken || s.ID == id
		}
		if _, err := os.Lstat(m.path(id)); err == nil {
			taken = true
		}
		if !taken {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}

// Result is what Create did besides making the snapshot: the snapshots the
// retention removed afterwards, and problems that did not fail the backup.
type Result struct {
	Snapshot Snapshot
	Pruned   []Snapshot
	Warnings []string
}

// Create takes a read-only snapshot of the instance for a reason. Afterwards
// (and only then, so that a failure never reduces the number of good
// snapshots) the retention runs and the post_backup hook is called; their
// failures are warnings.
func (m *Manager) Create(ctx context.Context, reason Reason) (Result, error) {
	return m.create(ctx, reason, true)
}

// create is Create; prune says whether the retention runs afterwards.
func (m *Manager) create(ctx context.Context, reason Reason, prune bool) (Result, error) {
	if !ValidReason(reason) {
		return Result{}, fmt.Errorf("backup: unknown reason %q", reason)
	}
	var res Result
	err := m.locked(func(ss *[]Snapshot) error {
		if m.Policy.MinFreeBytes > 0 {
			if w := m.freeUp(ss); w != "" {
				res.Warnings = append(res.Warnings, w)
			}
		}
		s := Snapshot{ID: m.newID(reason, *ss), Reason: reason, State: Creating, Created: m.now()}
		*ss = append(*ss, s)
		if err := m.save(*ss); err != nil {
			return err
		}
		if err := m.FS.Snapshot(m.InstanceDir, m.path(s.ID), true); err != nil {
			*ss = (*ss)[:len(*ss)-1]
			_ = m.save(*ss)
			return err
		}
		(*ss)[len(*ss)-1].State = Complete
		res.Snapshot = (*ss)[len(*ss)-1]
		if err := m.save(*ss); err != nil {
			return err
		}
		if !prune {
			return nil
		}
		pruned, errs := m.prune(ss, false)
		res.Pruned = pruned
		for _, e := range errs {
			res.Warnings = append(res.Warnings, "retention: "+e.Error())
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	if m.PostBackup != nil {
		if err := m.PostBackup(ctx, m.path(res.Snapshot.ID)); err != nil {
			res.Warnings = append(res.Warnings, "post_backup hook: "+err.Error())
		}
	}
	return res, nil
}

// freeUp deletes the oldest snapshots until MinFreeBytes are free, as far as
// the protections allow. It returns a warning if there is still not enough.
func (m *Manager) freeUp(ss *[]Snapshot) string {
	for {
		free, err := m.FS.FreeBytes(m.Root)
		if err != nil {
			return "free space: " + err.Error()
		}
		if free >= uint64(m.Policy.MinFreeBytes) { //nolint:gosec // a non-negative size
			return ""
		}
		cands := Oldest(*ss, max(m.Policy.MinKeep, 1))
		if len(cands) == 0 {
			return fmt.Sprintf("only %d MiB free (min_free_bytes is %d MiB) and nothing may be pruned", free>>20, m.Policy.MinFreeBytes>>20)
		}
		if err := m.deleteSnapshot(ss, cands[0]); err != nil {
			return "free space: " + err.Error()
		}
	}
}

// Prune applies the retention policy. With dryRun it only returns what it
// would delete.
func (m *Manager) Prune(dryRun bool) ([]Snapshot, error) {
	var out []Snapshot
	err := m.locked(func(ss *[]Snapshot) error {
		deleted, errs := m.prune(ss, dryRun)
		out = deleted
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
		return nil
	})
	return out, err
}

func (m *Manager) prune(ss *[]Snapshot, dryRun bool) ([]Snapshot, []error) {
	del := Select(*ss, m.Policy, m.now())
	if dryRun {
		return del, nil
	}
	var done []Snapshot
	var errs []error
	for _, s := range del {
		if err := m.deleteSnapshot(ss, s); err != nil {
			errs = append(errs, err)
			continue
		}
		done = append(done, s)
	}
	return done, errs
}

// deleteSnapshot removes a snapshot: marked deleting first, so that an
// interrupted deletion is finished by the next run, and dropped from the
// index only when the directory is gone.
func (m *Manager) deleteSnapshot(ss *[]Snapshot, s Snapshot) error {
	p, err := m.safePath(s.ID)
	if err != nil {
		return err
	}
	set := func(st State) {
		for i := range *ss {
			if (*ss)[i].ID == s.ID {
				(*ss)[i].State = st
			}
		}
	}
	set(Deleting)
	if err := m.save(*ss); err != nil {
		return err
	}
	if _, err := os.Lstat(p); err == nil {
		if err := m.FS.Delete(p); err != nil {
			return err
		}
	}
	var keep []Snapshot
	for _, x := range *ss {
		if x.ID != s.ID {
			keep = append(keep, x)
		}
	}
	*ss = keep
	return m.save(*ss)
}

// List returns the index, oldest first.
func (m *Manager) List() ([]Snapshot, error) {
	var out []Snapshot
	err := m.locked(func(ss *[]Snapshot) error { out = append([]Snapshot(nil), *ss...); return nil })
	return out, err
}

// Get returns one snapshot of the index.
func (m *Manager) Get(id string) (Snapshot, error) {
	ss, err := m.List()
	if err != nil {
		return Snapshot{}, err
	}
	for _, s := range ss {
		if s.ID == id {
			return s, nil
		}
	}
	return Snapshot{}, fmt.Errorf("backup: no snapshot %q of %s", id, m.Instance)
}

// Path returns the directory of a complete snapshot.
func (m *Manager) Path(id string) (string, error) {
	s, err := m.Get(id)
	if err != nil {
		return "", err
	}
	if s.State != Complete {
		return "", fmt.Errorf("backup: snapshot %s is %s", id, s.State)
	}
	return m.safePath(id)
}

// Pin protects a snapshot from automatic pruning, or lifts the protection.
func (m *Manager) Pin(id string, pinned bool) error {
	return m.locked(func(ss *[]Snapshot) error {
		for i := range *ss {
			if (*ss)[i].ID == id {
				(*ss)[i].Pinned = pinned
				return m.save(*ss)
			}
		}
		return fmt.Errorf("backup: no snapshot %q of %s", id, m.Instance)
	})
}

// Orphans lists directories in the snapshot root that the index does not
// know. They are reported, never deleted unless the operator asks.
func (m *Manager) Orphans() ([]string, error) {
	var out []string
	err := m.locked(func(ss *[]Snapshot) error {
		known := map[string]bool{indexFile: true, lockFile: true}
		for _, s := range *ss {
			known[s.ID] = true
		}
		es, err := os.ReadDir(m.Root)
		if err != nil {
			return err
		}
		for _, e := range es {
			if !known[e.Name()] && !strings.HasPrefix(e.Name(), ".index-") {
				out = append(out, e.Name())
			}
		}
		return nil
	})
	return out, err
}

// DeleteOrphans deletes the named orphans (each must be one of Orphans).
func (m *Manager) DeleteOrphans(names []string) error {
	orph, err := m.Orphans()
	if err != nil {
		return err
	}
	isOrphan := map[string]bool{}
	for _, o := range orph {
		isOrphan[o] = true
	}
	for _, n := range names {
		if !isOrphan[n] {
			return fmt.Errorf("backup: %q is not an orphan", n)
		}
		p, err := m.safePath(n)
		if err != nil {
			return err
		}
		if err := m.FS.Delete(p); err != nil {
			return err
		}
	}
	return nil
}

// AdoptOrphans enters the named orphans into the index as complete
// snapshots of reason "manual", dated from their name or else from now.
func (m *Manager) AdoptOrphans(names []string) error {
	orph, err := m.Orphans()
	if err != nil {
		return err
	}
	isOrphan := map[string]bool{}
	for _, o := range orph {
		isOrphan[o] = true
	}
	return m.locked(func(ss *[]Snapshot) error {
		for _, n := range names {
			if !isOrphan[n] {
				return fmt.Errorf("backup: %q is not an orphan", n)
			}
			created := m.now()
			if len(n) >= len(idFormat) {
				if t, err := time.Parse(idFormat, n[:len(idFormat)]); err == nil {
					created = t
				}
			}
			*ss = append(*ss, Snapshot{ID: n, Reason: Manual, State: Complete, Created: created})
		}
		return m.save(*ss)
	})
}
