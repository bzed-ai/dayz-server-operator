// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "x"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// renderSetup builds a data dir with a one-instance site repo whose mission
// source is a local git repo, plus an installed product build that ships a
// fallback mission. It returns the config path and the instance root.
func renderSetup(t *testing.T) (cfg, instRoot, missionRepo string) {
	t.Helper()
	data := t.TempDir()
	missionRepo = filepath.Join(t.TempDir(), "mission")
	writeFile(t, filepath.Join(missionRepo, "empty.m", "init.c"), "init")
	writeFile(t, filepath.Join(missionRepo, "empty.m", "db", "types.xml"), "types")
	commitAll(t, missionRepo)

	writeFile(t, filepath.Join(data, "site", "instances", "x", "instance.yaml"), `name: x
product: dayz-stable
map: empty.m
mission_source: {git: `+missionRepo+`, ref: main, path: empty.m}
fallback_mission: dayzOffline.fb
ports: {game: 2302}
network: host
`)
	build := filepath.Join(data, "cache", "products", "dayz-stable", "1")
	writeFile(t, filepath.Join(build, "mpmissions", "dayzOffline.fb", "cfgweather.xml"), "weather")
	if err := os.Symlink("1", filepath.Join(data, "cache", "products", "dayz-stable", "current")); err != nil {
		t.Fatal(err)
	}
	cfg = filepath.Join(data, "config.yaml")
	writeFile(t, cfg, "paths:\n  data: "+data+"\n")
	return cfg, filepath.Join(data, "instances", "x"), missionRepo
}

func TestInstanceRender(t *testing.T) {
	cfg, root, repo := renderSetup(t)
	live := filepath.Join(root, "mpmissions", "empty.m")

	out, err := runCmd(t, "instance", "render", "x", "--config", cfg, "--dry-run")
	if err != nil {
		t.Fatalf("render --dry-run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "+ init.c (new)") || strings.Contains(out, "written:") {
		t.Errorf("dry-run output = %s", out)
	}
	if _, err := os.Stat(live); !os.IsNotExist(err) {
		t.Fatal("dry-run must not create the live mission")
	}

	out, err = runCmd(t, "instance", "render", "x", "--config", cfg)
	if err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	if !strings.Contains(out, "written: 3") { // init.c, types.xml, cfgweather.xml from the fallback
		t.Errorf("output = %s", out)
	}
	if b, _ := os.ReadFile(filepath.Join(live, "cfgweather.xml")); string(b) != "weather" {
		t.Errorf("fallback file = %q", b)
	}

	// The mission repo moves on; a plain render must not fetch, --update-pristine must.
	writeFile(t, filepath.Join(repo, "empty.m", "init.c"), "init2")
	commitAll2(t, repo)
	if out, err = runCmd(t, "instance", "render", "x", "--config", cfg); err != nil || !strings.Contains(out, "no changes") {
		t.Fatalf("plain render = %v\n%s", err, out)
	}
	if out, err = runCmd(t, "instance", "render", "x", "--config", cfg, "--update-pristine"); err != nil || !strings.Contains(out, "~ init.c (update)") {
		t.Fatalf("--update-pristine render = %v\n%s", err, out)
	}
	if b, _ := os.ReadFile(filepath.Join(live, "init.c")); string(b) != "init2" {
		t.Errorf("init.c = %q", b)
	}
}

func commitAll2(t *testing.T, dir string) {
	t.Helper()
	c := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qam", "y")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func TestInstanceRenderFailures(t *testing.T) {
	cfg, _, repo := renderSetup(t)
	if _, err := runCmd(t, "instance", "render", "nope", "--config", cfg); err == nil {
		t.Error("unknown instance must fail")
	}
	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "instance", "render", "x", "--config", cfg); err == nil {
		t.Error("unfetchable pristine must fail")
	}
}
