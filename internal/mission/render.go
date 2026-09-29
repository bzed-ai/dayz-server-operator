// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package mission

import (
	"fmt"
	"os"
	"path/filepath"
)

// BaseFiles are the mission files every instance needs, and that a map's
// mission repo may lack: they are filled in from the fallback mission (§C6,
// legacy step 5). It is the legacy refresh list.
var BaseFiles = []string{
	"env/zombie_territories.xml", "cfgrandompresets.xml", "mapgrouppos.xml", "mapgroupproto.xml",
	"cfgeconomycore.xml", "cfgenvironment.xml", "cfgeventgroups.xml", "cfgeventspawns.xml",
	"cfggameplay.json", "cfgundergroundtriggers.json", "cfgeffectarea.json", "cfgweather.xml", "init.c",
}

// RenderInput describes one instance's mission render.
type RenderInput struct {
	PristineDir    string
	FallbackDir    string // optional: mission dir to take missing BaseFiles from
	LiveDir        string
	ManifestPath   string
	FileHistoryDir string
	Unmanaged      []string
	DryRun         bool
	Options        Options
}

// Render builds the staging tree from the pristine mission (plus the
// fallback fill), classifies it against the live mission and, unless DryRun,
// applies it in place and saves the manifest. The first render into an empty
// live dir is the one-time initialisation; later renders only touch files in
// the manifest or new in pristine. Nothing is written when the plan fails.
func Render(in RenderInput) (Plan, Report, error) {
	staging, err := os.MkdirTemp("", "dzo-staging-")
	if err != nil {
		return Plan{}, Report{}, fmt.Errorf("mission: %w", err)
	}
	defer os.RemoveAll(staging) //nolint:errcheck // scratch dir

	if err := CopyPristine(in.PristineDir, staging); err != nil {
		return Plan{}, Report{}, err
	}
	if in.FallbackDir != "" {
		if err := fillFromFallback(in.FallbackDir, staging); err != nil {
			return Plan{}, Report{}, err
		}
	}
	manifest, err := LoadManifest(in.ManifestPath)
	if err != nil {
		return Plan{}, Report{}, err
	}
	plan, err := Classify(staging, in.LiveDir, manifest, in.Unmanaged)
	if err != nil || in.DryRun {
		return plan, Report{}, err
	}
	if err := os.MkdirAll(filepath.Dir(in.ManifestPath), 0o750); err != nil {
		return plan, Report{}, fmt.Errorf("mission: %w", err)
	}
	report, err := Apply(plan, staging, in.LiveDir, in.FileHistoryDir, manifest, in.Options)
	if err != nil {
		return plan, report, err
	}
	return plan, report, manifest.Save(in.ManifestPath)
}

func fillFromFallback(fallbackDir, staging string) error {
	if fi, err := os.Stat(fallbackDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("mission: fallback mission %s not found", fallbackDir)
	}
	for _, rel := range BaseFiles {
		if _, err := os.Stat(filepath.Join(staging, rel)); err == nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(fallbackDir, rel)) //nolint:gosec // rel is from the fixed BaseFiles list
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("mission: read fallback %s: %w", rel, err)
		}
		if err := WriteFile(staging, rel, data); err != nil {
			return err
		}
	}
	return nil
}
