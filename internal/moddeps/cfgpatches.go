// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package moddeps

import "fmt"

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

// ExtractPatchesFromConfigCpp is a convenience wrapper: parses data as a
// config.cpp source and extracts its CfgPatches entries in one call.
func ExtractPatchesFromConfigCpp(data []byte) ([]Patch, error) {
	root, err := ParseConfig(data)
	if err != nil {
		return nil, err
	}
	return ExtractPatches(root)
}

// ExtractPatchesFromPBO finds name (conventionally "config.cpp") inside
// pbo and extracts its CfgPatches entries. It returns an error naming the
// entry if name is missing, or ErrCompressedEntry if it is a compressed
// (rapified config.bin-style) entry this package cannot decode (see the
// package doc comment).
func ExtractPatchesFromPBO(pbo *PBO, name string) ([]Patch, error) {
	entry, ok := pbo.Find(name)
	if !ok {
		return nil, fmt.Errorf("moddeps: PBO has no entry named %q", name)
	}
	data, err := pbo.ReadEntry(entry)
	if err != nil {
		return nil, err
	}
	return ExtractPatchesFromConfigCpp(data)
}
