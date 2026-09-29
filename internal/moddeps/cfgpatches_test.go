// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package moddeps

import "testing"

func TestExtractPatchesFromConfig(t *testing.T) {
	src := `
class CfgPatches {
	class ModA { requiredAddons[] = {"DZ_Data"}; };
	class ModB { requiredAddons[] = {"DZ_Data", "ModA"}; };
};
`
	patches, err := ExtractPatchesFromConfig([]byte(src))
	if err != nil {
		t.Fatalf("ExtractPatchesFromConfig: %v", err)
	}
	if len(patches) != 2 {
		t.Fatalf("patches = %d, want 2", len(patches))
	}
	byName := map[string]Patch{}
	for _, p := range patches {
		byName[p.Name] = p
	}
	if !equalStrings(byName["ModB"].RequiredAddons, []string{"DZ_Data", "ModA"}) {
		t.Errorf("ModB.RequiredAddons = %v", byName["ModB"].RequiredAddons)
	}
}

func TestExtractPatchesNoCfgPatches(t *testing.T) {
	patches, err := ExtractPatchesFromConfig([]byte(`class Something { x = 1; };`))
	if err != nil {
		t.Fatalf("ExtractPatchesFromConfig: %v", err)
	}
	if patches != nil {
		t.Errorf("patches = %v, want nil", patches)
	}
}

func TestExtractPatchesNoRequiredAddons(t *testing.T) {
	patches, err := ExtractPatchesFromConfig([]byte(`class CfgPatches { class M { units[] = {}; }; };`))
	if err != nil {
		t.Fatalf("ExtractPatchesFromConfig: %v", err)
	}
	if len(patches) != 1 || patches[0].Name != "M" || patches[0].RequiredAddons != nil {
		t.Errorf("patches = %+v", patches)
	}
}

func TestExtractPatchesScalarRequiredAddonsErrors(t *testing.T) {
	_, err := ExtractPatchesFromConfig([]byte(`class CfgPatches { class M { requiredAddons = "not an array"; }; };`))
	if err == nil {
		t.Fatal("expected an error for a non-array requiredAddons")
	}
}

func TestExtractPatchesInvalidConfigErrors(t *testing.T) {
	if _, err := ExtractPatchesFromConfig([]byte(`class {`)); err == nil {
		t.Fatal("expected a parse error to propagate")
	}
}

func TestExtractPatchesFromPBO(t *testing.T) {
	data := buildPBO(t, false, []pboEntry{
		{name: "config.cpp", data: []byte(`class CfgPatches { class M { requiredAddons[] = {"A"}; }; };`)},
	})
	pbo, err := OpenPBO(data)
	if err != nil {
		t.Fatalf("OpenPBO: %v", err)
	}
	patches, err := ExtractPatchesFromPBO(pbo)
	if err != nil {
		t.Fatalf("ExtractPatchesFromPBO: %v", err)
	}
	if len(patches) != 1 || patches[0].Name != "M" {
		t.Errorf("patches = %+v", patches)
	}
}

func TestExtractPatchesFromPBOMissingEntry(t *testing.T) {
	data := buildPBO(t, false, []pboEntry{{name: "other.txt", data: []byte("x")}})
	pbo, err := OpenPBO(data)
	if err != nil {
		t.Fatalf("OpenPBO: %v", err)
	}
	if _, err := ExtractPatchesFromPBO(pbo); err == nil {
		t.Fatal("expected an error for a missing config.cpp entry")
	}
}
