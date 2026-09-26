// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigCpp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.cpp")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config.cpp: %v", err)
	}
	return path
}

func TestModCfgPatchesCmd(t *testing.T) {
	path := writeConfigCpp(t, `class CfgPatches { class M { requiredAddons[] = {"A", "B"}; }; };`)
	out, err := runCmd(t, "mod", "cfgpatches", path)
	if err != nil {
		t.Fatalf("mod cfgpatches: %v\n%s", err, out)
	}
	if !strings.Contains(out, "M requires [A B]") {
		t.Errorf("output = %q", out)
	}
}

func TestModCfgPatchesCmdMissingFile(t *testing.T) {
	if _, err := runCmd(t, "mod", "cfgpatches", "/nonexistent/config.cpp"); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestModCfgPatchesCmdInvalidConfig(t *testing.T) {
	path := writeConfigCpp(t, `class {`)
	if _, err := runCmd(t, "mod", "cfgpatches", path); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestModDepsCmdAllSatisfied(t *testing.T) {
	a := writeConfigCpp(t, `class CfgPatches { class ModA { requiredAddons[] = {"DZ_Data"}; }; };`)
	out, err := runCmd(t, "mod", "deps", a, "--provides", "DZ_Data,DZ_Scripts")
	if err != nil {
		t.Fatalf("mod deps: %v\n%s", err, out)
	}
	if !strings.Contains(out, "all requiredAddons entries are satisfied") {
		t.Errorf("output = %q", out)
	}
}

func TestModDepsCmdMissingDependency(t *testing.T) {
	a := writeConfigCpp(t, `class CfgPatches { class ModA { requiredAddons[] = {"SomethingMissing"}; }; };`)
	out, err := runCmd(t, "mod", "deps", a)
	if err == nil {
		t.Fatal("expected an error for a missing dependency")
	}
	if !strings.Contains(out, "SomethingMissing") {
		t.Errorf("output = %q", out)
	}
}

func TestModDepsCmdShowsOrder(t *testing.T) {
	a := writeConfigCpp(t, `class CfgPatches { class ModA { requiredAddons[] = {"DZ_Data"}; }; };`)
	b := writeConfigCpp(t, `class CfgPatches { class ModB { requiredAddons[] = {"ModA"}; }; };`)
	out, err := runCmd(t, "mod", "deps", b, a, "--provides", "DZ_Data", "--order")
	if err != nil {
		t.Fatalf("mod deps: %v\n%s", err, out)
	}
	if !strings.Contains(out, "load order") {
		t.Errorf("output = %q", out)
	}
}

func TestModDepsCmdMissingFile(t *testing.T) {
	if _, err := runCmd(t, "mod", "deps", "/nonexistent/config.cpp"); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
