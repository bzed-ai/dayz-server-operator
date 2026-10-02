// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package instance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeSystemctl returns a Lifecycle whose systemctl logs its calls to a file
// and answers is-active from the file's presence.
func fakeSystemctl(t *testing.T, active bool) (Lifecycle, func() []string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$@\" >> " + log + "\n"
	if active {
		script += "[ \"$2\" = is-active ] && echo active\n"
	} else {
		script += "[ \"$2\" = is-active ] && { echo inactive; exit 3; }\n"
	}
	script += "exit 0\n"
	p := filepath.Join(dir, "systemctl")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}
	return Lifecycle{Command: p, UserMode: true}, func() []string {
		b, _ := os.ReadFile(log)
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
}

type recCommander struct {
	cmds    []string
	players string
}

func (r *recCommander) Command(_ context.Context, c string) (string, error) {
	r.cmds = append(r.cmds, c)
	if c == "players" {
		out := r.players
		r.players = "" // everybody is gone after the first kick pass
		return out, nil
	}
	return "", nil
}

func TestRestartAnnouncesLocksKicksStopsAndStarts(t *testing.T) {
	lc, calls := fakeSystemctl(t, true)
	cmd := &recCommander{players: "Players on server:\n[#] [IP Address]:[Port] [Ping] [GUID] [Name]\n--------\n0   1.2.3.4:5 10 abc(OK) Bob\n(1 players in total)\n"}
	var slept []time.Duration
	r := Restarter{Lifecycle: lc, Dial: func(context.Context) (Commander, error) { return cmd, nil }, Sleep: func(d time.Duration) { slept = append(slept, d) }}
	var order []string
	err := r.Restart(context.Background(), "x", Announcement{Minutes: 3, Lock: 2, Delay: 4, Text: "MOD UPDATE"}, func(context.Context) error {
		order = append(order, "between")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"say -1 MOD UPDATE in 3 minutes", "#lock", "say -1 MOD UPDATE in 2 minutes", "say -1 MOD UPDATE in 1 minute", "players", "kick 0 MOD UPDATE", "players", "#kick -1"}
	if strings.Join(cmd.cmds, "|") != strings.Join(want, "|") {
		t.Errorf("RCon commands:\n got %q\nwant %q", cmd.cmds, want)
	}
	if len(slept) != 4 || slept[0] != time.Minute || slept[3] != 4*time.Second {
		t.Errorf("sleeps = %v (three minutes of countdown, then the delay)", slept)
	}
	got := calls()
	if len(got) < 3 || !strings.Contains(got[len(got)-2], "stop") || !strings.HasSuffix(got[len(got)-1], "start dzo-x.service") {
		t.Errorf("systemctl calls = %q: stop, then start", got)
	}
	if len(order) != 1 {
		t.Error("between must run once, while the instance is down")
	}
}

func TestRestartWithoutRConStopsAtOnce(t *testing.T) {
	lc, calls := fakeSystemctl(t, true)
	var logged string
	r := Restarter{Lifecycle: lc, Dial: func(context.Context) (Commander, error) { return nil, errors.New("connection refused") }, Sleep: func(time.Duration) { t.Error("nothing to wait for without RCon") }, Log: func(f string, a ...any) { logged = f }}
	if err := r.Restart(context.Background(), "x", Announcement{Minutes: 10}, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logged, "RCon unreachable") || !strings.Contains(strings.Join(calls(), "\n"), "stop") {
		t.Errorf("logged %q calls %v", logged, calls())
	}
}

func TestRestartOfAStoppedInstanceOnlyStartsIt(t *testing.T) {
	lc, calls := fakeSystemctl(t, false)
	r := Restarter{Lifecycle: lc, Dial: func(context.Context) (Commander, error) { t.Error("a stopped server has no RCon"); return nil, nil }}
	ran := false
	if err := r.Restart(context.Background(), "x", Announcement{Minutes: 5}, func(context.Context) error { ran = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if !ran || strings.Contains(strings.Join(calls(), "\n"), " stop ") {
		t.Errorf("between ran %v, calls %v", ran, calls())
	}
}

func TestAFailingBetweenStartsTheInstanceAgain(t *testing.T) {
	lc, calls := fakeSystemctl(t, true)
	r := Restarter{Lifecycle: lc, Dial: func(context.Context) (Commander, error) { return &recCommander{}, nil }, Sleep: func(time.Duration) {}}
	boom := errors.New("snapshot failed")
	err := r.Restart(context.Background(), "x", Announcement{}, func(context.Context) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	got := calls()
	if !strings.HasSuffix(got[len(got)-1], "start dzo-x.service") {
		t.Errorf("the server must be running again, unchanged: %v", got)
	}
}

func TestCancelledCountdownUnlocksTheServer(t *testing.T) {
	lc, _ := fakeSystemctl(t, true)
	cmd := &recCommander{}
	ctx, cancel := context.WithCancel(context.Background())
	r := Restarter{Lifecycle: lc, Dial: func(context.Context) (Commander, error) { return cmd, nil }, Sleep: func(time.Duration) { cancel() }}
	if err := r.Restart(ctx, "x", Announcement{Minutes: 5, Lock: 5}, nil); err == nil {
		t.Fatal("a cancelled restart must stop")
	}
	if cmd.cmds[len(cmd.cmds)-1] != "#unlock" {
		t.Errorf("a locked server must be unlocked again: %v", cmd.cmds)
	}
}

func TestACancelMarkerEndsTheCountdown(t *testing.T) {
	lc, calls := fakeSystemctl(t, true)
	cmd := &recCommander{}
	cancelled := false
	r := Restarter{Lifecycle: lc, Dial: func(context.Context) (Commander, error) { return cmd, nil },
		Sleep: func(time.Duration) { cancelled = true }, Cancelled: func() bool { return cancelled }}
	err := r.Restart(context.Background(), "x", Announcement{Minutes: 10, Lock: 5}, nil)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("err = %v", err)
	}
	if cmd.cmds[len(cmd.cmds)-1] != "#unlock" || strings.Contains(strings.Join(calls(), "\n"), " stop ") {
		t.Errorf("a cancelled restart unlocks and does not stop: %v %v", cmd.cmds, calls())
	}
}
