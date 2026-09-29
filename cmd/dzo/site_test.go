// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// siteConfig writes a config.yaml whose site checkout is the synthetic site
// repo used by internal/resolve's golden tests.
func siteConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	site, err := filepath.Abs("../../internal/resolve/testdata/site")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("paths:\n  data: "+dir+"\n  site: "+site+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInstanceShow(t *testing.T) {
	out, err := runCmd(t, "instance", "show", "alpha", "--config", siteConfig(t))
	if err != nil {
		t.Fatalf("instance show: %v\n%s", err, out)
	}
	for _, want := range []string{"name: alpha", "app_id: 223350", "missing:", "mod 1559212036: not downloaded"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestInstanceShowQuadletNeedsInstall(t *testing.T) {
	if _, err := runCmd(t, "instance", "show", "alpha", "--quadlet", "--config", siteConfig(t)); err == nil {
		t.Fatal("expected an error for --quadlet on an instance that is not installed")
	}
}

func TestInstanceShowQuadlet(t *testing.T) {
	cfg := siteConfig(t)
	// "install" everything alpha needs
	data := filepath.Dir(cfg)
	for _, p := range []string{"products/dayz-stable", "workshop/221100/1559212036", "workshop/221100/1828439124"} {
		root := filepath.Join(data, "cache", p)
		if err := os.MkdirAll(filepath.Join(root, "1"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("1", filepath.Join(root, "current")); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runCmd(t, "instance", "show", "alpha", "--quadlet", "--config", cfg)
	if err != nil {
		t.Fatalf("instance show --quadlet: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ContainerName=dzo-alpha") {
		t.Errorf("output = %s", out)
	}
}

func TestInstanceShowUnknown(t *testing.T) {
	if _, err := runCmd(t, "instance", "show", "nope", "--config", siteConfig(t)); err == nil {
		t.Fatal("expected an error for an unknown instance")
	}
}

func TestConfigValidateResolvesSite(t *testing.T) {
	out, err := runCmd(t, "config", "validate", "--config", siteConfig(t))
	if err != nil {
		t.Fatalf("config validate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "2 instances resolve") || !strings.Contains(out, "note: alpha:") {
		t.Errorf("output = %s", out)
	}
}

func TestConfigValidateReportsBrokenSite(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "site", "instances", "x")
	if err := os.MkdirAll(bad, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "instance.yaml"), []byte("name: x\nbogus: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfg, []byte("paths:\n  data: "+dir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "config", "validate", "--config", cfg); err == nil {
		t.Fatal("expected an error for an unknown key in instance.yaml")
	}
}

func TestConfigValidateUnresolvableSite(t *testing.T) {
	dir := t.TempDir()
	inst := filepath.Join(dir, "site", "instances", "x")
	if err := os.MkdirAll(inst, 0o750); err != nil {
		t.Fatal(err)
	}
	y := "name: x\nproduct: ghost\nmap: m\nmission_source: {git: g, ref: r, path: p}\nports: {game: 1}\nnetwork: host\n"
	if err := os.WriteFile(filepath.Join(inst, "instance.yaml"), []byte(y), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfg, []byte("paths:\n  data: "+dir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "config", "validate", "--config", cfg); err == nil || !strings.Contains(err.Error(), "unknown product") {
		t.Fatalf("err = %v, want unknown product", err)
	}
}
