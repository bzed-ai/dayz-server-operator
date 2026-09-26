// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeFakeSystemctlCmd(t *testing.T, exitCode int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-systemctl.sh")
	script := "#!/bin/sh\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil { //nolint:gosec // test fixture, fixed 0755 mode
		t.Fatalf("write fake systemctl: %v", err)
	}
	return path
}

func TestInstanceStart(t *testing.T) {
	script := writeFakeSystemctlCmd(t, 0)
	if _, err := runCmd(t, "instance", "start", "deerisle", "--command", script); err != nil {
		t.Fatalf("instance start: %v", err)
	}
}

func TestInstanceStop(t *testing.T) {
	script := writeFakeSystemctlCmd(t, 0)
	if _, err := runCmd(t, "instance", "stop", "deerisle", "--command", script); err != nil {
		t.Fatalf("instance stop: %v", err)
	}
}

func TestInstanceRestartPlain(t *testing.T) {
	script := writeFakeSystemctlCmd(t, 0)
	if _, err := runCmd(t, "instance", "restart", "deerisle", "--command", script); err != nil {
		t.Fatalf("instance restart: %v", err)
	}
}

func TestInstanceStartFailurePropagates(t *testing.T) {
	script := writeFakeSystemctlCmd(t, 1)
	if _, err := runCmd(t, "instance", "start", "deerisle", "--command", script); err == nil {
		t.Fatal("expected an error for a failing systemctl")
	}
}

func TestInstanceRestartGracefulRequiresRconAddr(t *testing.T) {
	script := writeFakeSystemctlCmd(t, 0)
	if _, err := runCmd(t, "instance", "restart", "deerisle", "--command", script, "--graceful"); err == nil {
		t.Fatal("expected an error for --graceful without --rcon-addr")
	}
}

func TestInstanceRestartGracefulFallsBackWhenRconUnreachable(t *testing.T) {
	script := writeFakeSystemctlCmd(t, 0)
	out, err := runCmd(t, "instance", "restart", "deerisle", "--command", script,
		"--graceful", "--rcon-addr", "127.0.0.1:1", "--rcon-password", "x", "--rcon-timeout", "200ms")
	if err != nil {
		t.Fatalf("instance restart --graceful: %v\n%s", err, out)
	}
	if !strings.Contains(out, "RCon unreachable") {
		t.Errorf("output = %q", out)
	}
}

func TestInstanceAckFailure(t *testing.T) {
	script := writeFakeSystemctlCmd(t, 0)
	gateFile := filepath.Join(t.TempDir(), "gate.json")
	if err := os.WriteFile(gateFile, []byte(`{"failed":true,"input_hash":"sha256:abc"}`), 0o600); err != nil {
		t.Fatalf("write gate file: %v", err)
	}
	if _, err := runCmd(t, "instance", "ack-failure", "deerisle", "--command", script, "--gate-file", gateFile); err != nil {
		t.Fatalf("instance ack-failure: %v", err)
	}
	data, err := os.ReadFile(gateFile) //nolint:gosec // test fixture path under t.TempDir()
	if err != nil {
		t.Fatalf("read gate file: %v", err)
	}
	if strings.Contains(string(data), `"failed": true`) {
		t.Errorf("gate file should be cleared: %s", data)
	}
}

func TestInstanceAckFailureWithoutGateFile(t *testing.T) {
	script := writeFakeSystemctlCmd(t, 0)
	if _, err := runCmd(t, "instance", "ack-failure", "deerisle", "--command", script); err != nil {
		t.Fatalf("instance ack-failure: %v", err)
	}
}
