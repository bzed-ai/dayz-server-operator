// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package hooks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hook.sh")
	full := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(full), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func TestRunSuccessCapturesStdout(t *testing.T) {
	script := writeScript(t, `echo "hello from hook"`)
	r := &Runner{}
	res := r.Run(context.Background(), script, Context{Instance: "deerisle", Point: "pre_start"})
	if res.Err != nil {
		t.Fatalf("Run: %v", res.Err)
	}
	if !strings.Contains(res.Stdout, "hello from hook") {
		t.Errorf("Stdout = %q", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d", res.ExitCode)
	}
}

func TestRunPassesEnvContract(t *testing.T) {
	script := writeScript(t, `echo "$DZO_INSTANCE/$DZO_HOOK_POINT/$DZO_MAP"`)
	r := &Runner{Env: map[string]string{"DZO_MAP": "empty.deerisle"}}
	res := r.Run(context.Background(), script, Context{Instance: "deerisle", Point: "pre_start"})
	if res.Err != nil {
		t.Fatalf("Run: %v", res.Err)
	}
	if !strings.Contains(res.Stdout, "deerisle/pre_start/empty.deerisle") {
		t.Errorf("Stdout = %q", res.Stdout)
	}
}

func TestRunPassesJSONContextOnStdin(t *testing.T) {
	script := writeScript(t, `cat`)
	r := &Runner{}
	res := r.Run(context.Background(), script, Context{
		Instance: "hashima",
		Point:    "post_download",
		Extra:    map[string]any{"mod_id": float64(1559212036)},
	})
	if res.Err != nil {
		t.Fatalf("Run: %v", res.Err)
	}
	if !strings.Contains(res.Stdout, `"mod_id":1.559212036e+09`) && !strings.Contains(res.Stdout, "1559212036") {
		t.Errorf("Stdout (echoed stdin) = %q", res.Stdout)
	}
	if !strings.Contains(res.Stdout, `"instance":"hashima"`) {
		t.Errorf("Stdout missing instance field: %q", res.Stdout)
	}
}

func TestRunNonZeroExit(t *testing.T) {
	script := writeScript(t, `echo "failing" >&2; exit 3`)
	r := &Runner{}
	res := r.Run(context.Background(), script, Context{Instance: "x", Point: "post_render"})
	if res.Err == nil {
		t.Fatal("expected an error for a non-zero exit")
	}
	if res.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", res.ExitCode)
	}
	if !strings.Contains(res.Err.Error(), "failing") {
		t.Errorf("error should include the stderr tail: %v", res.Err)
	}
}

func TestRunMissingScript(t *testing.T) {
	r := &Runner{}
	res := r.Run(context.Background(), filepath.Join(t.TempDir(), "nonexistent.sh"), Context{})
	if res.Err == nil {
		t.Fatal("expected an error for a missing script")
	}
}

func TestRunTimeout(t *testing.T) {
	script := writeScript(t, `sleep 5`)
	r := &Runner{Timeout: 100 * time.Millisecond}

	start := time.Now()
	res := r.Run(context.Background(), script, Context{})
	elapsed := time.Since(start)

	if res.Err == nil {
		t.Fatal("expected a timeout error")
	}
	// The whole process group must be killed promptly; if only the
	// direct child (e.g. a shell) is killed while a grandchild it forked
	// (here, "sleep" itself) is left running and holding the stdout/
	// stderr pipes open, Cmd.Wait blocks until that grandchild exits on
	// its own - defeating Timeout. This must return in well under 5s.
	if elapsed > 2*time.Second {
		t.Fatalf("Run took %v after a 100ms timeout - the child process tree was not killed promptly", elapsed)
	}
}

func TestRunAllStopsOnError(t *testing.T) {
	ok := writeScript(t, `echo ok`)
	fail := writeScript(t, `exit 1`)
	neverRun := writeScript(t, `touch "$MARKER_FILE"`)
	markerDir := t.TempDir()
	marker := filepath.Join(markerDir, "ran")

	r := &Runner{Env: map[string]string{"MARKER_FILE": marker}}
	results := RunAll(context.Background(), r, []string{ok, fail, neverRun}, Context{}, true)

	if len(results) != 2 {
		t.Fatalf("expected 2 results (stopped after the failure), got %d", len(results))
	}
	if results[0].Err != nil || results[1].Err == nil {
		t.Fatalf("results = %+v", results)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the third script should never have run")
	}
}

func TestRunAllContinuesOnError(t *testing.T) {
	ok := writeScript(t, `echo ok`)
	fail := writeScript(t, `exit 1`)
	alsoOK := writeScript(t, `echo also-ok`)

	r := &Runner{}
	results := RunAll(context.Background(), r, []string{ok, fail, alsoOK}, Context{}, false)

	if len(results) != 3 {
		t.Fatalf("expected all 3 scripts to run, got %d results", len(results))
	}
	if results[0].Err != nil || results[1].Err == nil || results[2].Err != nil {
		t.Fatalf("results = %+v", results)
	}
}

func TestRunAllEmpty(t *testing.T) {
	r := &Runner{}
	results := RunAll(context.Background(), r, nil, Context{}, true)
	if len(results) != 0 {
		t.Fatalf("expected no results, got %v", results)
	}
}

func TestLastLine(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"one":               "one",
		"one\n":             "one",
		"one\ntwo\n":        "two",
		"one\ntwo\nthree\n": "three",
		"\n\n":              "",
	}
	for in, want := range cases {
		if got := lastLine(in); got != want {
			t.Errorf("lastLine(%q) = %q, want %q", in, got, want)
		}
	}
}
