// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package instance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Announcement is the countdown before a restart (the legacy dayz_restart
// options): players are told every minute, the server is locked against new
// logins Lock minutes before the end, and Delay seconds pass between the last
// kick and the stop.
type Announcement struct {
	Minutes int
	Lock    int
	Delay   int // seconds
	Text    string
}

// Restarter restarts an instance gracefully: announce, lock, kick, stop, do
// something while it is down (an update's snapshot and unit switch), start.
type Restarter struct {
	Lifecycle Lifecycle
	// Dial connects to the instance's RCon; an error means it is unreachable
	// and the restart goes ahead at once, without announcing (§C8).
	Dial func(ctx context.Context) (Commander, error)
	// Sleep waits; tests replace it.
	Sleep func(time.Duration)
	// Log reports what happens, for the journal.
	Log func(format string, a ...any)
	// Cancelled, if set, is asked every minute of the countdown; true unlocks
	// the server and ends the restart (dzo restart --cancel).
	Cancelled func() bool
}

// ErrCancelled is returned by a restart that was cancelled during its countdown.
var ErrCancelled = errors.New("instance: the restart was cancelled")

func (r Restarter) sleep(d time.Duration) {
	if r.Sleep != nil {
		r.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (r Restarter) log(format string, a ...any) {
	if r.Log != nil {
		r.Log(format, a...)
	}
}

// Restart runs the sequence. While the instance is down, between runs; if it
// fails the instance is started again unchanged and the error is returned, so a
// failed snapshot never leaves the server down. An instance that is not running
// is not announced to: it is only started (after between).
func (r Restarter) Restart(ctx context.Context, name string, a Announcement, between func(ctx context.Context) error) error {
	active, _ := r.Lifecycle.IsActive(ctx, name)
	if active {
		if c, err := r.Dial(ctx); err != nil {
			r.log("RCon unreachable (%v), stopping without announcement", err)
		} else {
			err := r.countdown(ctx, c, a)
			if cl, ok := c.(io.Closer); ok {
				_ = cl.Close()
			}
			if err != nil {
				return err
			}
		}
		if err := r.Lifecycle.Stop(ctx, name); err != nil {
			return err
		}
	}
	if between != nil {
		if err := between(ctx); err != nil {
			if active {
				if serr := r.Lifecycle.Start(ctx, name); serr != nil {
					return fmt.Errorf("%w (and starting %s again failed: %v)", err, name, serr)
				}
			}
			return err
		}
	}
	return r.Lifecycle.Start(ctx, name)
}

func (r Restarter) countdown(ctx context.Context, c Commander, a Announcement) error {
	text := strings.TrimSpace(a.Text)
	if text == "" {
		text = "Server restart"
	}
	locked := false
	for left := a.Minutes; left > 0; left-- {
		if !locked && a.Lock > 0 && left <= a.Lock {
			if err := Lock(ctx, c); err != nil {
				return fmt.Errorf("instance: lock: %w", err)
			}
			locked = true
		}
		unit := "minutes"
		if left == 1 {
			unit = "minute"
		}
		if err := Announce(ctx, c, fmt.Sprintf("%s in %d %s", text, left, unit)); err != nil {
			return fmt.Errorf("instance: announce: %w", err)
		}
		r.sleep(time.Minute)
		if err := ctx.Err(); err != nil {
			_ = Unlock(context.Background(), c) // a cancelled restart must not leave the server locked
			return err
		}
		if r.Cancelled != nil && r.Cancelled() {
			_ = Announce(ctx, c, "The restart was cancelled")
			_ = Unlock(ctx, c)
			return ErrCancelled
		}
	}
	if !locked {
		if err := Lock(ctx, c); err != nil {
			return fmt.Errorf("instance: lock: %w", err)
		}
	}
	if err := KickAll(ctx, c, text, 3); err != nil {
		return err
	}
	if a.Delay > 0 {
		r.sleep(time.Duration(a.Delay) * time.Second)
	}
	return nil
}
