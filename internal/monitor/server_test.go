// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package monitor

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerMetrics(t *testing.T) {
	h := Handler(SourceFunc(func() (Snapshot, error) { return sampleSnapshot(), nil }))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestHandlerStatusGlobal(t *testing.T) {
	h := Handler(SourceFunc(func() (Snapshot, error) { return sampleSnapshot(), nil }))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/status")
	if err != nil {
		t.Fatalf("GET /status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestHandlerStatusInstance(t *testing.T) {
	h := Handler(SourceFunc(func() (Snapshot, error) { return sampleSnapshot(), nil }))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/status/deerisle")
	if err != nil {
		t.Fatalf("GET /status/deerisle: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestHandlerStatusInstanceNotFound(t *testing.T) {
	h := Handler(SourceFunc(func() (Snapshot, error) { return sampleSnapshot(), nil }))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/status/nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHandlerStatusEmptyName(t *testing.T) {
	h := Handler(SourceFunc(func() (Snapshot, error) { return sampleSnapshot(), nil }))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/status/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHandlerSourceError(t *testing.T) {
	h := Handler(SourceFunc(func() (Snapshot, error) { return Snapshot{}, errors.New("boom") }))
	server := httptest.NewServer(h)
	defer server.Close()

	for _, path := range []string{"/metrics", "/status", "/status/x"} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("GET %s: status = %d, want 500", path, resp.StatusCode)
		}
	}
}

func TestSnapshotFind(t *testing.T) {
	snap := sampleSnapshot()
	if _, ok := snap.Find("deerisle"); !ok {
		t.Error("expected to find deerisle")
	}
	if _, ok := snap.Find("nope"); ok {
		t.Error("expected not to find nope")
	}
}
