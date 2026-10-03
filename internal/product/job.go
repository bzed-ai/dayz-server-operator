// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package product

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bzed/dayz-server-operator/internal/steam"
)

// AppUpdateOptions configures a `+app_update` job.
type AppUpdateOptions struct {
	Command      string // steamcmd binary; defaults to "steamcmd"
	Account      string
	AppID        uint32
	InstallDir   string
	BetaBranch   string // optional
	BetaPassword string // optional, only meaningful with BetaBranch
	Validate     bool
}

// WorkshopDownloadOptions configures a `+workshop_download_item` job.
type WorkshopDownloadOptions struct {
	Command       string // steamcmd binary; defaults to "steamcmd"
	Account       string
	WorkshopAppID uint32
	ItemIDs       []uint64
	InstallDir    string // +force_install_dir: items land in <dir>/steamapps/workshop/content/<app>/<id>
	Validate      bool
}

// JobResult is the outcome of one non-interactive steamcmd job.
type JobResult struct {
	Success bool
	// AuthRequired is true when the job could not even log in with the
	// cached session (steam.ErrAuthRequired): the caller should set the
	// global steam_auth_required state (§C7) rather than treat this as an
	// ordinary job failure.
	AuthRequired bool
	// FailedItems maps a workshop item id to steamcmd's own failure text,
	// for WorkshopDownload jobs that downloaded some items but not others.
	FailedItems map[uint64]string
	Log         string
}

const jobKillGraceDelay = 5 * time.Second

// ensureLoggedIn runs a bare `+login <account> +quit` through
// steam.Run with a NonInteractivePrompter, so a job that follows can
// assume the cached session is fresh: a cached session lets this succeed
// silently, and a stale one surfaces as steam.ErrAuthRequired here rather
// than in the middle of a download. A stale session can fail two
// different ways: steamcmd may prompt (NonInteractivePrompter declines,
// Run returns a non-nil error), or it may reject the cached credentials
// outright with no prompt at all (Run returns a classified failure
// Result with a nil error) - both must map to ErrAuthRequired here, or
// the caller would treat the second case as a successful login.
func ensureLoggedIn(ctx context.Context, command, account string) error {
	result, err := steam.Run(ctx, steam.Options{Command: command, Account: account, Prompter: steam.NonInteractivePrompter{}})
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("%w: %s", steam.ErrAuthRequired, result.Reason)
	}
	return nil
}

// RunAppUpdate downloads/updates a product build. The login phase is
// verified first (see ensureLoggedIn); the update itself then runs as a
// second, ordinary (non-pty) steamcmd invocation, since no further
// interactivity is expected once login succeeded.
func RunAppUpdate(ctx context.Context, opts AppUpdateOptions) (JobResult, error) {
	if opts.Account == "" || opts.AppID == 0 || opts.InstallDir == "" {
		return JobResult{}, fmt.Errorf("product: Account, AppID and InstallDir are required")
	}
	command := opts.Command
	if command == "" {
		command = "steamcmd"
	}
	if err := ensureLoggedIn(ctx, command, opts.Account); err != nil {
		if errors.Is(err, steam.ErrAuthRequired) {
			return JobResult{AuthRequired: true}, nil
		}
		return JobResult{}, err
	}

	args := []string{
		"+force_install_dir", opts.InstallDir,
		"+login", opts.Account,
		"+app_update", strconv.FormatUint(uint64(opts.AppID), 10),
	}
	if opts.BetaBranch != "" {
		args = append(args, "-beta", opts.BetaBranch)
		if opts.BetaPassword != "" {
			args = append(args, "-betapassword", opts.BetaPassword)
		}
	}
	if opts.Validate {
		args = append(args, "validate")
	}
	args = append(args, "+quit")

	log, err := runNonInteractive(ctx, command, args)
	result := JobResult{Log: log}
	if err != nil {
		return result, err
	}
	if containsFold(log, "success! app ") && containsFold(log, "fully installed") {
		result.Success = true
		return result, nil
	}
	if containsFold(log, appAuthFailureMarker) {
		result.AuthRequired = true
		return result, nil
	}
	return result, nil
}

