// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotifyTestSendsMessage(t *testing.T) {
	var gotContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct{ Content string }
		_ = json.NewDecoder(r.Body).Decode(&payload)
		gotContent = payload.Content
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	out, err := runCmd(t, "notify", "test", "--url", server.URL, "--message", "hello dzo")
	if err != nil {
		t.Fatalf("notify test: %v", err)
	}
	if !strings.Contains(out, "sent") {
		t.Errorf("output = %q", out)
	}
	if gotContent != "hello dzo" {
		t.Errorf("posted content = %q", gotContent)
	}
}

func TestNotifyTestRequiresURL(t *testing.T) {
	if _, err := runCmd(t, "notify", "test"); err == nil {
		t.Fatal("expected an error for missing --url")
	}
}

func TestNotifyTestHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	if _, err := runCmd(t, "notify", "test", "--url", server.URL); err == nil {
		t.Fatal("expected an error for a non-2xx webhook response")
	}
}
