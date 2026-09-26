// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestVersionCmd(t *testing.T) {
	out, err := runCmd(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "dev") {
		t.Errorf("version output = %q, want it to mention the dev version", out)
	}
}

func TestConfigValidateCmd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("paths:\n  data: "+dir+"\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	out, err := runCmd(t, "config", "validate", "--config", path)
	if err != nil {
		t.Fatalf("config validate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "is valid") {
		t.Errorf("output = %q", out)
	}
}

func TestConfigValidateCmdMissingFile(t *testing.T) {
	_, err := runCmd(t, "config", "validate", "--config", "/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected an error for a missing config file")
	}
}

func TestServerCfgShowAndDiff(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.cfg")
	newPath := filepath.Join(dir, "new.cfg")
	if err := os.WriteFile(oldPath, []byte(`hostname = "old"; maxPlayers = 40;`), 0o600); err != nil {
		t.Fatalf("write old.cfg: %v", err)
	}
	if err := os.WriteFile(newPath, []byte(`hostname = "new"; maxPlayers = 40;`), 0o600); err != nil {
		t.Fatalf("write new.cfg: %v", err)
	}

	out, err := runCmd(t, "servercfg", "show", oldPath)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out, `hostname = "old";`) {
		t.Errorf("show output = %q", out)
	}

	out, err = runCmd(t, "servercfg", "diff", oldPath, newPath)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !strings.Contains(out, "hostname") {
		t.Errorf("diff output = %q", out)
	}
}

func TestServerCfgShowMissingFile(t *testing.T) {
	_, err := runCmd(t, "servercfg", "show", "/nonexistent/serverDZ.cfg")
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestServerCfgDiffParseError(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.cfg")
	if err := os.WriteFile(badPath, []byte("not valid"), 0o600); err != nil {
		t.Fatalf("write bad.cfg: %v", err)
	}
	goodPath := filepath.Join(dir, "good.cfg")
	if err := os.WriteFile(goodPath, []byte(`hostname = "ok";`), 0o600); err != nil {
		t.Fatalf("write good.cfg: %v", err)
	}
	if _, err := runCmd(t, "servercfg", "diff", badPath, goodPath); err == nil {
		t.Fatal("expected a parse error")
	}
}