// RunWorkshopDownload downloads/updates one or more workshop items,
// batched into a single steamcmd invocation (one +workshop_download_item
// per item, matching the legacy tool's approach of one steamcmd call for
// every mod update - §C7).
func RunWorkshopDownload(ctx context.Context, opts WorkshopDownloadOptions) (JobResult, error) {
	if opts.Account == "" || opts.WorkshopAppID == 0 || len(opts.ItemIDs) == 0 {
		return JobResult{}, fmt.Errorf("product: Account, WorkshopAppID and at least one ItemID are required")
	}
	command := opts.Command
	if command == "" {
		command = "steamcmd"
	}
	if err := ensureLoggedIn(ctx, command, opts.Account); err != nil {
		if errors.Is(err, steam.ErrAuthRequired) {
			return JobResult{AuthRequired: true}, nil
		}
		return JobResult{}, err
	}

	var args []string
	if opts.InstallDir != "" {
		args = append(args, "+force_install_dir", opts.InstallDir)
	}
	args = append(args, "+login", opts.Account)
	for _, id := range opts.ItemIDs {
		args = append(args, "+workshop_download_item", strconv.FormatUint(uint64(opts.WorkshopAppID), 10), strconv.FormatUint(id, 10))
		if opts.Validate {
			args = append(args, "validate")
		}
	}
	args = append(args, "+quit")

	log, err := runNonInteractive(ctx, command, args)
	result := JobResult{Log: log}
	if err != nil {
		return result, err
	}
	if containsFold(log, appAuthFailureMarker) {
		result.AuthRequired = true
		return result, nil
	}

	succeeded := map[uint64]bool{}
	failed := map[uint64]string{}
	scanWorkshopResults(log, succeeded, failed)
	for _, id := range opts.ItemIDs {
		if !succeeded[id] {
			if _, ok := failed[id]; !ok {
				failed[id] = "no result line for this item in steamcmd's output"
			}
		}
	}
	result.FailedItems = failed
	result.Success = len(failed) == 0
	return result, nil
}

// appAuthFailureMarker is steamcmd's login-failure wording, checked again
// here even though ensureLoggedIn already verified the session: a session
// can still expire in the (short) window between the two separate steamcmd
// invocations.
const appAuthFailureMarker = "failed (invalid password)"

// scanWorkshopResults fills succeeded/failed from steamcmd's per-item
// download report. The legacy implementation's own success detection is
// quoted directly in the plan (§C7): "grepping `Success. Downloaded item
// <id>`"; the failure form mirrors it ("ERROR! Download item <id> failed").
func scanWorkshopResults(log string, succeeded map[uint64]bool, failed map[uint64]string) {
	scanner := bufio.NewScanner(strings.NewReader(log))
	for scanner.Scan() {
		line := scanner.Text()
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "success. downloaded item"):
			if id, ok := extractItemID(lower, "downloaded item"); ok {
				succeeded[id] = true
			}
		case strings.Contains(lower, "download item") && strings.Contains(lower, "failed"):
			if id, ok := extractItemID(lower, "download item"); ok {
				failed[id] = strings.TrimSpace(line)
			}
		}
	}
}

// extractItemID pulls the numeric id that follows marker in line (both
// already lower-cased).
func extractItemID(line, marker string) (uint64, bool) {
	idx := strings.Index(line, marker)
	if idx < 0 {
		return 0, false
	}
	rest := strings.TrimSpace(line[idx+len(marker):])
	end := strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' })
	if end == 0 {
		return 0, false
	}
	if end < 0 {
		end = len(rest)
	}
	id, err := strconv.ParseUint(rest[:end], 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// runNonInteractive runs steamcmd expecting no interactivity at all (the
// login phase was already verified by ensureLoggedIn): a plain
// exec.CommandContext with captured combined output, no pty required.
func runNonInteractive(ctx context.Context, command string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...) //nolint:gosec // command/args are operator-configured (product/instance def), not user input
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = jobKillGraceDelay

	out, err := cmd.CombinedOutput()
	if err != nil {
		// steamcmd's exit code is not a reliable success/failure signal
		// (the legacy tool grepped its output instead, per §C7); only a
		// failure to even run the process - not an ExitError - is a hard
		// error here, so callers still get the log to classify either way.
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return string(out), fmt.Errorf("product: run %s: %w", command, err)
		}
	}
	return string(out), nil
}
