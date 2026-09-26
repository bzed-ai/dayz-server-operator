// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package steam

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// Prompter supplies interactive input in response to steamcmd's prompts.
// Password and Code must respect ctx (Run's overall timeout bounds them
// too); Waiting is informational only (no response expected) and is called
// at most once per session, on the transition into "waiting for app
// confirmation".
type Prompter interface {
	Password(ctx context.Context, account string) (string, error)
	Code(ctx context.Context, kind PromptKind) (string, error)
	Waiting(ctx context.Context)
}

// Result is the outcome of one login attempt.
type Result struct {
	Success bool
	Reason  FailureReason // set only when !Success
	Log     string        // scrubbed transcript (passwords/codes redacted), safe to store or print
}

// Options configures one steamcmd login session.
type Options struct {
	// Command is the steamcmd binary/wrapper to run. Defaults to
	// "steamcmd"; overridden in tests to point at a fake script.
	Command string
	// Args is appended after "+login <Account>" and before a final
	// "+quit" that Run always adds. internal/product composes further
	// steamcmd commands (app_update, workshop_download_item, ...) here
	// once it exists.
	Args    []string
	Account string
	// Prompter answers steamcmd's interactive prompts. Required.
	Prompter Prompter
	// PollInterval bounds how often Run checks for new pty output;
	// defaults to 50ms. Tests lower it to keep the fake-steamcmd suite fast.
	PollInterval time.Duration
	// Env, if non-nil, replaces the spawned process's environment.
	Env []string
}

const (
	defaultPollInterval = 50 * time.Millisecond
)

// killGraceDelay bounds how long Run waits for steamcmd to exit on its own
// (after "+quit", or after a ctx cancellation kills its process group)
// before forcing it. It is a var, not a const, so tests can shorten it
// instead of taking the full delay on the "steamcmd doesn't exit" path.
var killGraceDelay = 5 * time.Second

// Run drives one interactive steamcmd login attempt end to end: spawns
// steamcmd attached to a pty, classifies its output as it arrives, and
// answers password/Steam-Guard prompts via opts.Prompter until steamcmd
// reports success, a classified failure, or exits/times out without
// either (returned as FailureUnknown). Cancelling ctx (or ctx's deadline
// expiring) kills the whole process group, since steamcmd forks helper
// processes that would otherwise keep the pty open after the direct child
// is gone - the same failure mode internal/hooks' Runner works around, for
// the same reason.
func Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Account == "" {
		return Result{}, fmt.Errorf("steam: Account is required")
	}
	if opts.Prompter == nil {
		return Result{}, fmt.Errorf("steam: Prompter is required")
	}
	command := opts.Command
	if command == "" {
		command = "steamcmd"
	}
	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	args := append([]string{"+login", opts.Account}, opts.Args...)
	args = append(args, "+quit")

	cmd := exec.CommandContext(ctx, command, args...) //nolint:gosec // command/args are operator-configured (config.yaml/product def), not user input
	if opts.Env != nil {
		cmd.Env = opts.Env
	}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = killGraceDelay

	// pty.Start sets cmd.SysProcAttr.Setsid (needed to attach the pty as
	// the child's controlling terminal), which also makes the child its
	// own new process group leader - the property Cancel above and
	// waitBounded below rely on to kill steamcmd's whole process tree, not
	// just the direct child (see internal/hooks for why that distinction
	// matters). No separate Setpgid is needed or wanted alongside Setsid.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return Result{}, fmt.Errorf("steam: start %s: %w", command, err)
	}
	defer func() { _ = ptmx.Close() }()

	d := &driver{ptmx: ptmx, prompter: opts.Prompter, account: opts.Account, poll: pollInterval}
	result, runErr := d.loop(ctx)

	waitErr := waitBounded(cmd)
	if runErr != nil {
		if waitErr != nil {
			return result, fmt.Errorf("%w (steamcmd wait: %v)", runErr, waitErr) //nolint:errorlint // combining two independent causes into one message
		}
		return result, runErr
	}
	return result, nil
}

// waitBounded waits for cmd to exit, force-killing its process group if it
// doesn't within killGraceDelay of the caller already having decided the
// session is over (Run always calls this after the driver loop has
// returned a terminal Result, so a steamcmd that lingers after "+quit"
// must not block Run forever).
func waitBounded(cmd *exec.Cmd) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(killGraceDelay):
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return <-done
	}
}

// driver holds one Run invocation's mutable state.
type driver struct {
	ptmx     *os.File
	prompter Prompter
	account  string
	poll     time.Duration

	buf             strings.Builder // accumulated output not yet matched against a marker
	log             strings.Builder // full transcript, scrubbed lazily in result()
	secrets         []string
	waitingNotified bool
}

func (d *driver) loop(ctx context.Context) (Result, error) {
	chunk := make([]byte, 4096)
	for {
		if err := ctx.Err(); err != nil {
			return d.result(false, FailureUnknown), err
		}

		_ = d.ptmx.SetReadDeadline(time.Now().Add(d.poll))
		n, readErr := d.ptmx.Read(chunk)
		if n > 0 {
			d.buf.Write(chunk[:n])
			d.log.Write(chunk[:n])

			event, kind, reason := classify(d.buf.String())
			switch event {
			case EventPrompt:
				if err := d.respond(ctx, kind); err != nil {
					return d.result(false, FailureUnknown), err
				}
				d.buf.Reset()
			case EventAppConfirmWaiting:
				if !d.waitingNotified {
					d.prompter.Waiting(ctx)
					d.waitingNotified = true
				}
				d.buf.Reset()
			case EventSuccess:
				return d.result(true, ""), nil
			case EventFailure:
				return d.result(false, reason), nil
			case EventNone:
				// Not enough output yet to recognise anything; keep reading.
			}
		}
		if readErr != nil {
			if errors.Is(readErr, os.ErrDeadlineExceeded) {
				continue
			}
			if isProcessExited(readErr) {
				// steamcmd exited without a message this table recognises
				// as success or failure - an unknown outcome, not a Run error.
				return d.result(false, FailureUnknown), nil
			}
			return d.result(false, FailureUnknown), fmt.Errorf("steam: read steamcmd output: %w", readErr)
		}
	}
}

func (d *driver) respond(ctx context.Context, kind PromptKind) error {
	var secret string
	var err error
	switch kind {
	case PromptPassword:
		secret, err = d.prompter.Password(ctx, d.account)
	case PromptGuardCode:
		secret, err = d.prompter.Code(ctx, kind)
	case PromptNone:
		return fmt.Errorf("steam: unhandled prompt kind %v", kind)
	}
	if err != nil {
		return fmt.Errorf("steam: get input for %v prompt: %w", kind, err)
	}
	if secret == "" {
		return fmt.Errorf("steam: empty input for %v prompt", kind)
	}
	d.secrets = append(d.secrets, secret)
	if _, err := d.ptmx.Write([]byte(secret + "\n")); err != nil {
		return fmt.Errorf("steam: write response to steamcmd: %w", err)
	}
	return nil
}

func (d *driver) result(success bool, reason FailureReason) Result {
	log := d.log.String()
	for _, secret := range d.secrets {
		log = strings.ReplaceAll(log, secret, "[REDACTED]")
	}
	return Result{Success: success, Reason: reason, Log: log}
}

// isProcessExited reports whether err from reading a pty master means the
// child has exited and closed its end: Linux pty masters report this as
// EIO, not io.EOF, once the slave side is gone (a well known quirk of the
// pty line discipline that every Go pty-reading loop has to account for).
func isProcessExited(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return errors.Is(pathErr.Err, syscall.EIO)
	}
	return false
}
