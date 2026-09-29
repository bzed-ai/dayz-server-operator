// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package moddeps

import (
	"errors"
	"fmt"
)

// Patch is one CfgPatches child class: the name other addons'
// requiredAddons entries refer to, and what it itself requires.
type Patch struct {
	Name           string
	RequiredAddons []string
}

// ExtractPatches reads every child class of root's top-level CfgPatches
// class. A config with no CfgPatches class returns (nil, nil): that is not
// itself an error here (some configs - e.g. a mission config.cpp - never
// have one), though a real mod PBO missing CfgPatches would be a problem
// for the caller to judge.
func ExtractPatches(root *Class) ([]Patch, error) {
	patches, ok := root.Classes["CfgPatches"]
	if !ok {
		return nil, nil
	}
	result := make([]Patch, 0, len(patches.Classes))
	for name, cls := range patches.Classes {
		p := Patch{Name: name}
		if req, ok := cls.Properties["requiredAddons"]; ok {
			if !req.IsArray {
				return nil, fmt.Errorf("moddeps: CfgPatches.%s.requiredAddons is not an array", name)
			}
			p.RequiredAddons = req.Array
		}
		result = append(result, p)
	}
	return result, nil
}

// ExtractPatchesFromConfig extracts CfgPatches from data, which is either
// a rapified config.bin (detected by its magic) or config.cpp source.
func ExtractPatchesFromConfig(data []byte) ([]Patch, error) {
	parse := ParseConfig
	if IsRapified(data) {
		parse = ParseRapified
	}
	root, err := parse(data)
	if err != nil {
		return nil, err
	}
	return ExtractPatches(root)
}

// ExtractPatchesFromPBO reads the PBO's config.bin (or, failing that,
// config.cpp) and extracts its CfgPatches entries; compressed entries are
// decompressed transparently.
func ExtractPatchesFromPBO(pbo *PBO) ([]Patch, error) {
	entry, ok := pbo.Find("config.bin")
	if !ok {
		if entry, ok = pbo.Find("config.cpp"); !ok {
			return nil, errors.New("moddeps: PBO has no config.bin or config.cpp entry")
		}
	}
	data, err := pbo.ReadEntry(entry)
	if err != nil {
		return nil, err
	}
	return ExtractPatchesFromConfig(data)
}
