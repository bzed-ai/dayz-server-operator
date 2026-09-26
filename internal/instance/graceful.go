// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package instance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Commander is the subset of internal/battleye.Client's interface the
// graceful restart sequence needs; a real *battleye.Client satisfies it
// directly (its Command method has this exact signature).
type Commander interface {
	Command(ctx context.Context, command string) (string, error)
}

// ParsePlayerIDs extracts client slot ids from BattlEye's "players"
// command output. Real BE RCon output is not verified against a live
// server from this environment; the format parsed here (a header line, a
// "---" separator, one "<id> <ip:port> <ping> <guid>(status) <name>" line
// per player, a "(N players in total)" footer) is the widely documented
// BE RCon players layout.
func ParsePlayerIDs(raw string) []int {
	var ids []int
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "(") ||
			strings.HasPrefix(line, "Players") || strings.HasPrefix(line, "[") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		id, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// Lock sends BE's "#lock" command (no new logins).
func Lock(ctx context.Context, c Commander) error {
	_, err := c.Command(ctx, "#lock")
	return err
}

// Unlock sends BE's "#unlock" command (`dzo restart <name> --cancel`
// uses this to undo a Lock without going through with the restart).
func Unlock(ctx context.Context, c Commander) error {
	_, err := c.Command(ctx, "#unlock")
	return err
}

// Announce sends text as an in-game broadcast ("say -1 <text>", -1
// meaning every connected player).
func Announce(ctx context.Context, c Commander, text string) error {
	_, err := c.Command(ctx, "say -1 "+text)
	return err
}

// KickAll runs up to passes rounds of "players" + "kick <id> <reason>"
// (players who reconnect between passes get caught by the next one),
// then a final catch-all "#kick -1" (§C8). It stops early once a pass
// finds nobody left to kick.
func KickAll(ctx context.Context, c Commander, reason string, passes int) error {
	for i := 0; i < passes; i++ {
		raw, err := c.Command(ctx, "players")
		if err != nil {
			return fmt.Errorf("instance: list players: %w", err)
		}
		ids := ParsePlayerIDs(raw)
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			if _, err := c.Command(ctx, fmt.Sprintf("kick %d %s", id, reason)); err != nil {
				return fmt.Errorf("instance: kick %d: %w", id, err)
			}
		}
	}
	if _, err := c.Command(ctx, "#kick -1"); err != nil {
		return fmt.Errorf("instance: final kick -1: %w", err)
	}
	return nil
}

// GracefulRestartOptions configures GracefulRestart.
type GracefulRestartOptions struct {
	Reason     string        // passed to each kick command
	KickPasses int           // defaults to 3, matching the legacy dayz_restart behaviour
	Delay      time.Duration // wait after the final kick before the actual restart
	Sleep      func(time.Duration)
}

func (o GracefulRestartOptions) passes() int {
	if o.KickPasses <= 0 {
		return 3
	}
	return o.KickPasses
}

func (o GracefulRestartOptions) sleep(d time.Duration) {
	if o.Sleep != nil {
		o.Sleep(d)
		return
	}
	time.Sleep(d)
}

// GracefulRestart runs the lock -> kick-all -> wait -> restart sequence
// (§C8, port of legacy dayz_restart's final phase; the earlier
// announcement countdown is the caller's scheduling concern, not this
// function's). If RCon is unreachable at all, callers fall back to
// Lifecycle.Restart directly rather than calling this (§C8: "If RCon is
// unreachable: log it and restart via systemd immediately").
func GracefulRestart(ctx context.Context, c Commander, lc Lifecycle, name string, opts GracefulRestartOptions) error {
	if err := Lock(ctx, c); err != nil {
		return fmt.Errorf("instance: lock: %w", err)
	}
	if err := KickAll(ctx, c, opts.Reason, opts.passes()); err != nil {
		return err
	}
	if opts.Delay > 0 {
		opts.sleep(opts.Delay)
	}
	return lc.Restart(ctx, name)
}
