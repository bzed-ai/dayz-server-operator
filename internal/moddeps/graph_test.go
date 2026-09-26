// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package moddeps

import "testing"

func TestValidateAllSatisfied(t *testing.T) {
	mods := []Mod{
		{ID: 0, Patches: []Patch{{Name: "DZ_Data"}, {Name: "DZ_Scripts"}}},
		{ID: 1, Patches: []Patch{{Name: "ModA", RequiredAddons: []string{"DZ_Data"}}}},
		{ID: 2, Patches: []Patch{{Name: "ModB", RequiredAddons: []string{"DZ_Data", "ModA"}}}},
	}
	result := Validate(mods)
	if !result.OK() {
		t.Errorf("Missing = %v, want none", result.Missing)
	}
}

func TestValidateMissingDependency(t *testing.T) {
	mods := []Mod{
		{ID: 0, Patches: []Patch{{Name: "DZ_Data"}}},
		{ID: 1, Patches: []Patch{{Name: "ModA", RequiredAddons: []string{"DZ_Data", "SomeMissingMod"}}}},
	}
	result := Validate(mods)
	if result.OK() {
		t.Fatal("expected a missing dependency")
	}
	if len(result.Missing) != 1 {
		t.Fatalf("Missing = %+v, want 1 entry", result.Missing)
	}
	m := result.Missing[0]
	if m.ModID != 1 || m.PatchName != "ModA" || m.RequiredBy != "SomeMissingMod" {
		t.Errorf("Missing[0] = %+v", m)
	}
	if m.String() == "" {
		t.Error("String() should not be empty")
	}
}

func TestValidateEmptyMods(t *testing.T) {
	if !Validate(nil).OK() {
		t.Error("Validate(nil) should be OK")
	}
}

func TestTopoSortOrdersByDependency(t *testing.T) {
	mods := []Mod{
		{ID: 2, Patches: []Patch{{Name: "ModB", RequiredAddons: []string{"ModA"}}}},
		{ID: 1, Patches: []Patch{{Name: "ModA", RequiredAddons: []string{"DZ_Data"}}}},
		{ID: 0, Patches: []Patch{{Name: "DZ_Data"}}},
	}
	sorted, err := TopoSort(mods)
	if err != nil {
		t.Fatalf("TopoSort: %v", err)
	}
	pos := map[uint64]int{}
	for i, m := range sorted {
		pos[m.ID] = i
	}
	if pos[0] >= pos[1] || pos[1] >= pos[2] {
		t.Errorf("order = %v, want 0 before 1 before 2", idsInOrder(sorted))
	}
}

func TestTopoSortDeterministicTieBreak(t *testing.T) {
	mods := []Mod{
		{ID: 5, Patches: []Patch{{Name: "M5"}}},
		{ID: 3, Patches: []Patch{{Name: "M3"}}},
		{ID: 1, Patches: []Patch{{Name: "M1"}}},
	}
	sorted, err := TopoSort(mods)
	if err != nil {
		t.Fatalf("TopoSort: %v", err)
	}
	if idsInOrder(sorted)[0] != 1 || idsInOrder(sorted)[1] != 3 || idsInOrder(sorted)[2] != 5 {
		t.Errorf("order = %v, want ascending ID for unconstrained mods", idsInOrder(sorted))
	}
}

func TestTopoSortDetectsCycle(t *testing.T) {
	mods := []Mod{
		{ID: 1, Patches: []Patch{{Name: "ModA", RequiredAddons: []string{"ModB"}}}},
		{ID: 2, Patches: []Patch{{Name: "ModB", RequiredAddons: []string{"ModA"}}}},
	}
	_, err := TopoSort(mods)
	if err == nil {
		t.Fatal("expected a cycle error")
	}
	var cycleErr ErrCycle
	if !asErrCycle(err, &cycleErr) {
		t.Fatalf("err = %v (%T), want ErrCycle", err, err)
	}
	if cycleErr.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestTopoSortIgnoresUnsatisfiedRequirement(t *testing.T) {
	// A requiredAddons entry nothing provides is not a graph edge (there
	// is no provider to order against); TopoSort must not error on it -
	// that is Validate's job.
	mods := []Mod{
		{ID: 1, Patches: []Patch{{Name: "ModA", RequiredAddons: []string{"NothingProvidesThis"}}}},
	}
	sorted, err := TopoSort(mods)
	if err != nil {
		t.Fatalf("TopoSort: %v", err)
	}
	if len(sorted) != 1 {
		t.Fatalf("sorted = %+v", sorted)
	}
}

func TestTopoSortEmpty(t *testing.T) {
	sorted, err := TopoSort(nil)
	if err != nil {
		t.Fatalf("TopoSort: %v", err)
	}
	if len(sorted) != 0 {
		t.Errorf("sorted = %+v, want empty", sorted)
	}
}

func TestTopoSortSelfReferenceIsNotAnEdge(t *testing.T) {
	mods := []Mod{
		{ID: 1, Patches: []Patch{{Name: "ModA", RequiredAddons: []string{"ModA"}}}},
	}
	if _, err := TopoSort(mods); err != nil {
		t.Fatalf("TopoSort: %v (a patch requiring its own name should not create a self-loop)", err)
	}
}

func idsInOrder(mods []Mod) []uint64 {
	ids := make([]uint64, len(mods))
	for i, m := range mods {
		ids[i] = m.ID
	}
	return ids
}

func asErrCycle(err error, target *ErrCycle) bool {
	e, ok := err.(ErrCycle)
	if ok {
		*target = e
	}
	return ok
}
