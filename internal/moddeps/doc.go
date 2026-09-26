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
//     and extract one named entry's bytes. Only uncompressed entries are
//     supported; a compressed (LZSS, "Cprs") entry returns a clear error
//     rather than a guessed-at decompression, since there is no real
//     compressed-PBO fixture available here to verify against.
//   - CfgPatches: extracts each requiredAddons[] list from a parsed
//     CfgPatches class tree.
//   - Graph: validates that every required addon name is provided by
//     some active mod or the product build, and topologically sorts the
//     result for mods_order: dependencies.
//
// The gap this package does NOT close: real DayZ mod PBOs almost always
// ship a rapified config.bin (Bohemia's compiled binary config format),
// not a plain-text config.cpp. Decoding that binary format needs a
// verified real-mod fixture to check the decoder against, which this
// environment does not have (no Steam/workshop access); Config here only
// parses the text form. Until config.bin decoding exists, this package's
// PBO+Config path only works end to end for a mod that happens to ship an
// unrapified config.cpp (rare in practice) or when fed a config.cpp
// extracted by other means.
package moddeps
