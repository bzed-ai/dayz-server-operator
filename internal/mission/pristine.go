// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package mission

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FetchPristine fetches ref of the mission git repo and replaces dest (the
// instance's servermpmissions/<map>) with the directory at path inside it.
// path may be a glob but must match exactly one directory. The tree is built
// next to dest and swapped in with a rename, so a failed fetch leaves the old
// pristine mission alone (§C6: pristine is replaceable any time).
func FetchPristine(ctx context.Context, gitURL, ref, path, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("mission: %w", err)
	}
	work, err := os.MkdirTemp(filepath.Dir(dest), ".fetch-*")
	if err != nil {
		return fmt.Errorf("mission: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // best-effort cleanup of the scratch checkout

	repo := filepath.Join(work, "repo")
	for _, args := range [][]string{
		{"init", "-q", repo},
		{"-C", repo, "fetch", "-q", "--depth", "1", gitURL, ref},
		{"-C", repo, "checkout", "-q", "FETCH_HEAD"},
	} {
		if err := runGit(ctx, args...); err != nil {
			return err
		}
	}

	matches, err := filepath.Glob(filepath.Join(repo, path))
	if err != nil {
		return fmt.Errorf("mission: mission_source.path %q: %w", path, err)
	}
	if len(matches) != 1 {
		return fmt.Errorf("mission: mission_source.path %q matches %d entries in %s@%s, want exactly 1", path, len(matches), gitURL, ref)
	}
	if fi, err := os.Stat(matches[0]); err != nil || !fi.IsDir() {
		return fmt.Errorf("mission: mission_source.path %q is not a directory", path)
	}

	staged := filepath.Join(work, "pristine")
	if err := os.Mkdir(staged, 0o750); err != nil {
		return fmt.Errorf("mission: %w", err)
	}
	if err := CopyPristine(matches[0], staged); err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("mission: replace pristine: %w", err)
	}
	if err := os.Rename(staged, dest); err != nil {
		return fmt.Errorf("mission: install pristine: %w", err)
	}
	return nil
}

func runGit(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed subcommands; url/ref come from the operator's site repo
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mission: git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return nil
}
