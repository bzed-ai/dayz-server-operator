// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFakeSteamcmdProductCmd(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-steamcmd.sh")
	full := "#!/bin/sh\necho \"Waiting for user info...OK\"\n" + script
	if err := os.WriteFile(path, []byte(full), 0o755); err != nil { //nolint:gosec // test fixture, fixed 0755 mode
		t.Fatalf("write fake steamcmd: %v", err)
	}
	return path
}

func TestProductUpdateSuccess(t *testing.T) {
	script := writeFakeSteamcmdProductCmd(t, `
case "$*" in
  *app_update*) echo "Success! App '223350' fully installed." ;;
esac
`)
	out, err := runCmd(t, "product", "update", "223350",
		"--account", "bob", "--command", script, "--install-dir", t.TempDir())
	if err != nil {
		t.Fatalf("product update: %v\n%s", err, out)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("output = %q", out)
	}
}

func TestProductUpdateFailure(t *testing.T) {
	script := writeFakeSteamcmdProductCmd(t, `
case "$*" in
  *app_update*)
    echo "ERROR! Failed to install app '223350'"
    exit 7
    ;;
esac
`)
	out, err := runCmd(t, "product", "update", "223350",
		"--account", "bob", "--command", script, "--install-dir", t.TempDir())
	if err == nil {
		t.Fatal("expected an error for a failed update")
	}
	if !strings.Contains(out, "FAILED") {
		t.Errorf("output = %q", out)
	}
}

func TestProductUpdateAuthRequired(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fake-steamcmd.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"FAILED (Invalid Password)\"\n"), 0o755); err != nil { //nolint:gosec // test fixture
		t.Fatalf("write fake steamcmd: %v", err)
	}
	out, err := runCmd(t, "product", "update", "223350",
		"--account", "bob", "--command", script, "--install-dir", t.TempDir())
	if err == nil {
		t.Fatal("expected an error for auth required")
	}
	if !strings.Contains(out, "AUTH REQUIRED") {
		t.Errorf("output = %q", out)
	}
}

func TestProductUpdateInvalidAppID(t *testing.T) {
	if _, err := runCmd(t, "product", "update", "not-a-number",
		"--account", "bob", "--command", "steamcmd", "--install-dir", t.TempDir()); err == nil {
		t.Fatal("expected an error for a non-numeric app id")
	}
}

func TestProductUpdateRequiresAccountAndInstallDir(t *testing.T) {
	if _, err := runCmd(t, "product", "update", "223350"); err == nil {
		t.Fatal("expected an error for missing required flags")
	}
}

func TestModDownloadSuccess(t *testing.T) {
	script := writeFakeSteamcmdProductCmd(t, `
case "$*" in
  *workshop_download_item*) echo "Success. Downloaded item 111111 to..." ;;
esac
`)
	out, err := runCmd(t, "mod", "download", "111111", "--account", "bob", "--command", script)
	if err != nil {
		t.Fatalf("mod download: %v\n%s", err, out)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("output = %q", out)
	}
}

func TestModDownloadPartialFailure(t *testing.T) {
	script := writeFakeSteamcmdProductCmd(t, `
case "$*" in
  *workshop_download_item*)
    echo "Success. Downloaded item 111111 to..."
    echo "ERROR! Download item 222222 failed (Failure)."
    ;;
esac
`)
	out, err := runCmd(t, "mod", "download", "111111", "222222", "--account", "bob", "--command", script)
	if err == nil {
		t.Fatal("expected an error for a partial failure")
	}
	if !strings.Contains(out, "FAILED") || !strings.Contains(out, "222222") {
		t.Errorf("output = %q", out)
	}
}

func TestModDownloadInvalidItemID(t *testing.T) {
	if _, err := runCmd(t, "mod", "download", "not-a-number", "--account", "bob"); err == nil {
		t.Fatal("expected an error for a non-numeric item id")
	}
}

func TestModDownloadRequiresAccount(t *testing.T) {
	if _, err := runCmd(t, "mod", "download", "111111"); err == nil {
		t.Fatal("expected an error for a missing --account flag")
	}
}
