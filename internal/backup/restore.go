// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package backup

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// RestoreOptions say how to restore. Stop and Start bring the instance down
// and up around it; both may be nil.
type RestoreOptions struct {
	// Path restores only this path below the instance (for example
	// mpmissions/<map>/storage_1), from the snapshot into the stopped
	// instance. Empty restores the whole instance.
	Path  string
	Stop  func(ctx context.Context) error
	Start func(ctx context.Context) error
}

// Restore puts a snapshot back. A full restore stops the instance, takes a
// pre_restore safety snapshot, moves the live subvolume aside, makes a
// writable snapshot of the backup at the instance path, keeps the replaced
// subvolume as a snapshot of reason "replaced" (subject to retention), and
// starts the instance. A partial restore replaces one path from the snapshot.
func (m *Manager) Restore(ctx context.Context, id string, o RestoreOptions) (Result, error) {
	snap, err := m.Get(id)
	if err != nil {
		return Result{}, err
	}
	if snap.State != Complete {
		return Result{}, fmt.Errorf("backup: snapshot %s is %s and cannot be restored", id, snap.State)
	}
	src, err := m.safePath(id)
	if err != nil {
		return Result{}, err
	}
	rel := ""
	if o.Path != "" {
		if rel, err = cleanRel(o.Path); err != nil {
			return Result{}, err
		}
		if _, err := os.Lstat(filepath.Join(src, rel)); err != nil {
			return Result{}, fmt.Errorf("backup: %s is not in snapshot %s: %w", rel, id, err)
		}
	}
	if o.Stop != nil {
		if err := o.Stop(ctx); err != nil {
			return Result{}, fmt.Errorf("backup: stop %s: %w", m.Instance, err)
		}
	}
	// The snapshot we restore from must not be pruned meanwhile, by the
	// retention of the safety snapshot or by a prune timer.
	if !snap.Pinned {
		if err := m.Pin(id, true); err != nil {
			return Result{}, err
		}
		defer func() { _ = m.Pin(id, false) }()
	}
	// A safety snapshot first: a restore is destructive, and restoring the
	// wrong snapshot must be undoable. The retention runs only afterwards.
	res, err := m.create(ctx, PreRestore, false)
	if err != nil {
		return res, fmt.Errorf("backup: the safety snapshot failed, nothing was restored: %w", err)
	}
	if rel != "" {
		err = m.restorePath(src, rel)
	} else {
		err = m.restoreFull(src, &res)
	}
	if err != nil {
		return res, err
	}
	if o.Start != nil {
		if err := o.Start(ctx); err != nil {
			return res, fmt.Errorf("backup: restored, but starting %s failed: %w", m.Instance, err)
		}
	}
	return res, nil
}

func cleanRel(p string) (string, error) {
	c := filepath.Clean(p)
	if filepath.IsAbs(c) || c == "." || c == ".." || strings.HasPrefix(c, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("backup: %q is not a path below the instance", p)
	}
	return c, nil
}

func (m *Manager) restoreFull(src string, res *Result) error {
	aside := filepath.Join(filepath.Dir(m.InstanceDir), "."+filepath.Base(m.InstanceDir)+".replaced-"+m.now().Format(idFormat))
	if err := os.Rename(m.InstanceDir, aside); err != nil {
		return fmt.Errorf("backup: move %s aside: %w", m.InstanceDir, err)
	}
	if err := m.FS.Snapshot(src, m.InstanceDir, false); err != nil {
		if rerr := os.Rename(aside, m.InstanceDir); rerr != nil {
			return fmt.Errorf("backup: restore failed (%v) and so did moving %s back (%w); the data is in %s", err, m.InstanceDir, rerr, aside)
		}
		return fmt.Errorf("backup: restore failed, the instance is as it was: %w", err)
	}
	// the replaced subvolume becomes a snapshot of its own, under retention
	err := m.locked(func(ss *[]Snapshot) error {
		id := m.newID(Replaced, *ss)
		if err := os.Rename(aside, m.path(id)); err != nil {
			return err
		}
		if err := m.FS.SetReadOnly(m.path(id), true); err != nil {
			return err
		}
		*ss = append(*ss, Snapshot{ID: id, Reason: Replaced, State: Complete, Created: m.now()})
		return m.save(*ss)
	})
	if err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("the replaced data stays in %s: %v", aside, err))
	}
	return nil
}

