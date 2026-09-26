// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package hooks runs the operator's extension points (§C11): post_backup,
// post_download(mod), post_merge(mod), post_render, pre_start, post_stop,
// pre_update, post_update. A hook is an executable with a DZO_* env
// contract and a JSON context on stdin; a non-zero exit can abort the
// operation that triggered it (the caller decides, via RunAll's
// stopOnError).
package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// killGraceDelay bounds how long Run waits for a killed hook's stdout/
// stderr pipes to close after a timeout, in case a background process it
// spawned inherited them and keeps them open. Without this, Cmd.Wait can
// block for as long as that descendant runs, defeating Timeout entirely.
const killGraceDelay = 5 * time.Second

// Context is the JSON payload sent to a hook on stdin, and the source of
// the DZO_INSTANCE/DZO_HOOK_POINT environment variables.
type Context struct {
	Instance string         `json:"instance"`
	Point    string         `json:"point"`
	Extra    map[string]any `json:"extra,omitempty"`
}

// Runner executes hook scripts with a shared environment (e.g. DZO_STAGING,
// DZO_LIVE_MISSION, DZO_MAP - set by the caller, since their meaning is
// specific to where in the render/update pipeline the hook point fires).
type Runner struct {
	Dir     string            // working directory for the hook process
	Env     map[string]string // additional fixed env vars merged over os.Environ()
	Timeout time.Duration     // 0 means no timeout
}

// Result is the outcome of running one hook script.
type Result struct {
	Script   string
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Err      error // non-nil on failure to start, timeout, or a non-zero exit
}

func (r *Runner) timeout() time.Duration { return r.Timeout }

// Run executes one hook script, sending hctx as JSON on stdin.
func (r *Runner) Run(ctx context.Context, script string, hctx Context) Result {
	start := time.Now()
	res := Result{Script: script}

	payload, err := json.Marshal(hctx)
	if err != nil {
		res.Err = fmt.Errorf("hooks: marshal context: %w", err)
		return res
	}

	if t := r.timeout(); t > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, script) //nolint:gosec // script comes from the site repo's configured hook list, not external input
	cmd.Dir = r.Dir
	cmd.Env = mergeEnv(hctx)
	for k, v := range r.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdin = bytes.NewReader(payload)

	// Run the hook in its own process group and kill the whole group on
	// timeout/cancellation, then give lingering pipe holders a bounded
	// grace period before Wait forcibly closes the pipes - otherwise a
	// background process the hook spawned (inheriting stdout/stderr) can
	// keep Wait blocked long past Timeout, defeating it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = killGraceDelay

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	res.Duration = time.Since(start)
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()
	res.ExitCode = cmd.ProcessState.ExitCode()

	if err != nil {
		res.Err = fmt.Errorf("hooks: %s: %w: %s", script, err, lastLine(res.Stderr))
	}
	return res
}

// RunAll runs scripts in order. If stopOnError is true, it stops at the
// first failure and returns the results so far (the caller aborts the
// triggering operation); otherwise every script runs regardless of
// earlier failures, and the caller inspects each Result.
func RunAll(ctx context.Context, r *Runner, scripts []string, hctx Context, stopOnError bool) []Result {
	results := make([]Result, 0, len(scripts))
	for _, script := range scripts {
		res := r.Run(ctx, script, hctx)
		results = append(results, res)
		if stopOnError && res.Err != nil {
			break
		}
	}
	return results
}

func mergeEnv(hctx Context) []string {
	env := os.Environ()
	env = append(env, "DZO_INSTANCE="+hctx.Instance, "DZO_HOOK_POINT="+hctx.Point)
	return env
}

func lastLine(s string) string {
	if s == "" {
		return ""
	}
	end := len(s)
	for end > 0 && (s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	start := end
	for start > 0 && s[start-1] != '\n' {
		start--
	}
	return s[start:end]
}
