// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package steam

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakePrompter drives Run's Prompter interface with scripted answers, and
// records everything it was asked, so tests can assert on the sequence of
// prompts the state machine actually issued.
type fakePrompter struct {
	password    string
	codes       []string // consumed in order, one per Code call
	codeCalls   int
	waitCalls   int
	passwordErr error
	codeErr     error
}

func (p *fakePrompter) Password(_ context.Context, _ string) (string, error) {
	if p.passwordErr != nil {
		return "", p.passwordErr
	}
	return p.password, nil
}

func (p *fakePrompter) Code(_ context.Context, _ PromptKind) (string, error) {
	if p.codeErr != nil {
		return "", p.codeErr
	}
	idx := p.codeCalls
	p.codeCalls++
	if idx < len(p.codes) {
		return p.codes[idx], nil
	}
	return "000000", nil
}

func (p *fakePrompter) Waiting(_ context.Context) {
	p.waitCalls++
}

// writeFakeSteamcmd writes a shell script that scripts one steamcmd
// session for the login state machine to drive, and returns its path.
// Real steamcmd's exact prompt/failure wording is not verified against a
// live account from this environment (see the package doc comment); this
// fixture reproduces the wording the classifier table recognises, so it
// tests the state machine's control flow (respond to a prompt, terminate
// on success/failure, survive an unrecognised exit) rather than steamcmd
// fidelity.
func writeFakeSteamcmd(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-steamcmd.sh")
	full := "#!/bin/sh\n" + script
	if err := os.WriteFile(path, []byte(full), 0o755); err != nil { //nolint:gosec // test fixture, fixed 0755 mode
		t.Fatalf("write fake steamcmd: %v", err)
	}
	return path
}

func runOpts(t *testing.T, command string, prompter Prompter) Options {
	t.Helper()
	return Options{
		Command:      command,
		Account:      "bob",
		Prompter:     prompter,
		PollInterval: 5 * time.Millisecond,
	}
}

func TestRunSuccessAfterPassword(t *testing.T) {
	script := writeFakeSteamcmd(t, `
echo "Logging in user 'bob' to Steam Public..."
printf 'password: '
read pass
echo ""
echo "Waiting for user info...OK"
`)
	prompter := &fakePrompter{password: "s3cr3t"}
	result, err := Run(context.Background(), runOpts(t, script, prompter))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Success {
		t.Fatalf("Success = false, reason = %v, log = %q", result.Reason, result.Log)
	}
	if strings.Contains(result.Log, "s3cr3t") {
		t.Errorf("Log leaked the password: %q", result.Log)
	}
}

func TestRunInvalidPassword(t *testing.T) {
	script := writeFakeSteamcmd(t, `
printf 'password: '
read pass
echo ""
echo "FAILED (Invalid Password)"
`)
	prompter := &fakePrompter{password: "wrong"}
	result, err := Run(context.Background(), runOpts(t, script, prompter))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Success || result.Reason != FailureInvalidPassword {
		t.Fatalf("Success=%v Reason=%v, want failure/invalid_password", result.Success, result.Reason)
	}
}

func TestRunGuardCodeThenSuccess(t *testing.T) {
	script := writeFakeSteamcmd(t, `
printf 'password: '
read pass
echo ""
printf 'Steam Guard code: '
read code
echo ""
echo "Waiting for user info...OK"
`)
	prompter := &fakePrompter{password: "s3cr3t", codes: []string{"123456"}}
	result, err := Run(context.Background(), runOpts(t, script, prompter))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Success {
		t.Fatalf("Success = false, reason = %v, log = %q", result.Reason, result.Log)
	}
	if prompter.codeCalls != 1 {
		t.Errorf("codeCalls = %d, want 1", prompter.codeCalls)
	}
}

func TestRunGuardCodeMismatch(t *testing.T) {
	script := writeFakeSteamcmd(t, `
printf 'password: '
read pass
echo ""
printf 'Steam Guard code: '
read code
echo ""
echo "FAILED (Two-factor code mismatch)"
`)
	prompter := &fakePrompter{password: "s3cr3t", codes: []string{"000000"}}
	result, err := Run(context.Background(), runOpts(t, script, prompter))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Success || result.Reason != FailureGuardCodeMismatch {
		t.Fatalf("Success=%v Reason=%v, want failure/guard_code_mismatch", result.Success, result.Reason)
	}
}

func TestRunAppConfirmWaitingThenSuccess(t *testing.T) {
	script := writeFakeSteamcmd(t, `
printf 'password: '
read pass
echo ""
echo "Please confirm this login in the Steam Mobile app..."
sleep 0.05
echo "Waiting for user info...OK"
`)
	prompter := &fakePrompter{password: "s3cr3t"}
	result, err := Run(context.Background(), runOpts(t, script, prompter))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Success {
		t.Fatalf("Success = false, reason = %v", result.Reason)
	}
	if prompter.waitCalls != 1 {
		t.Errorf("waitCalls = %d, want 1", prompter.waitCalls)
	}
}

func TestRunRateLimitedWithoutAnyPrompt(t *testing.T) {
	script := writeFakeSteamcmd(t, `
echo "FAILED (Rate Limit Exceeded)"
`)
	prompter := &fakePrompter{}
	result, err := Run(context.Background(), runOpts(t, script, prompter))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Success || result.Reason != FailureRateLimited {
		t.Fatalf("Success=%v Reason=%v, want failure/rate_limited", result.Success, result.Reason)
	}
}

