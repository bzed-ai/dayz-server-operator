// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package health

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeProc builds a minimal /proc-like tree: fakeProc(t, map[pid]struct{comm, cmdline}).
func fakeProc(t *testing.T, procs map[string]struct{ comm, cmdline string }) string {
	t.Helper()
	root := t.TempDir()
	// Non-PID entries that real /proc also has, to prove they're skipped.
	if err := os.WriteFile(filepath.Join(root, "version"), []byte("Linux version 6.12\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "self"), 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}

	for pid, p := range procs {
		dir := filepath.Join(root, pid)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if p.comm != "" {
			if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(p.comm+"\n"), 0o644); err != nil {
				t.Fatalf("write comm: %v", err)
			}
		}
		if p.cmdline != "" {
			if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(p.cmdline), 0o644); err != nil {
				t.Fatalf("write cmdline: %v", err)
			}
		}
	}
	return root
}

func TestProcessRunningMatchesComm(t *testing.T) {
	root := fakeProc(t, map[string]struct{ comm, cmdline string }{
		"1234": {comm: "DayZServer"},
	})
	running, err := ProcessRunning(root, DefaultProcessNames)
	if err != nil {
		t.Fatalf("ProcessRunning: %v", err)
	}
	if !running {
		t.Error("expected the process to be found by comm")
	}
}

func TestProcessRunningMatchesCmdlineBasename(t *testing.T) {
	root := fakeProc(t, map[string]struct{ comm, cmdline string }{
		"1234": {comm: "enfMain", cmdline: "/dayz/DayZServer\x00-config=x\x00"},
	})
	running, err := ProcessRunning(root, []string{"DayZServer"})
	if err != nil {
		t.Fatalf("ProcessRunning: %v", err)
	}
	if !running {
		t.Error("expected the process to be found by cmdline basename")
	}
}

func TestProcessRunningNoMatch(t *testing.T) {
	root := fakeProc(t, map[string]struct{ comm, cmdline string }{
		"1": {comm: "systemd"},
		"2": {comm: "bash"},
	})
	running, err := ProcessRunning(root, DefaultProcessNames)
	if err != nil {
		t.Fatalf("ProcessRunning: %v", err)
	}
	if running {
		t.Error("expected no match")
	}
}

func TestProcessRunningSkipsNonPidEntries(t *testing.T) {
	root := fakeProc(t, map[string]struct{ comm, cmdline string }{})
	running, err := ProcessRunning(root, DefaultProcessNames)
	if err != nil {
		t.Fatalf("ProcessRunning: %v", err)
	}
	if running {
		t.Error("expected no match against an empty process tree")
	}
}

func TestProcessRunningMissingProcRoot(t *testing.T) {
	if _, err := ProcessRunning(filepath.Join(t.TempDir(), "nope"), DefaultProcessNames); err == nil {
		t.Fatal("expected an error for a missing procRoot")
	}
}

func TestProcessRunningIgnoresUnreadableCommFile(t *testing.T) {
	// A pid dir with neither comm nor cmdline readable (process exited
	// mid-scan, a normal /proc race) must not error, just not match.
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "999"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	running, err := ProcessRunning(root, DefaultProcessNames)
	if err != nil {
		t.Fatalf("ProcessRunning: %v", err)
	}
	if running {
		t.Error("expected no match")
	}
}
