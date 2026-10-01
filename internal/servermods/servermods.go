// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package servermods packs dzo's own servermods (D39) from their source trees
// into PBOs: `make servermods`. A servermod repository (a submodule under
// servermods/<name>) holds one source directory per addon under src/<Addon>/,
// whose $PBOPREFIX$ file names the PBO prefix. The output is
// <out>/<name>/addons/<addon>.pbo plus a meta.cpp, in the layout of a local
// servermod (§C7), and is byte-for-byte reproducible: entries are sorted, all
// timestamps are zero and nothing is compressed.
//
// Scripts go into the PBO as plain text; config.cpp is not rapified. The
// server reads a text config.cpp from a PBO (verified on 1.29).
package servermods

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/woozymasta/pbo"
)

// PrefixFile is the file in an addon source directory that holds the PBO
// prefix (the DayZ Tools convention). It is not packed.
const PrefixFile = "$PBOPREFIX$"

// metaCPP is the meta.cpp of a locally built mod: no workshop id, no time.
const metaCPP = "protocol = 2;\npublishedid = 0;\ntimestamp = 0;\n"

// Result describes one packed addon.
type Result struct {
	Mod    string // servermod name, e.g. dzo-admin
	Addon  string // addon directory name, e.g. DZOAdmin
	PBO    string // path of the PBO
	Prefix string
	SHA256 string // of the PBO file
}

// Pack writes one addon source directory to a PBO and returns its prefix and
// file hash.
func Pack(ctx context.Context, srcDir, outPBO string) (prefix, sum string, err error) {
	raw, err := os.ReadFile(filepath.Join(srcDir, PrefixFile)) //nolint:gosec // build-time source tree
	if err != nil {
		return "", "", fmt.Errorf("servermods: %s: %w", srcDir, err)
	}
	prefix = strings.TrimSpace(string(raw))
	if prefix == "" {
		return "", "", fmt.Errorf("servermods: %s/%s is empty", srcDir, PrefixFile)
	}
	var inputs []pbo.Input
	err = filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(srcDir, p)
		if d.IsDir() || rel == PrefixFile {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", p)
		}
		file := p
		inputs = append(inputs, pbo.Input{
			Path: filepath.ToSlash(rel), ModTime: time.Unix(0, 0).UTC(),
			Open: func() (io.ReadCloser, error) { return os.Open(file) }, //nolint:gosec // build-time source tree
		})
		return nil
	})
	if err != nil {
		return "", "", fmt.Errorf("servermods: %w", err)
	}
	if len(inputs) == 0 {
		return "", "", fmt.Errorf("servermods: %s has no files", srcDir)
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Path < inputs[j].Path })
	if err := os.MkdirAll(filepath.Dir(outPBO), 0o750); err != nil {
		return "", "", err
	}
	if _, err := pbo.PackFile(ctx, outPBO, inputs, pbo.PackOptions{Headers: []pbo.HeaderPair{{Key: "prefix", Value: prefix}}}); err != nil {
		return "", "", fmt.Errorf("servermods: packing %s: %w", srcDir, err)
	}
	sum, err = fileSHA256(outPBO)
	return prefix, sum, err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // the PBO just written
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Build packs every servermod under srcRoot (servermods/*/src/*/) into
// outRoot/<mod>/addons/<addon>.pbo and writes each mod's meta.cpp. A servermod
// whose submodule is not checked out (no src/) is an error, so a build from a
// clone without --recurse-submodules fails loudly instead of shipping nothing.
func Build(ctx context.Context, srcRoot, outRoot string) ([]Result, error) {
	mods, err := filepath.Glob(filepath.Join(srcRoot, "*"))
	if err != nil {
		return nil, err
	}
	var out []Result
	for _, mod := range mods {
		if fi, err := os.Stat(mod); err != nil || !fi.IsDir() {
			continue
		}
		name := filepath.Base(mod)
		addons, _ := filepath.Glob(filepath.Join(mod, "src", "*"))
		if len(addons) == 0 {
			return nil, fmt.Errorf("servermods: %s has no src/<Addon>/ (submodule not checked out? git submodule update --init)", mod)
		}
		sort.Strings(addons)
		for _, a := range addons {
			if fi, err := os.Stat(a); err != nil || !fi.IsDir() {
				continue
			}
			addon := filepath.Base(a)
			dst := filepath.Join(outRoot, name, "addons", strings.ToLower(addon)+".pbo")
			prefix, sum, err := Pack(ctx, a, dst)
			if err != nil {
				return nil, err
			}
			out = append(out, Result{Mod: name, Addon: addon, PBO: dst, Prefix: prefix, SHA256: sum})
		}
		if err := os.WriteFile(filepath.Join(outRoot, name, "meta.cpp"), []byte(metaCPP), 0o600); err != nil {
			return nil, err
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("servermods: no servermods found under %s", srcRoot)
	}
	return out, nil
}
