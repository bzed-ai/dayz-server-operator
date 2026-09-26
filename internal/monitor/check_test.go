// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package monitor

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEvaluateInstanceDown(t *testing.T) {
	r := EvaluateInstance(InstanceStatus{Name: "x", Up: false})
	if r.Status != StatusCritical {
		t.Errorf("Status = %v, want Critical", r.Status)
	}
}

func TestEvaluateInstanceUnhealthy(t *testing.T) {
	r := EvaluateInstance(InstanceStatus{Name: "x", Up: true, Health: HealthUnhealthy})
	if r.Status != StatusCritical {
		t.Errorf("Status = %v, want Critical", r.Status)
	}
}

func TestEvaluateInstanceStarting(t *testing.T) {
	r := EvaluateInstance(InstanceStatus{Name: "x", Up: true, Health: HealthStarting})
	if r.Status != StatusWarning {
		t.Errorf("Status = %v, want Warning", r.Status)
	}
}

func TestEvaluateInstanceRenderFailed(t *testing.T) {
	r := EvaluateInstance(InstanceStatus{Name: "x", Up: true, Health: HealthHealthy, LastRenderSuccess: false})
	if r.Status != StatusWarning {
		t.Errorf("Status = %v, want Warning", r.Status)
	}
}

func TestEvaluateInstanceOK(t *testing.T) {
	r := EvaluateInstance(InstanceStatus{Name: "x", Up: true, Health: HealthHealthy, LastRenderSuccess: true, Players: 3, MaxPlayers: 60})
	if r.Status != StatusOK {
		t.Errorf("Status = %v, want OK", r.Status)
	}
	if !strings.Contains(r.Output(), "3/60") {
		t.Errorf("Output() = %q", r.Output())
	}
}

func TestEvaluateGlobalSteamAuthRequired(t *testing.T) {
	r := EvaluateGlobal(GlobalStatus{SteamAuthRequired: true})
	if r.Status != StatusWarning {
		t.Errorf("Status = %v, want Warning", r.Status)
	}
}

func TestEvaluateGlobalSteamSessionInvalid(t *testing.T) {
	r := EvaluateGlobal(GlobalStatus{SteamSessionValid: false})
	if r.Status != StatusWarning {
		t.Errorf("Status = %v, want Warning", r.Status)
	}
}

func TestEvaluateGlobalOK(t *testing.T) {
	r := EvaluateGlobal(GlobalStatus{SteamSessionValid: true})
	if r.Status != StatusOK {
		t.Errorf("Status = %v, want OK", r.Status)
	}
}

func TestCheckResultOutputFormat(t *testing.T) {
	r := CheckResult{Status: StatusOK, Message: "all good", Perf: []PerfDatum{{Label: "players", Value: 5}}}
	out := r.Output()
	if !strings.HasPrefix(out, "OK: all good | ") {
		t.Errorf("Output() = %q", out)
	}
	if !strings.Contains(out, "'players'=5") {
		t.Errorf("Output() = %q", out)
	}
}

func TestCheckResultOutputNoPerf(t *testing.T) {
	r := CheckResult{Status: StatusCritical, Message: "down"}
	if r.Output() != "CRITICAL: down" {
		t.Errorf("Output() = %q", r.Output())
	}
}

func TestPerfDatumWithThresholds(t *testing.T) {
	warn, crit := 10.0, 20.0
	p := PerfDatum{Label: "x", Value: 5, Warn: &warn, Crit: &crit}
	if got := p.String(); got != "'x'=5;10;20" {
		t.Errorf("String() = %q", got)
	}
}

func TestStatusExitCodes(t *testing.T) {
	cases := map[Status]int{StatusOK: 0, StatusWarning: 1, StatusCritical: 2, StatusUnknown: 3}
	for s, want := range cases {
		if got := s.ExitCode(); got != want {
			t.Errorf("%v.ExitCode() = %d, want %d", s, got, want)
		}
	}
}

func TestCheckRemoteInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(InstanceStatus{Name: "deerisle", Up: true, Health: HealthHealthy, LastRenderSuccess: true})
	}))
	defer server.Close()

	result, err := CheckRemoteInstance(context.Background(), server.URL, time.Second)
	if err != nil {
		t.Fatalf("CheckRemoteInstance: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("Status = %v", result.Status)
	}
}

func TestCheckRemoteGlobal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Snapshot{Global: GlobalStatus{SteamSessionValid: true}})
	}))
	defer server.Close()

	result, err := CheckRemoteGlobal(context.Background(), server.URL, time.Second)
	if err != nil {
		t.Fatalf("CheckRemoteGlobal: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("Status = %v", result.Status)
	}
}

func TestCheckRemoteHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := CheckRemoteInstance(context.Background(), server.URL, time.Second); err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func TestCheckRemoteInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	if _, err := CheckRemoteInstance(context.Background(), server.URL, time.Second); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestCheckRemoteUnreachable(t *testing.T) {
	conn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := conn.Addr().String()
	_ = conn.Close()

	if _, err := CheckRemoteInstance(context.Background(), "http://"+addr, 500*time.Millisecond); err == nil {
		t.Fatal("expected an error for an unreachable server")
	}
}

func TestCheckA2SSuccess(t *testing.T) {
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = udpConn.Close() }()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, remote, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_ = n
			body := append([]byte{17}, make([]byte, 14)...)
			resp := append([]byte{0xFF, 0xFF, 0xFF, 0xFF, 'I'}, body...)
			if _, err := udpConn.WriteToUDP(resp, remote); err != nil {
				return
			}
		}
	}()

	result := CheckA2S(udpConn.LocalAddr().String(), 2*time.Second)
	if result.Status != StatusOK {
		t.Errorf("Status = %v, message = %q", result.Status, result.Message)
	}
}

func TestCheckA2SFailure(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := conn.LocalAddr().String()
	_ = conn.Close()

	result := CheckA2S(addr, 300*time.Millisecond)
	if result.Status != StatusCritical {
		t.Errorf("Status = %v, want Critical", result.Status)
	}
}
