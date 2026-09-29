// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cache manages dzo's immutable download generations (§C2, §C7):
// product builds and workshop mods are downloaded once into
// "<root>/<generation-id>/" directories that are never modified again, with
// a "current" symlink selecting the active one and never-running servers
// touched by garbage collection. Immutable generations mean an update or a
// forced re-download never changes files under a running server (NFR-03).
package cache

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"syscall"
)

const currentLinkName = "current"
const lockFileName = ".dzo-lock"

// Store manages the generations under one root directory, e.g.
// cache/products/<product> or cache/workshop/<appid>/<modid>.
type Store struct {
	Root string
}

// New returns a Store rooted at root. It does not touch the filesystem;
// call EnsureRoot before first use.
func New(root string) *Store {
	return &Store{Root: root}
}

// EnsureRoot creates the store's root directory if missing.
func (s *Store) EnsureRoot() error {
	if err := os.MkdirAll(s.Root, 0o750); err != nil {
		return fmt.Errorf("cache: create root %s: %w", s.Root, err)
	}
	return nil
}

// withLock serialises mutating operations on this store (create, GC,
// switching "current") across processes via an flock on a lock file, so a
// forced refresh and the automatic update pipeline never race (D7).
func (s *Store) withLock(fn func() error) error {
	if err := s.EnsureRoot(); err != nil {
		return err
	}
	unlock, err := Flock(filepath.Join(s.Root, lockFileName))
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// Flock takes an exclusive advisory lock on path (created if missing) and
// returns the function that releases it. It blocks until the lock is free.
func Flock(path string) (unlock func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // path is a dzo-configured cache dir + a constant filename
	if err != nil {
		return nil, fmt.Errorf("cache: open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("cache: acquire lock: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

// isReservedName reports whether name is a store-internal entry rather than
// a generation (the current symlink or the lock file).
func isReservedName(name string) bool {
	return name == currentLinkName || name == lockFileName
}

// NewGeneration creates a new, empty generation directory for id. It
// refuses to overwrite an existing generation: once created, a generation
// is meant to be immutable (NFR-03).
func (s *Store) NewGeneration(id string) (string, error) {
	if id == "" || id == currentLinkName || id == lockFileName {
		return "", fmt.Errorf("cache: invalid generation id %q", id)
	}
	var path string
	err := s.withLock(func() error {
		path = filepath.Join(s.Root, id)
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("cache: generation %q already exists", id)
		}
		return os.MkdirAll(path, 0o750)
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// Generations lists existing generation ids, sorted by directory creation
// time (oldest first) - the order GC prunes against, since generation ids
// (Steam build ids, workshop time_updated values) are not guaranteed to
// sort lexicographically.
func (s *Store) Generations() ([]string, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cache: list %s: %w", s.Root, err)
	}

	type gen struct {
		id      string
		modTime int64
	}
	var gens []gen
	for _, e := range entries {
		if isReservedName(e.Name()) || !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		gens = append(gens, gen{id: e.Name(), modTime: info.ModTime().UnixNano()})
	}
	sort.Slice(gens, func(i, j int) bool { return gens[i].modTime < gens[j].modTime })

	ids := make([]string, len(gens))
	for i, g := range gens {
		ids[i] = g.id
	}
	return ids, nil
}

// Current resolves the "current" symlink and returns the generation id it
// points to, or "" if none is set.
func (s *Store) Current() (string, error) {
	link := filepath.Join(s.Root, currentLinkName)
	target, err := os.Readlink(link)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("cache: read current link: %w", err)
	}
	return filepath.Base(target), nil
}

// SetCurrent atomically points "current" at generation id (create-and-
// rename, so readers never observe a missing or half-written symlink).
func (s *Store) SetCurrent(id string) error {
	return s.withLock(func() error {
		target := filepath.Join(s.Root, id)
		if _, err := os.Stat(target); err != nil {
			return fmt.Errorf("cache: generation %q does not exist: %w", id, err)
		}
		link := filepath.Join(s.Root, currentLinkName)
		tmp := link + ".tmp"
		_ = os.Remove(tmp) // best-effort cleanup of a stale temp link; SetCurrent below still catches a real problem
		if err := os.Symlink(id, tmp); err != nil {
			return fmt.Errorf("cache: create temp symlink: %w", err)
		}
		if err := os.Rename(tmp, link); err != nil {
			return fmt.Errorf("cache: activate current link: %w", err)
		}
		return nil
	})
}

// Remove deletes one generation directory. It refuses to remove the
// generation "current" currently points to.
func (s *Store) Remove(id string) error {
	return s.withLock(func() error { return s.removeLocked(id) })
}

func (s *Store) removeLocked(id string) error {
	if isReservedName(id) || id == "" {
		return fmt.Errorf("cache: refusing to remove reserved name %q", id)
	}
	current, err := s.currentLocked()
	if err != nil {
		return err
	}
	if id == current {
		return fmt.Errorf("cache: refusing to remove the current generation %q", id)
	}
	path := filepath.Join(s.Root, id)
	if filepath.Dir(path) != s.Root {
		return fmt.Errorf("cache: refusing to remove path outside the store root: %s", path)
	}
	return os.RemoveAll(path)
}

func (s *Store) currentLocked() (string, error) {
	link := filepath.Join(s.Root, currentLinkName)
	target, err := os.Readlink(link)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("cache: read current link: %w", err)
	}
	return filepath.Base(target), nil
}

// GC removes generations beyond the keep most recent (Generations' order),
// always preserving the current generation regardless of age. It returns
// the ids it removed.
func (s *Store) GC(keep int) ([]string, error) {
	if keep < 1 {
		return nil, fmt.Errorf("cache: keep must be >= 1, got %d", keep)
	}
	var removed []string
	err := s.withLock(func() error {
		entries, err := os.ReadDir(s.Root)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("cache: list %s: %w", s.Root, err)
		}
		type gen struct {
			id      string
			modTime int64
		}
		var gens []gen
		for _, e := range entries {
			if isReservedName(e.Name()) || !e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			gens = append(gens, gen{id: e.Name(), modTime: info.ModTime().UnixNano()})
		}
		sort.Slice(gens, func(i, j int) bool { return gens[i].modTime < gens[j].modTime })

		current, err := s.currentLocked()
		if err != nil {
			return err
		}

		keepCount := 0
		for i := len(gens) - 1; i >= 0; i-- {
			if keepCount < keep || gens[i].id == current {
				keepCount++
				continue
			}
			if err := s.removeLocked(gens[i].id); err != nil {
				return err
			}
			removed = append(removed, gens[i].id)
		}
		return nil
	})
	return removed, err
}

// CopyGeneration populates dst (an existing, empty generation directory)
// from src using a reflink copy where the filesystem supports it
// (`cp -a --reflink=auto`, near-instant on btrfs, §C2), falling back to a
// full copy everywhere else.
func CopyGeneration(src, dst string) error {
	cmd := exec.Command("cp", "-a", "--reflink=auto", src+"/.", dst) //nolint:gosec // src/dst are dzo-internal cache paths, never raw network/user input
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cache: copy %s -> %s: %w: %s", src, dst, err, out)
	}
	return nil
}
