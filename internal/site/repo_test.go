// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package site

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newTestRemote creates a bare git repo with one commit on branch main, and
// returns its filesystem path (usable as a "URL" for local git clones).
func newTestRemote(t *testing.T) string {
	t.Helper()
	remote := t.TempDir()
	runGit(t, remote, "init", "--bare", "--initial-branch=main", ".")

	seed := t.TempDir()
	runGit(t, seed, "init", "--initial-branch=main", ".")
	runGit(t, seed, "config", "user.email", "test@example.invalid")
	runGit(t, seed, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(seed, "site.yaml"), []byte("defaults: {}\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	runGit(t, seed, "add", "-A")
	runGit(t, seed, "commit", "-m", "initial")
	runGit(t, seed, "remote", "add", "origin", remote)
	runGit(t, seed, "push", "origin", "main")

	return remote
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestCloneAndPull(t *testing.T) {
	remote := newTestRemote(t)
	dir := filepath.Join(t.TempDir(), "checkout")

	r := &Repo{Dir: dir, RemoteURL: remote, Branch: "main"}
	if r.Cloned() {
		t.Fatal("Cloned() should be false before Clone")
	}
	if err := r.Clone(context.Background()); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if !r.Cloned() {
		t.Fatal("Cloned() should be true after Clone")
	}
	if _, err := os.Stat(filepath.Join(dir, "site.yaml")); err != nil {
		t.Fatalf("expected site.yaml to be checked out: %v", err)
	}

	dirty, err := r.IsDirty(context.Background())
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if dirty {
		t.Error("fresh clone should not be dirty")
	}

	if err := r.Pull(context.Background()); err != nil {
		t.Fatalf("Pull on an up-to-date clone: %v", err)
	}
}

func TestCloneRejectsEmptyRemoteURL(t *testing.T) {
	r := &Repo{Dir: t.TempDir(), Branch: "main"}
	if err := r.Clone(context.Background()); err == nil {
		t.Fatal("expected an error for an empty remote URL")
	}
}

func TestPullFetchesNewCommits(t *testing.T) {
	remote := newTestRemote(t)
	dir := filepath.Join(t.TempDir(), "checkout")
	r := &Repo{Dir: dir, RemoteURL: remote, Branch: "main"}
	if err := r.Clone(context.Background()); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Push a second commit from a different clone, simulating another
	// actor (an admin editing the site repo) updating the remote.
	other := filepath.Join(t.TempDir(), "other")
	runGit(t, filepath.Dir(other), "clone", remote, other)
	runGit(t, other, "config", "user.email", "test@example.invalid")
	runGit(t, other, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(other, "new-file.yaml"), []byte("x: 1\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	runGit(t, other, "add", "-A")
	runGit(t, other, "commit", "-m", "second commit")
	runGit(t, other, "push", "origin", "main")

	if err := r.Pull(context.Background()); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new-file.yaml")); err != nil {
		t.Fatalf("expected the new commit to be pulled: %v", err)
	}
}

func TestPullRefusesWhenDirty(t *testing.T) {
	remote := newTestRemote(t)
	dir := filepath.Join(t.TempDir(), "checkout")
	r := &Repo{Dir: dir, RemoteURL: remote, Branch: "main"}
	if err := r.Clone(context.Background()); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), []byte("defaults: {edited: true}\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := r.Pull(context.Background()); err != ErrDirty {
		t.Fatalf("Pull() = %v, want ErrDirty", err)
	}
}

func TestCommitAndPush(t *testing.T) {
	remote := newTestRemote(t)
	dir := filepath.Join(t.TempDir(), "checkout")
	r := &Repo{Dir: dir, RemoteURL: remote, Branch: "main", AutoPush: true}
	if err := r.Clone(context.Background()); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	runGit(t, dir, "config", "user.email", "dzo@example.invalid")
	runGit(t, dir, "config", "user.name", "dzo")

	if err := os.MkdirAll(filepath.Join(dir, "instances"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "instances/new.yaml"), []byte("name: new\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	committed, err := r.Commit(context.Background(), "dzo mod add", "dzo <dzo@example.invalid>")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if !committed {
		t.Fatal("expected Commit to report a new commit")
	}

	if err := r.Push(context.Background()); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Verify the remote actually received it, by cloning it again.
	verify := filepath.Join(t.TempDir(), "verify")
	r2 := &Repo{Dir: verify, RemoteURL: remote, Branch: "main"}
	if err := r2.Clone(context.Background()); err != nil {
		t.Fatalf("Clone (verify): %v", err)
	}
	if _, err := os.Stat(filepath.Join(verify, "instances/new.yaml")); err != nil {
		t.Fatalf("expected the pushed commit to be visible: %v", err)
	}
}

func TestCommitNoOpWhenClean(t *testing.T) {
	remote := newTestRemote(t)
	dir := filepath.Join(t.TempDir(), "checkout")
	r := &Repo{Dir: dir, RemoteURL: remote, Branch: "main"}
	if err := r.Clone(context.Background()); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	runGit(t, dir, "config", "user.email", "dzo@example.invalid")
	runGit(t, dir, "config", "user.name", "dzo")

	committed, err := r.Commit(context.Background(), "nothing changed", "")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if committed {
		t.Fatal("Commit should report no commit when nothing changed")
	}
}

func TestPushWithoutAutoPushIsNoOp(t *testing.T) {
	remote := newTestRemote(t)
	dir := filepath.Join(t.TempDir(), "checkout")
	r := &Repo{Dir: dir, RemoteURL: remote, Branch: "main", AutoPush: false}
	if err := r.Clone(context.Background()); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if err := r.Push(context.Background()); err != nil {
		t.Fatalf("Push (AutoPush=false) should be a no-op, got: %v", err)
	}
}

func TestHeadCommit(t *testing.T) {
	remote := newTestRemote(t)
	dir := filepath.Join(t.TempDir(), "checkout")
	r := &Repo{Dir: dir, RemoteURL: remote, Branch: "main"}
	if err := r.Clone(context.Background()); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	head, err := r.HeadCommit(context.Background())
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}
	if len(head) != 40 {
		t.Errorf("HeadCommit() = %q, want a 40-char SHA", head)
	}
}

func TestRunReportsGitErrors(t *testing.T) {
	r := &Repo{Dir: t.TempDir(), Branch: "main"}
	if _, err := r.run(context.Background(), r.Dir, "not-a-real-git-command"); err == nil {
		t.Fatal("expected an error for an invalid git subcommand")
	}
}

func TestDeployKeySetsGitSSHCommand(t *testing.T) {
	r := &Repo{DeployKeyPath: "/tmp/does-not-need-to-exist"}
	env := r.env()
	found := false
	for _, e := range env {
		if e == "GIT_SSH_COMMAND=ssh -i /tmp/does-not-need-to-exist -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new" {
			found = true
		}
	}
	if !found {
		t.Errorf("env() = %v, missing GIT_SSH_COMMAND", env)
	}
}
