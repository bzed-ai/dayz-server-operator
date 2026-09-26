// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package instance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bzed-ai/dayz-server-operator/internal/battleye"
)

// A real battleye.Client's Command method must satisfy Commander (that
// is the whole point of this interface): checked at compile time so a
// future signature change in either package fails the build, not a
// runtime type assertion somewhere else.
var _ Commander = (*battleye.Client)(nil)

func TestParsePlayerIDs(t *testing.T) {
	raw := `Players on server:
[#] [IP Address]:[Port] [Ping] [GUID] [Name]
--------------------------------------------------
0    127.0.0.1:2304        50   b1234567890abcdef1234567890abcd1(OK) PlayerOne
1    127.0.0.1:2305        30   b1234567890abcdef1234567890abcd2(OK) PlayerTwo
(2 players in total)
`
	ids := ParsePlayerIDs(raw)
	if len(ids) != 2 || ids[0] != 0 || ids[1] != 1 {
		t.Errorf("ids = %v, want [0 1]", ids)
	}
}

func TestParsePlayerIDsEmpty(t *testing.T) {
	raw := `Players on server:
[#] [IP Address]:[Port] [Ping] [GUID] [Name]
--------------------------------------------------
(0 players in total)
`
	if ids := ParsePlayerIDs(raw); len(ids) != 0 {
		t.Errorf("ids = %v, want none", ids)
	}
}

// fakeCommander records every command sent to it and returns scripted
// responses/errors in order (or a default response after they're
// exhausted).
type fakeCommander struct {
	responses map[string][]string
	errs      map[string]error
	sent      []string
}

func (f *fakeCommander) Command(_ context.Context, command string) (string, error) {
	f.sent = append(f.sent, command)
	if err := f.errs[command]; err != nil {
		return "", err
	}
	if resps, ok := f.responses[command]; ok && len(resps) > 0 {
		resp := resps[0]
		f.responses[command] = resps[1:]
		return resp, nil
	}
	return "", nil
}

func TestLockAndUnlock(t *testing.T) {
	c := &fakeCommander{}
	if err := Lock(context.Background(), c); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := Unlock(context.Background(), c); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if !equalStrings(c.sent, []string{"#lock", "#unlock"}) {
		t.Errorf("sent = %v", c.sent)
	}
}

func TestAnnounce(t *testing.T) {
	c := &fakeCommander{}
	if err := Announce(context.Background(), c, "Restart in 5 minutes"); err != nil {
		t.Fatalf("Announce: %v", err)
	}
	if c.sent[0] != "say -1 Restart in 5 minutes" {
		t.Errorf("sent = %v", c.sent)
	}
}

func TestKickAllStopsWhenNoPlayersLeft(t *testing.T) {
	playersOutput := `0    127.0.0.1:2304        50   guid1(OK) PlayerOne
(1 players in total)
`
	c := &fakeCommander{responses: map[string][]string{
		"players": {playersOutput, "(0 players in total)\n"},
	}}
	if err := KickAll(context.Background(), c, "restart", 3); err != nil {
		t.Fatalf("KickAll: %v", err)
	}
	// Expect: players, kick 0, players (empty -> stop early), #kick -1.
	want := []string{"players", "kick 0 restart", "players", "#kick -1"}
	if !equalStrings(c.sent, want) {
		t.Errorf("sent = %v, want %v", c.sent, want)
	}
}

func TestKickAllRunsAllPassesWhenPlayersKeepReconnecting(t *testing.T) {
	playersOutput := `0    127.0.0.1:2304        50   guid1(OK) PlayerOne
(1 players in total)
`
	c := &fakeCommander{responses: map[string][]string{
		"players": {playersOutput, playersOutput, playersOutput},
	}}
	if err := KickAll(context.Background(), c, "restart", 3); err != nil {
		t.Fatalf("KickAll: %v", err)
	}
	kickCount := 0
	for _, s := range c.sent {
		if s == "kick 0 restart" {
			kickCount++
		}
	}
	if kickCount != 3 {
		t.Errorf("kick count = %d, want 3", kickCount)
	}
	if c.sent[len(c.sent)-1] != "#kick -1" {
		t.Errorf("last command = %q, want #kick -1", c.sent[len(c.sent)-1])
	}
}

func TestKickAllPlayersCommandErrorPropagates(t *testing.T) {
	c := &fakeCommander{errs: map[string]error{"players": errors.New("connection lost")}}
	if err := KickAll(context.Background(), c, "restart", 3); err == nil {
		t.Fatal("expected an error")
	}
}

func TestKickAllKickCommandErrorPropagates(t *testing.T) {
	playersOutput := "0    127.0.0.1:2304        50   guid1(OK) PlayerOne\n(1 players in total)\n"
	c := &fakeCommander{
		responses: map[string][]string{"players": {playersOutput}},
		errs:      map[string]error{"kick 0 restart": errors.New("kick failed")},
	}
	if err := KickAll(context.Background(), c, "restart", 3); err == nil {
		t.Fatal("expected an error")
	}
}

func TestGracefulRestartFullSequence(t *testing.T) {
	playersOutput := "0    127.0.0.1:2304        50   guid1(OK) PlayerOne\n(1 players in total)\n"
	c := &fakeCommander{responses: map[string][]string{"players": {playersOutput, "(0 players in total)\n"}}}

	argsFile := "/dev/null"
	script := writeFakeSystemctl(t, argsFile, "", 0)
	lc := Lifecycle{Command: script}

	var slept time.Duration
	opts := GracefulRestartOptions{
		Reason: "maintenance", Delay: 5 * time.Second,
		Sleep: func(d time.Duration) { slept = d },
	}
	if err := GracefulRestart(context.Background(), c, lc, "deerisle", opts); err != nil {
		t.Fatalf("GracefulRestart: %v", err)
	}
	if c.sent[0] != "#lock" {
		t.Errorf("first command = %q, want #lock", c.sent[0])
	}
	if c.sent[len(c.sent)-1] != "#kick -1" {
		t.Errorf("last RCon command = %q, want #kick -1", c.sent[len(c.sent)-1])
	}
	if slept != 5*time.Second {
		t.Errorf("slept = %v, want 5s", slept)
	}
}

func TestGracefulRestartLockFailureAbortsBeforeRestart(t *testing.T) {
	c := &fakeCommander{errs: map[string]error{"#lock": errors.New("rcon down")}}
	script := writeFakeSystemctl(t, "/dev/null", "", 0)
	lc := Lifecycle{Command: script}

	if err := GracefulRestart(context.Background(), c, lc, "deerisle", GracefulRestartOptions{}); err == nil {
		t.Fatal("expected an error when #lock fails")
	}
}

func TestGracefulRestartDefaultPassesAndSleep(t *testing.T) {
	opts := GracefulRestartOptions{}
	if opts.passes() != 3 {
		t.Errorf("passes() = %d, want 3", opts.passes())
	}
	opts.sleep(0) // must not block or panic with the default time.Sleep
}
