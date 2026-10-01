// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupDryRunCommand(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	writeFile(t, cfg, "paths:\n  data: "+filepath.Join(dir, "data")+"\n")
	images := filepath.Join(dir, "images")
	for _, n := range []string{"runtime", "steamcmd"} {
		writeFile(t, filepath.Join(images, n, "Containerfile"), "FROM scratch\n")
	}
	if os.Geteuid() == 0 {
		t.Skip("dzo setup refuses to run as root")
	}
	out, err := runCmd(t, "setup", "--dry-run", "--config", cfg, "--images-dir", images, "--unit-dir", filepath.Join(dir, "units"))
	if err != nil && !strings.Contains(out, "podman is not installed") {
		t.Fatalf("setup --dry-run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "todo  create paths.data") || !strings.Contains(out, "dzo-image-refresh.timer") {
		t.Errorf("output:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "data")); !os.IsNotExist(err) {
		t.Error("--dry-run created the data directory")
	}
	if _, err := runCmd(t, "setup", "--config", filepath.Join(dir, "none.yaml")); err == nil {
		t.Error("a missing config must fail")
	}
}
