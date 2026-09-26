// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func fakeProcRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "1234"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "1234", "comm"), []byte("DayZServer\n"), 0o600); err != nil {
		t.Fatalf("write comm: %v", err)
	}
	return root
}

func fakeA2SForCLI(t *testing.T) string {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	go func() {
		buf := make([]byte, 4096)
		for {
			n, remote, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_ = n
			body := append([]byte{17}, make([]byte, 14)...)
			resp := append([]byte{0xFF, 0xFF, 0xFF, 0xFF, 'I'}, body...)
			if _, err := conn.WriteToUDP(resp, remote); err != nil {
				return
			}
		}
	}()
	return conn.LocalAddr().String()
}

func TestHealthStartupSuccess(t *testing.T) {
	_, err := runCmd(t, "health", "startup",
		"--query", fakeA2SForCLI(t),
		"--proc-root", fakeProcRoot(t),
		"--timeout", "2s",
	)
	if err != nil {
		t.Fatalf("health startup: %v", err)
	}
}

func TestHealthStartupMissingQuery(t *testing.T) {
	if _, err := runCmd(t, "health", "startup", "--proc-root", fakeProcRoot(t)); err == nil {
		t.Fatal("expected an error for missing --query")
	}
}

func TestHealthLiveProcessNotRunning(t *testing.T) {
	_, err := runCmd(t, "health", "live",
		"--query", fakeA2SForCLI(t),
		"--proc-root", t.TempDir(),
		"--timeout", "500ms",
	)
	if err == nil {
		t.Fatal("expected an error when the process isn't running")
	}
}
