// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package moddeps

import (
	"fmt"
	"sort"
)

// Mod is one active mod's contribution to the dependency graph: every
// CfgPatches class name it provides, and what each of those requires.
// A mod is usually one Patch per PBO, but some ship several.
type Mod struct {
	ID      uint64 // the mod's workshop id, 0 for the product build itself
	Patches []Patch
}

// MissingDependency is one requiredAddons entry that no active mod or the
// product build provides.
type MissingDependency struct {
	ModID      uint64
	PatchName  string
	RequiredBy string // the requiredAddons value that is missing
}

func (m MissingDependency) String() string {
	return fmt.Sprintf("mod %d: CfgPatches.%s requires %q, which nothing provides", m.ModID, m.PatchName, m.RequiredBy)
}

// ValidateResult is the outcome of Validate.
type ValidateResult struct {
	Missing []MissingDependency
}

// OK reports whether every requiredAddons entry across mods is satisfied.
func (r ValidateResult) OK() bool { return len(r.Missing) == 0 }

// Validate checks that every requiredAddons entry across mods' patches is
// provided by some patch name across all of mods (FR-06a: "every
// requiredAddons entry must be provided by the product build or an active
// mod"). Callers include the product build itself as a Mod with ID 0 so
// its own CfgPatches classes count as provided.
func Validate(mods []Mod) ValidateResult {
	provided := map[string]bool{}
	for _, m := range mods {
		for _, p := range m.Patches {
			provided[p.Name] = true
		}
	}

	var missing []MissingDependency
	for _, m := range mods {
		for _, p := range m.Patches {
			for _, req := range p.RequiredAddons {
				if !provided[req] {
					missing = append(missing, MissingDependency{ModID: m.ID, PatchName: p.Name, RequiredBy: req})
				}
			}
		}
	}
	return ValidateResult{Missing: missing}
}

// ErrCycle is returned by TopoSort when the dependency graph contains a
// cycle (a configuration that could never load in-game either).
type ErrCycle struct {
	Cycle []string
}

func (e ErrCycle) Error() string {
	return fmt.Sprintf("moddeps: dependency cycle: %v", e.Cycle)
}

// TopoSort orders mods so that every mod appears after every other mod
// providing one of its requiredAddons entries (mods_order: dependencies,
// §C7). Ties (mods with no ordering constraint between them) are broken
// by ID, ascending, so the result is deterministic (A8#1). A
// requiredAddons entry nothing provides is simply not a constraint here -
// call Validate separately to catch that; TopoSort only reports cycles.
func TopoSort(mods []Mod) ([]Mod, error) {
	providerOf := map[string]uint64{}
	byID := map[uint64]Mod{}
	for _, m := range mods {
		byID[m.ID] = m
		for _, p := range m.Patches {
			providerOf[p.Name] = m.ID
		}
	}

	// edges[a] = mods that must come before a (a depends on them).
	edges := map[uint64][]uint64{}
	for _, m := range mods {
		seen := map[uint64]bool{}
		for _, p := range m.Patches {
			for _, req := range p.RequiredAddons {
				if provider, ok := providerOf[req]; ok && provider != m.ID && !seen[provider] {
					edges[m.ID] = append(edges[m.ID], provider)
					seen[provider] = true
				}
			}
		}
	}

	ids := make([]uint64, 0, len(mods))
	for _, m := range mods {
		ids = append(ids, m.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := map[uint64]int{}
	var order []uint64
	var path []uint64

	var visit func(id uint64) error
	visit = func(id uint64) error {
		switch color[id] {
		case black:
			return nil
		case grey:
			return ErrCycle{Cycle: idsToStrings(append(path, id))}
		}
		color[id] = grey
		path = append(path, id)

		deps := append([]uint64{}, edges[id]...)
		sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}

		path = path[:len(path)-1]
		color[id] = black
		order = append(order, id)
		return nil
	}

	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}

	sorted := make([]Mod, 0, len(order))
	for _, id := range order {
		sorted = append(sorted, byID[id])
	}
	return sorted, nil
}

func idsToStrings(ids []uint64) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = fmt.Sprintf("%d", id)
	}
	return out
}
