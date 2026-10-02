// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package btrfs is the few btrfs operations dzo needs for per-instance
// subvolumes and their snapshots (§C20), done with the kernel's ioctls as the
// unprivileged service user: no root, no helper, no btrfs-progs.
package btrfs

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// SuperMagic is btrfs's statfs magic number.
const SuperMagic = 0x9123683E

const (
	ioctlSubvolCreate   = 0x5000940e // _IOW(0x94, 14, struct btrfs_ioctl_vol_args)
	ioctlSnapDestroy    = 0x5000940f // _IOW(0x94, 15, struct btrfs_ioctl_vol_args)
	ioctlSnapCreateV2   = 0x50009417 // _IOW(0x94, 23, struct btrfs_ioctl_vol_args_v2)
	ioctlSubvolGetflags = 0x80089419 // _IOR(0x94, 25, __u64)
	ioctlSubvolSetflags = 0x4008941a // _IOW(0x94, 26, __u64)

	subvolRdonly = 1 << 1
	nameMax      = 4087
	firstInode   = 256 // the root directory of every subvolume
)

// volArgs is struct btrfs_ioctl_vol_args.
type volArgs struct {
	fd   int64
	name [nameMax + 1]byte
}

// volArgsV2 is struct btrfs_ioctl_vol_args_v2 (the qgroup union is unused).
type volArgsV2 struct {
	fd      int64
	transid uint64
	flags   uint64
	unused  [4]uint64
	name    [4040]byte
}

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

func setName(dst []byte, name string) error {
	if name == "" || strings.ContainsRune(name, '/') || len(name) >= len(dst) {
		return fmt.Errorf("btrfs: bad subvolume name %q", name)
	}
	copy(dst, name)
	return nil
}

// IsBtrfs reports whether path is on a btrfs filesystem.
func IsBtrfs(path string) (bool, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return false, err
	}
	return st.Type == SuperMagic, nil
}

// IsSubvolume reports whether path is the root of a btrfs subvolume.
func IsSubvolume(path string) (bool, error) {
	ok, err := IsBtrfs(path)
	if err != nil || !ok {
		return false, err
	}
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return false, err
	}
	return st.Ino == firstInode && st.Mode&unix.S_IFMT == unix.S_IFDIR, nil
}

// SameFilesystem reports whether two paths are on the same filesystem; a
// snapshot cannot cross filesystems.
func SameFilesystem(a, b string) (bool, error) {
	var sa, sb unix.Stat_t
	if err := unix.Stat(a, &sa); err != nil {
		return false, err
	}
	if err := unix.Stat(b, &sb); err != nil {
		return false, err
	}
	return sa.Dev == sb.Dev, nil
}

func withDir(dir string, f func(fd int) error) error {
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(fd) }()
	return f(fd)
}

// CreateSubvolume creates a subvolume at path; its parent must exist.
func CreateSubvolume(path string) error {
	var a volArgs
	if err := setName(a.name[:], filepath.Base(path)); err != nil {
		return err
	}
	return withDir(filepath.Dir(path), func(fd int) error {
		if err := ioctl(fd, ioctlSubvolCreate, unsafe.Pointer(&a)); err != nil { //nolint:gosec // the ioctl argument struct
			return fmt.Errorf("btrfs: create subvolume %s: %w", path, err)
		}
		return nil
	})
}

// Snapshot makes a snapshot of the subvolume src at dst, read-only if asked.
// dst must not exist, and must be on the same filesystem.
func Snapshot(src, dst string, readonly bool) error {
	a := volArgsV2{}
	if readonly {
		a.flags = subvolRdonly
	}
	if err := setName(a.name[:], filepath.Base(dst)); err != nil {
		return err
	}
	return withDir(src, func(srcFd int) error {
		a.fd = int64(srcFd)
		return withDir(filepath.Dir(dst), func(dirFd int) error {
			if err := ioctl(dirFd, ioctlSnapCreateV2, unsafe.Pointer(&a)); err != nil { //nolint:gosec // the ioctl argument struct
				return fmt.Errorf("btrfs: snapshot %s to %s: %w", src, dst, err)
			}
			return nil
		})
	})
}

