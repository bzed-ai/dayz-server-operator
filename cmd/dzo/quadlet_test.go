// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const quadletSpecYAML = `
name: dzo-deerisle
description: DayZ instance deerisle
image: localhost/dzo-runtime:latest
network: host
volumes:
  - source: /var/lib/dzo/instances/deerisle/mpmissions
    destination: /dayz/mpmissions
environment:
  FOO: bar
health:
  cmd: /usr/local/bin/dzo health startup
  interval: 60s
  start_period: 45m
  retries: 5
stop_timeout: 30s
`

func TestQuadletRender(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")
	if err := os.WriteFile(path, []byte(quadletSpecYAML), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	out, err := runCmd(t, "quadlet", "render", path)
	if err != nil {
		t.Fatalf("quadlet render: %v", err)
	}
	for _, want := range []string{"Image=localhost/dzo-runtime:latest", "Network=host", "HealthRetries=5", "ContainerStopTimeout=30s"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestQuadletRenderInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")
	bad := "name: n\nimage: i\nnetwork: host\nstop_timeout: not-a-duration\n"
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := runCmd(t, "quadlet", "render", path); err == nil {
		t.Fatal("expected an error for an invalid duration")
	}
}

func TestQuadletRenderMissingFile(t *testing.T) {
	if _, err := runCmd(t, "quadlet", "render", "/nonexistent/spec.yaml"); err == nil {
		t.Fatal("expected an error for a missing spec file")
	}
}

func TestQuadletRenderInvalidSpec(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yaml")
	if err := os.WriteFile(path, []byte("name: n\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := runCmd(t, "quadlet", "render", path); err == nil {
		t.Fatal("expected an error for a spec missing required fields")
	}
}
