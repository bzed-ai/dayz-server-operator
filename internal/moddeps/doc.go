// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package moddeps validates mod load-order dependencies from CfgPatches
// requiredAddons (§C7, FR-06a): DayZ does not give the -mod= list a
// reliable ordering meaning, so what actually decides whether a mod's
// scripts/configs load is each addon's CfgPatches requiredAddons entries,
// which every active mod (and the product build itself) must collectively
// satisfy.
//
// Implemented, and testable without a real mod or Steam access:
//
//   - Config: a parser for the plain-text "raw" Arma/Enfusion config
//     grammar (class blocks, scalar and array assignments, comments) -
//     the same grammar config.cpp source files use.
//   - PBO: a reader for Bohemia's PBO archive container, enough to find
//     and extract one named entry's bytes, LZSS ("Cprs") entries included.
//   - Rapified: a decoder for binary config.bin files (rap.go), producing
//     the same tree as Config.
//   - CfgPatches: extracts each requiredAddons[] list from a parsed
//     CfgPatches class tree.
//   - Graph: validates that every required addon name is provided by
//     some active mod or the product build, and topologically sorts the
//     result for mods_order: dependencies.
//
// The rapified and LZSS decoders follow published format descriptions and
// are verified against synthetic fixtures only, not a real mod PBO.
package moddeps