// ReadOnly reports the read-only flag of a subvolume.
func ReadOnly(path string) (bool, error) {
	var flags uint64
	err := withDir(path, func(fd int) error { return ioctl(fd, ioctlSubvolGetflags, unsafe.Pointer(&flags)) }) //nolint:gosec // the ioctl argument
	return flags&subvolRdonly != 0, err
}

// SetReadOnly sets or clears the read-only flag of a subvolume the caller owns.
func SetReadOnly(path string, ro bool) error {
	var flags uint64
	return withDir(path, func(fd int) error {
		if err := ioctl(fd, ioctlSubvolGetflags, unsafe.Pointer(&flags)); err != nil { //nolint:gosec // the ioctl argument struct
			return err
		}
		if ro {
			flags |= subvolRdonly
		} else {
			flags &^= subvolRdonly
		}
		return ioctl(fd, ioctlSubvolSetflags, unsafe.Pointer(&flags)) //nolint:gosec // the ioctl argument struct
	})
}

// Delete removes a subvolume or snapshot. It first asks the kernel to destroy
// it, which is instant but allowed to the owner only with the
// user_subvol_rm_allowed mount option, and refused (EROFS) for a read-only
// snapshot, so the read-only flag is cleared first, which the owner may do.
// Without the mount option the tree is removed file by file and the then empty
// subvolume with rmdir (also allowed for the owner): slower, proportional to the
// number of files, but it needs no privileges.
func Delete(path string) error {
	var a volArgs
	if err := setName(a.name[:], filepath.Base(path)); err != nil {
		return err
	}
	destroy := func() error {
		return withDir(filepath.Dir(path), func(fd int) error { return ioctl(fd, ioctlSnapDestroy, unsafe.Pointer(&a)) }) //nolint:gosec // the ioctl argument struct
	}
	err := destroy()
	if err == nil {
		return nil
	}
	ro, _ := ReadOnly(path)
	if ro && (errors.Is(err, unix.EROFS) || errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES)) {
		if serr := SetReadOnly(path, false); serr != nil {
			return fmt.Errorf("btrfs: delete %s: clear read-only: %w", path, serr)
		}
		if err = destroy(); err == nil {
			return nil
		}
	}
	if !errors.Is(err, unix.EPERM) && !errors.Is(err, unix.EACCES) {
		return fmt.Errorf("btrfs: delete %s: %w", path, err)
	}
	if err := os.RemoveAll(path); err != nil { //nolint:gosec // the caller (internal/backup) checked that path is a direct child of the snapshot root
		return fmt.Errorf("btrfs: delete %s: %w", path, err)
	}
	return nil
}

// MountOptions returns the mount options of the filesystem that holds path,
// from /proc/self/mountinfo (the longest matching mount point).
func MountOptions(path string) (string, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	best, opts := "", ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 6 {
			continue
		}
		mp := fields[4]
		if (real == mp || strings.HasPrefix(real, strings.TrimSuffix(mp, "/")+"/")) && len(mp) >= len(best) {
			best, opts = mp, fields[5]
			// the super options come after the " - fstype source" separator
			for i, f := range fields {
				if f == "-" && i+3 < len(fields) {
					opts = fields[5] + "," + fields[i+3]
				}
			}
		}
	}
	return opts, sc.Err()
}

// UserSubvolRmAllowed reports whether path's filesystem is mounted with
// user_subvol_rm_allowed, which makes deleting snapshots instant for the owner.
func UserSubvolRmAllowed(path string) bool {
	opts, err := MountOptions(path)
	return err == nil && strings.Contains(","+opts+",", ",user_subvol_rm_allowed,")
}
