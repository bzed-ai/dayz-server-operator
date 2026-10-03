// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bzed/dayz-server-operator/internal/monitor"
)

func TestCheckRemoteInstanceOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(monitor.InstanceStatus{
			Name: "deerisle", Up: true, Health: monitor.HealthHealthy, LastRenderSuccess: true,
		})
	}))
	defer server.Close()

	out, err := runCmd(t, "check", "remote", "--url", server.URL, "--instance", "deerisle")
	if err != nil {
		t.Fatalf("check remote: %v", err)
	}
	if !strings.HasPrefix(out, "OK:") {
		t.Errorf("output = %q", out)
	}
}

func TestCheckRemoteInstanceCritical(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(monitor.InstanceStatus{Name: "deerisle", Up: false})
	}))
	defer server.Close()

	out, err := runCmd(t, "check", "remote", "--url", server.URL, "--instance", "deerisle")
	if err == nil {
		t.Fatal("expected a non-nil error for a CRITICAL check result")
	}
	var exitErr *checkExitErr
	if ce, ok := err.(*checkExitErr); ok {
		exitErr = ce
	}
	if exitErr == nil || exitErr.code != 2 {
		t.Fatalf("err = %v, want a *checkExitErr with code 2", err)
	}
	if !strings.HasPrefix(out, "CRITICAL:") {
		t.Errorf("output = %q", out)
	}
}

func TestCheckRemoteGlobalOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(monitor.Snapshot{Global: monitor.GlobalStatus{SteamSessionValid: true}})
	}))
	defer server.Close()

	out, err := runCmd(t, "check", "remote", "--url", server.URL)
	if err != nil {
		t.Fatalf("check remote: %v", err)
	}
	if !strings.HasPrefix(out, "OK:") {
		t.Errorf("output = %q", out)
	}
}

func TestCheckRemoteUnknownOnFetchFailure(t *testing.T) {
	conn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := conn.Addr().String()
	_ = conn.Close()

	out, err := runCmd(t, "check", "remote", "--url", "http://"+addr, "--timeout", "300ms")
	if err == nil {
		t.Fatal("expected an error")
	}
	var exitErr *checkExitErr
	if ce, ok := err.(*checkExitErr); ok {
		exitErr = ce
	}
	if exitErr == nil || exitErr.code != 3 {
		t.Fatalf("err = %v, want a *checkExitErr with code 3 (UNKNOWN)", err)
	}
	if !strings.HasPrefix(out, "UNKNOWN:") {
		t.Errorf("output = %q", out)
	}
}

func TestCheckRemoteRequiresURL(t *testing.T) {
	if _, err := runCmd(t, "check", "remote"); err == nil {
		t.Fatal("expected an error for missing --url")
	}
}

func TestCheckA2SCmd(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := conn.LocalAddr().String()
	_ = conn.Close() // closed on purpose: nothing answers, expect CRITICAL

	out, err := runCmd(t, "check", "a2s", addr, "--timeout", "300ms")
	if err == nil {
		t.Fatal("expected an error for an unreachable query port")
	}
	if !strings.HasPrefix(out, "CRITICAL:") {
		t.Errorf("output = %q", out)
	}
}
