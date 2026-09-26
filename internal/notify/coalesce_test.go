// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeBatchNotifier records every Notify/NotifyBatch call.
type fakeBatchNotifier struct {
	mu      sync.Mutex
	single  []Event
	batches [][]Event
	err     error
}

func (f *fakeBatchNotifier) Notify(_ context.Context, ev Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.single = append(f.single, ev)
	return f.err
}

func (f *fakeBatchNotifier) NotifyBatch(_ context.Context, _ Kind, events []Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches = append(f.batches, events)
	return f.err
}

func (f *fakeBatchNotifier) snapshot() (single []Event, batches [][]Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Event(nil), f.single...), append([][]Event(nil), f.batches...)
}

func TestCoalescerDisabledDeliversImmediately(t *testing.T) {
	fake := &fakeBatchNotifier{}
	c := NewCoalescer(fake, 0, nil)
	if err := c.Notify(context.Background(), Event{Kind: KindHealthUnhealthy, Instance: "a"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	single, batches := fake.snapshot()
	if len(single) != 1 || len(batches) != 0 {
		t.Fatalf("single=%v batches=%v", single, batches)
	}
}

func TestCoalescerBuffersUntilFlush(t *testing.T) {
	fake := &fakeBatchNotifier{}
	c := NewCoalescer(fake, time.Hour, nil) // long window: only Flush() delivers in this test
	for _, inst := range []string{"a", "b", "c"} {
		if err := c.Notify(context.Background(), Event{Kind: KindModUpdateDetected, Instance: inst}); err != nil {
			t.Fatalf("Notify: %v", err)
		}
	}
	if _, batches := fake.snapshot(); len(batches) != 0 {
		t.Fatalf("expected nothing delivered before Flush, got %v", batches)
	}

	if err := c.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	_, batches := fake.snapshot()
	if len(batches) != 1 || len(batches[0]) != 3 {
		t.Fatalf("batches = %v", batches)
	}
}

func TestCoalescerSeparatesKinds(t *testing.T) {
	fake := &fakeBatchNotifier{}
	c := NewCoalescer(fake, time.Hour, nil)
	if err := c.Notify(context.Background(), Event{Kind: KindModUpdateDetected}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if err := c.Notify(context.Background(), Event{Kind: KindHealthUnhealthy}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if err := c.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	_, batches := fake.snapshot()
	if len(batches) != 2 {
		t.Fatalf("expected 2 separate batches (one per kind), got %v", batches)
	}
}

func TestCoalescerFlushIsIdempotent(t *testing.T) {
	fake := &fakeBatchNotifier{}
	c := NewCoalescer(fake, time.Hour, nil)
	if err := c.Notify(context.Background(), Event{Kind: KindHealthUnhealthy}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if err := c.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := c.Flush(); err != nil {
		t.Fatalf("second Flush: %v", err)
	}
	_, batches := fake.snapshot()
	if len(batches) != 1 {
		t.Fatalf("expected exactly one delivery, got %v", batches)
	}
}

func TestCoalescerFlushWithNothingPending(t *testing.T) {
	fake := &fakeBatchNotifier{}
	c := NewCoalescer(fake, time.Hour, nil)
	if err := c.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	_, batches := fake.snapshot()
	if len(batches) != 0 {
		t.Fatalf("expected no deliveries, got %v", batches)
	}
}

func TestCoalescerFlushReturnsFirstError(t *testing.T) {
	fake := &fakeBatchNotifier{err: errors.New("boom")}
	c := NewCoalescer(fake, time.Hour, nil)
	if err := c.Notify(context.Background(), Event{Kind: KindHealthUnhealthy}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if err := c.Flush(); err == nil {
		t.Fatal("expected Flush to report the delivery error")
	}
}

func TestCoalescerAutoFlushOnTimer(t *testing.T) {
	fake := &fakeBatchNotifier{}
	var onErrCalled bool
	c := NewCoalescer(fake, 20*time.Millisecond, func(error) { onErrCalled = true })

	if err := c.Notify(context.Background(), Event{Kind: KindHealthUnhealthy, Instance: "a"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if err := c.Notify(context.Background(), Event{Kind: KindHealthUnhealthy, Instance: "b"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, batches := fake.snapshot(); len(batches) == 1 {
			if len(batches[0]) != 2 {
				t.Fatalf("auto-flushed batch = %v, want 2 events", batches[0])
			}
			if onErrCalled {
				t.Error("onError should not have been called on success")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the auto-flush to deliver the batch")
}

func TestCoalescerOnErrorCalledFromAutoFlush(t *testing.T) {
	fake := &fakeBatchNotifier{err: errors.New("boom")}
	errCh := make(chan error, 1)
	c := NewCoalescer(fake, 10*time.Millisecond, func(err error) { errCh <- err })

	if err := c.Notify(context.Background(), Event{Kind: KindHealthUnhealthy}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected a non-nil error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for onError")
	}
}
