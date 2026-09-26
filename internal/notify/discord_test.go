// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func newTestNotifier(t *testing.T, webhooks []Webhook, defaultName string) *DiscordNotifier {
	t.Helper()
	ts, err := NewTemplateSet()
	if err != nil {
		t.Fatalf("NewTemplateSet: %v", err)
	}
	return &DiscordNotifier{Webhooks: webhooks, Default: defaultName, Templates: ts}
}

func TestNotifyPostsToDefaultWebhook(t *testing.T) {
	var mu sync.Mutex
	var received []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct{ Content string }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
		}
		mu.Lock()
		received = append(received, payload.Content)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	n := newTestNotifier(t, []Webhook{{Name: "default", URL: server.URL}}, "default")
	if err := n.Notify(context.Background(), Event{Kind: KindHealthUnhealthy, Instance: "deerisle"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || !strings.Contains(received[0], "deerisle") {
		t.Fatalf("received = %v", received)
	}
}

func TestNotifyRejectsUnconfiguredDefault(t *testing.T) {
	n := newTestNotifier(t, nil, "default")
	if err := n.Notify(context.Background(), Event{Kind: KindHealthUnhealthy}); err == nil {
		t.Fatal("expected an error when the default webhook isn't configured")
	}
}

func TestNotifyExplicitTargetsEmptyIsNoOp(t *testing.T) {
	n := newTestNotifier(t, []Webhook{{Name: "default", URL: "http://unused.invalid"}}, "default")
	empty := []string{}
	if err := n.Notify(context.Background(), Event{Kind: KindHealthUnhealthy, Targets: &empty}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
}

func TestNotifyExplicitTargetsMultiple(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	n := newTestNotifier(t, []Webhook{
		{Name: "ops", URL: server.URL + "/ops"},
		{Name: "public", URL: server.URL + "/public"},
	}, "ops")
	targets := []string{"ops", "public"}
	if err := n.Notify(context.Background(), Event{Kind: KindRestartFinished, Instance: "x", Targets: &targets}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits["/ops"] != 1 || hits["/public"] != 1 {
		t.Errorf("hits = %v", hits)
	}
}

func TestNotifyUnknownTargetName(t *testing.T) {
	n := newTestNotifier(t, []Webhook{{Name: "ops", URL: "http://unused.invalid"}}, "ops")
	targets := []string{"nonexistent"}
	if err := n.Notify(context.Background(), Event{Kind: KindHealthUnhealthy, Targets: &targets}); err == nil {
		t.Fatal("expected an error for an unknown webhook name")
	}
}

func TestNotifyHTTPErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	n := newTestNotifier(t, []Webhook{{Name: "default", URL: server.URL}}, "default")
	if err := n.Notify(context.Background(), Event{Kind: KindHealthUnhealthy}); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

func TestNotifyUsesPostFuncSeam(t *testing.T) {
	var gotURL string
	var gotBody []byte
	n := newTestNotifier(t, []Webhook{{Name: "default", URL: "http://example.invalid/hook"}}, "default")
	n.PostFunc = func(ctx context.Context, url string, body []byte) error {
		gotURL = url
		gotBody = body
		return nil
	}
	if err := n.Notify(context.Background(), Event{Kind: KindHealthUnhealthy, Instance: "a"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if gotURL != "http://example.invalid/hook" {
		t.Errorf("gotURL = %q", gotURL)
	}
	var payload struct{ Content string }
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(payload.Content, "a") {
		t.Errorf("payload.Content = %q", payload.Content)
	}
}

func TestNotifyPostFuncError(t *testing.T) {
	n := newTestNotifier(t, []Webhook{{Name: "default", URL: "http://example.invalid/hook"}}, "default")
	n.PostFunc = func(ctx context.Context, url string, body []byte) error {
		return context.DeadlineExceeded
	}
	if err := n.Notify(context.Background(), Event{Kind: KindHealthUnhealthy}); err == nil {
		t.Fatal("expected the PostFunc error to propagate")
	}
}

func TestNotifyBatch(t *testing.T) {
	var mu sync.Mutex
	var received []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct{ Content string }
		_ = json.NewDecoder(r.Body).Decode(&payload)
		mu.Lock()
		received = append(received, payload.Content)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	n := newTestNotifier(t, []Webhook{{Name: "default", URL: server.URL}}, "default")
	events := []Event{
		{Kind: KindModUpdateDetected, Instance: "a"},
		{Kind: KindModUpdateDetected, Instance: "b"},
	}
	if err := n.NotifyBatch(context.Background(), KindModUpdateDetected, events); err != nil {
		t.Fatalf("NotifyBatch: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || !strings.Contains(received[0], "2") {
		t.Fatalf("received = %v", received)
	}
}

func TestNotifyBatchEmpty(t *testing.T) {
	n := newTestNotifier(t, nil, "default")
	if err := n.NotifyBatch(context.Background(), KindModUpdateDetected, nil); err != nil {
		t.Fatalf("NotifyBatch(nil) = %v", err)
	}
}