func (m *Manager) restorePath(snap, rel string) error {
	dst := filepath.Join(m.InstanceDir, rel)
	old := dst + ".dzo-restore-old"
	_ = os.RemoveAll(old)
	if _, err := os.Lstat(dst); err == nil {
		if err := os.Rename(dst, old); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	if err := copyTree(filepath.Join(snap, rel), dst); err != nil {
		_ = os.RemoveAll(dst)
		if _, serr := os.Lstat(old); serr == nil {
			_ = os.Rename(old, dst)
		}
		return fmt.Errorf("backup: restore %s: %w", rel, err)
	}
	return os.RemoveAll(old)
}

// copyTree copies src to dst preserving modes, times and symlinks. Files are
// cloned (reflink) where the filesystem can, which is instant and free on
// btrfs, and copied otherwise.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			if err := os.MkdirAll(target, info.Mode().Perm()|0o700); err != nil {
				return err
			}
			return nil
		case info.Mode()&os.ModeSymlink != 0:
			l, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(l, target) //nolint:gosec // the walk is below a read-only snapshot and the target below a stopped instance
		case info.Mode().IsRegular():
			return copyFile(p, target, info) //nolint:gosec // the walk is below a snapshot that is read-only, and the target is below a stopped instance
		}
		return nil // sockets, devices: not part of an instance
	})
}

func copyFile(src, dst string, info os.FileInfo) error {
	in, err := os.Open(src) //nolint:gosec // inside a snapshot
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm()) //nolint:gosec // inside the instance
	if err != nil {
		return err
	}
	if err := unix.IoctlFileClone(int(out.Fd()), int(in.Fd())); err != nil { //nolint:gosec // file descriptors
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

// Change is one difference between two trees.
type Change struct {
	Kind string // "+" only in b, "-" only in a, "~" differs
	Path string
}

type entry struct {
	dir     bool
	size    int64
	mtimeNS int64
	mode    os.FileMode
	link    string
}

func scan(root string) (map[string]entry, error) {
	out := map[string]entry{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		e := entry{dir: d.IsDir(), mode: info.Mode()}
		if !e.dir {
			e.size, e.mtimeNS = info.Size(), info.ModTime().UnixNano()
		}
		if info.Mode()&os.ModeSymlink != 0 {
			e.link, _ = os.Readlink(p)
		}
		out[rel] = e
		return nil
	})
	return out, err
}

// Diff compares two trees by size, time and mode: a snapshot keeps the times
// of its files, so an unchanged file looks the same on both sides.
func Diff(a, b string) ([]Change, error) {
	ea, err := scan(a)
	if err != nil {
		return nil, err
	}
	eb, err := scan(b)
	if err != nil {
		return nil, err
	}
	var out []Change
	for p, x := range ea {
		y, ok := eb[p]
		switch {
		case !ok:
			out = append(out, Change{"-", p})
		case x != y:
			out = append(out, Change{"~", p})
		}
	}
	for p := range eb {
		if _, ok := ea[p]; !ok {
			out = append(out, Change{"+", p})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// DiffWith compares snapshot a with snapshot b, or with the live instance if
// b is "live" or empty.
func (m *Manager) DiffWith(a, b string) ([]Change, error) {
	pa, err := m.Path(a)
	if err != nil {
		return nil, err
	}
	pb := m.InstanceDir
	if b != "" && b != "live" {
		if pb, err = m.Path(b); err != nil {
			return nil, err
		}
	}
	return Diff(pa, pb)
}
