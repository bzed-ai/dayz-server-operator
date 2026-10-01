// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WatchRule is a class watch rule (§C16 path 1): entities of these classes
// appear on the map layer. Cluster is a hint for the web map; OnlyIf is
// accepted in the site config but not supported by the mod yet, and
// Validate rejects it so that it is never silently ignored.
type WatchRule struct {
	Layer   string   `yaml:"layer" json:"layer"`
	Classes []string `yaml:"classes" json:"classes"`
	Icon    string   `yaml:"icon,omitempty" json:"icon"`
	Label   string   `yaml:"label,omitempty" json:"label"`
	Max     int      `yaml:"max,omitempty" json:"max"`
	Cluster int      `yaml:"cluster,omitempty" json:"-"`
	OnlyIf  string   `yaml:"only_if,omitempty" json:"-"`
}

// Validate checks one rule.
func (w WatchRule) Validate() error {
	if w.Layer == "" || len(w.Classes) == 0 {
		return fmt.Errorf("watch rule needs layer and classes")
	}
	if w.OnlyIf != "" {
		return fmt.Errorf("watch rule %q: only_if is not supported yet", w.Layer)
	}
	return nil
}

// ModConfig is $profile:dzo-admin/config.json (DZOAdminConfig in the mod).
type ModConfig struct {
	Version      int         `json:"version"`
	Endpoint     string      `json:"endpoint"`
	Token        string      `json:"token"`
	SyncMS       int         `json:"sync_ms"`
	PlayersS     int         `json:"players_s"`
	VehiclesS    int         `json:"vehicles_s"`
	MarkersS     int         `json:"markers_s"`
	EventsS      int         `json:"events_s"`
	AllowSpawn   bool        `json:"allow_spawn"`
	AllowClasses []string    `json:"allow_classes"`
	DenyClasses  []string    `json:"deny_classes"`
	Watch        []WatchRule `json:"watch"`
}

// WithDefaults fills the intervals the instance config left at zero.
func (c *ModConfig) WithDefaults() {
	c.Version = 1
	for _, d := range []struct {
		v *int
		d int
	}{{&c.SyncMS, 1000}, {&c.PlayersS, 5}, {&c.VehiclesS, 60}, {&c.MarkersS, 10}, {&c.EventsS, 30}} {
		if *d.v == 0 {
			*d.v = d.d
		}
	}
	if c.AllowClasses == nil {
		c.AllowClasses = []string{}
	}
	if c.DenyClasses == nil {
		c.DenyClasses = []string{}
	}
	if c.Watch == nil {
		c.Watch = []WatchRule{}
	}
}

// NewToken returns a fresh random instance token.
func NewToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func writeFile0600(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// WriteModConfig writes the mod's config.json under profilesDir and the
// token file under tokenDir. The token is rotated on every call (§C13: "rotated on every render"), so the mod
// and the registry always agree on the newest one: the registry re-reads the
// token file when it changes.
func WriteModConfig(profilesDir, tokenDir, instance string, cfg ModConfig) error {
	cfg.WithDefaults()
	tok, err := NewToken()
	if err != nil {
		return err
	}
	cfg.Token = tok
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile0600(filepath.Join(tokenDir, instance+".token"), []byte(tok+"\n")); err != nil {
		return err
	}
	return writeFile0600(filepath.Join(profilesDir, "dzo-admin", "config.json"), append(b, '\n'))
}

// MarkerDir is where the file drop of an instance lives.
func MarkerDir(profilesDir string) string { return filepath.Join(profilesDir, "dzo-admin", "markers") }
