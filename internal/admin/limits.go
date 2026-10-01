// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package admin

import (
	"strconv"
	"sync"
	"time"
)

func itoa(n int) string { return strconv.Itoa(n) }

// Limiter allows n events per window per key (a sliding window of event
// times). It guards the mod endpoint against token guessing and admin
// actions against runaway scripts (§C16: rate limits per admin and target).
type Limiter struct {
	n      int
	window time.Duration
	now    func() time.Time
	mu     sync.Mutex
	events map[string][]time.Time
}

// NewLimiter returns a Limiter of n events per window.
func NewLimiter(n int, window time.Duration) *Limiter {
	return &Limiter{n: n, window: window, now: time.Now, events: map[string][]time.Time{}}
}

func (l *Limiter) prune(key string, now time.Time) []time.Time {
	kept := l.events[key][:0]
	for _, t := range l.events[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.events, key)
	} else {
		l.events[key] = kept
	}
	return kept
}

// Allow records an event and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.prune(key, now)) >= l.n {
		return false
	}
	l.events[key] = append(l.events[key], now)
	return true
}

// Blocked reports whether the key is already at its limit, without
// recording anything.
func (l *Limiter) Blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(key, l.now())) >= l.n
}
