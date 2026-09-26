// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"context"
	"sync"
	"time"
)

// BatchNotifier is a Notifier that can also deliver a coalesced batch of
// same-kind events as one message.
type BatchNotifier interface {
	Notifier
	NotifyBatch(ctx context.Context, kind Kind, events []Event) error
}

// Coalescer batches same-kind events within Window into one delivered
// message (§C9: "one message for '12 mods updated, 3 servers
// restarting'"), instead of relaying every event individually. Events of
// different kinds are never merged. Batching only ever groups events that
// share the same Targets selection in practice (see BatchNotifier.NotifyBatch);
// mixed-target bursts of the same kind use the first event's targets.
type Coalescer struct {
	next    BatchNotifier
	window  time.Duration
	onError func(error)

	mu      sync.Mutex
	pending map[Kind][]Event
	timers  map[Kind]*time.Timer

	// afterFunc is a seam for tests; defaults to time.AfterFunc.
	afterFunc func(d time.Duration, f func()) *time.Timer
}

// NewCoalescer wraps next, batching each kind's events for window before
// delivering them together. window <= 0 disables coalescing (every event
// is delivered immediately, individually). onError (optional) receives
// delivery errors from the timer-triggered auto-flush, which has no
// caller to return them to.
func NewCoalescer(next BatchNotifier, window time.Duration, onError func(error)) *Coalescer {
	return &Coalescer{
		next:      next,
		window:    window,
		onError:   onError,
		pending:   map[Kind][]Event{},
		timers:    map[Kind]*time.Timer{},
		afterFunc: time.AfterFunc,
	}
}

// Notify buffers ev for later coalesced delivery (or delivers immediately
// if coalescing is disabled).
func (c *Coalescer) Notify(_ context.Context, ev Event) error {
	if c.window <= 0 {
		return c.next.Notify(context.Background(), ev)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending[ev.Kind] = append(c.pending[ev.Kind], ev)
	if _, scheduled := c.timers[ev.Kind]; !scheduled {
		kind := ev.Kind
		c.timers[kind] = c.afterFunc(c.window, func() { c.autoFlush(kind) })
	}
	return nil
}

func (c *Coalescer) autoFlush(kind Kind) {
	if err := c.flushKind(kind); err != nil && c.onError != nil {
		c.onError(err)
	}
}

func (c *Coalescer) take(kind Kind) []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	events := c.pending[kind]
	delete(c.pending, kind)
	if t, ok := c.timers[kind]; ok {
		t.Stop()
		delete(c.timers, kind)
	}
	return events
}

func (c *Coalescer) flushKind(kind Kind) error {
	events := c.take(kind)
	if len(events) == 0 {
		return nil
	}
	return c.next.NotifyBatch(context.Background(), kind, events)
}

// Flush delivers every pending batch immediately, bypassing the timer
// (used for deterministic tests and graceful shutdown). It returns the
// first error encountered, after attempting every kind.
func (c *Coalescer) Flush() error {
	c.mu.Lock()
	kinds := make([]Kind, 0, len(c.pending))
	for k := range c.pending {
		kinds = append(kinds, k)
	}
	c.mu.Unlock()

	var firstErr error
	for _, k := range kinds {
		if err := c.flushKind(k); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