func TestRunExitsWithoutRecognisedMessageIsUnknownFailure(t *testing.T) {
	script := writeFakeSteamcmd(t, `
echo "something steamcmd never actually prints"
exit 1
`)
	prompter := &fakePrompter{}
	result, err := Run(context.Background(), runOpts(t, script, prompter))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Success || result.Reason != FailureUnknown {
		t.Fatalf("Success=%v Reason=%v, want failure/unknown", result.Success, result.Reason)
	}
}

func TestRunPasswordPrompterErrorAborts(t *testing.T) {
	orig := killGraceDelay
	killGraceDelay = 100 * time.Millisecond
	defer func() { killGraceDelay = orig }()

	script := writeFakeSteamcmd(t, `
printf 'password: '
read pass
echo "Waiting for user info...OK"
`)
	prompter := &fakePrompter{passwordErr: errors.New("no password configured")}
	_, err := Run(context.Background(), runOpts(t, script, prompter))
	if err == nil {
		t.Fatal("expected an error when the prompter fails")
	}
}

func TestRunCodePrompterErrorAborts(t *testing.T) {
	orig := killGraceDelay
	killGraceDelay = 100 * time.Millisecond
	defer func() { killGraceDelay = orig }()

	script := writeFakeSteamcmd(t, `
printf 'password: '
read pass
echo ""
printf 'Steam Guard code: '
read code
echo "Waiting for user info...OK"
`)
	prompter := &fakePrompter{password: "s3cr3t", codeErr: errors.New("no code available")}
	_, err := Run(context.Background(), runOpts(t, script, prompter))
	if err == nil {
		t.Fatal("expected an error when the code prompter fails")
	}
}

func TestRunEmptyPasswordAborts(t *testing.T) {
	orig := killGraceDelay
	killGraceDelay = 100 * time.Millisecond
	defer func() { killGraceDelay = orig }()

	script := writeFakeSteamcmd(t, `
printf 'password: '
read pass
echo "Waiting for user info...OK"
`)
	prompter := &fakePrompter{password: ""}
	_, err := Run(context.Background(), runOpts(t, script, prompter))
	if err == nil {
		t.Fatal("expected an error when the prompter returns an empty password")
	}
}

func TestRunRequiresAccount(t *testing.T) {
	_, err := Run(context.Background(), Options{Prompter: &fakePrompter{}})
	if err == nil {
		t.Fatal("expected an error for a missing Account")
	}
}

func TestRunRequiresPrompter(t *testing.T) {
	_, err := Run(context.Background(), Options{Account: "bob"})
	if err == nil {
		t.Fatal("expected an error for a missing Prompter")
	}
}

func TestRunMissingCommandErrors(t *testing.T) {
	opts := runOpts(t, filepath.Join(t.TempDir(), "does-not-exist"), &fakePrompter{})
	_, err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("expected an error for a missing steamcmd binary")
	}
}

func TestRunContextCancelledDuringPrompt(t *testing.T) {
	script := writeFakeSteamcmd(t, `
printf 'password: '
read pass
echo "Waiting for user info...OK"
`)
	ctx, cancel := context.WithCancel(context.Background())
	// The prompter blocks until the test cancels ctx, so this exercises
	// Run tearing down a steamcmd process that is mid-prompt when the
	// caller gives up.
	prompter := &blockingPrompter{cancel: cancel}
	_, err := Run(ctx, runOpts(t, script, prompter))
	if err == nil {
		t.Fatal("expected a context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

type blockingPrompter struct {
	cancel context.CancelFunc
}

func (p *blockingPrompter) Password(ctx context.Context, _ string) (string, error) {
	p.cancel()
	<-ctx.Done()
	return "", ctx.Err()
}

func (p *blockingPrompter) Code(ctx context.Context, _ PromptKind) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func (p *blockingPrompter) Waiting(context.Context) {}

func TestRunKillsProcessThatHangsAfterSuccess(t *testing.T) {
	orig := killGraceDelay
	killGraceDelay = 100 * time.Millisecond
	defer func() { killGraceDelay = orig }()

	script := writeFakeSteamcmd(t, `
echo "Waiting for user info...OK"
sleep 30
`)
	prompter := &fakePrompter{}
	start := time.Now()
	result, err := Run(context.Background(), runOpts(t, script, prompter))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Success {
		t.Fatalf("Success = false, reason = %v", result.Reason)
	}
	if elapsed > 5*time.Second {
		t.Errorf("Run took %v, want well under the real 30s sleep (killGraceDelay should force it)", elapsed)
	}
}

func TestIsProcessExitedEOF(t *testing.T) {
	if !isProcessExited(io.EOF) {
		t.Error("expected io.EOF to be treated as process-exited")
	}
}

func TestIsProcessExitedEIO(t *testing.T) {
	err := &fs.PathError{Op: "read", Path: "/dev/ptmx", Err: syscall.EIO}
	if !isProcessExited(err) {
		t.Error("expected a wrapped syscall.EIO to be treated as process-exited")
	}
}

func TestIsProcessExitedOtherErrorIsNot(t *testing.T) {
	if isProcessExited(errors.New("some other read error")) {
		t.Error("expected an unrelated error not to be treated as process-exited")
	}
}
