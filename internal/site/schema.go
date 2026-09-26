// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package site models the site config repo (§C3): shared mod integrations
// and overlays plus per-instance configuration, checked out from a
// configurable git remote (D16). It holds the schema types for
// instance.yaml, integration.yaml and overlay manifests, and the git
// plumbing to clone/pull/commit/push the repo.
package site

import "fmt"

// Network selects how an instance's ports reach the host (D6), mirroring
// internal/quadlet.Network so callers can convert directly.
type Network string

const (
	NetworkHost    Network = "host"
	NetworkPublish Network = "publish"
)

// UpdatePolicy controls whether detected mod updates apply automatically.
type UpdatePolicy string

const (
	PolicyAuto   UpdatePolicy = "auto"
	PolicyNotify UpdatePolicy = "notify"
	PolicyManual UpdatePolicy = "manual"
)

// DriftPolicy controls what happens when a managed mission file has
// changed on disk since dzo last wrote it (Q11).
type DriftPolicy string

const (
	DriftWarnBackupOverwrite DriftPolicy = "warn-backup-overwrite"
)

// Ports is an instance's game/RCon/query port set.
type Ports struct {
	Game  int `yaml:"game"`
	RCon  int `yaml:"rcon"`
	Query int `yaml:"query"`
}

// Params are the DayZServer launch parameters not otherwise derived.
type Params struct {
	// CPUCount is either a positive integer as a string, or "auto".
	CPUCount string   `yaml:"cpuCount,omitempty"`
	LimitFPS *int     `yaml:"limitFPS,omitempty"`
	Extra    []string `yaml:"extra,omitempty"`
}

// MissionSource describes where an instance's pristine mission comes from:
// either a direct git repo, or a named preset from
// site/integrations/maps/<name>.yaml.
type MissionSource struct {
	Git    string `yaml:"git,omitempty"`
	Preset string `yaml:"preset,omitempty"`
	Ref    string `yaml:"ref,omitempty"`
	Path   string `yaml:"path,omitempty"`
}

// Validate checks that exactly one of Git/Preset is set, and that a direct
// git source has a ref and path.
func (m MissionSource) Validate() error {
	if (m.Git == "") == (m.Preset == "") {
		return fmt.Errorf("mission_source: exactly one of git or preset must be set")
	}
	if m.Git != "" {
		if m.Ref == "" {
			return fmt.Errorf("mission_source: ref is required with git")
		}
		if m.Path == "" {
			return fmt.Errorf("mission_source: path is required with git")
		}
	}
	return nil
}

// MissionConfig is the per-instance override of the render pipeline's file
// classification (§C6).
type MissionConfig struct {
	// Unmanaged lists glob patterns that stay foreign even if the mission
	// repo ships matching paths.
	Unmanaged []string    `yaml:"unmanaged,omitempty"`
	Drift     DriftPolicy `yaml:"drift,omitempty"`
}

// Announce is a restart/update announcement schedule (A4/legacy dayz_restart).
type Announce struct {
	Minutes int    `yaml:"minutes"`
	Lock    int    `yaml:"lock"`
	Delay   int    `yaml:"delay"`
	Text    string `yaml:"text,omitempty"`
}

// UpdatesConfig controls the mod update pipeline for one instance (§C7).
type UpdatesConfig struct {
	Policy                    UpdatePolicy `yaml:"policy,omitempty"`
	CheckInterval             Duration     `yaml:"check_interval,omitempty"`
	Window                    []string     `yaml:"window,omitempty"`
	QuietHours                []string     `yaml:"quiet_hours,omitempty"`
	MinRestartInterval        Duration     `yaml:"min_restart_interval,omitempty"`
	BatchDelay                Duration     `yaml:"batch_delay,omitempty"`
	ApplyWithScheduledRestart bool         `yaml:"apply_with_scheduled_restart,omitempty"`
	MaxDelay                  Duration     `yaml:"max_delay,omitempty"`
	RestartAnnounce           Announce     `yaml:"restart_announce,omitempty"`
}

// RestartsConfig controls scheduled maintenance restarts (FR-16).
type RestartsConfig struct {
	Schedule []string `yaml:"schedule,omitempty"`
	Announce Announce `yaml:"announce,omitempty"`
}

// HealthConfig drives HealthStartupRetries/HealthInterval (§C5/C9).
type HealthConfig struct {
	StartupTimeout Duration `yaml:"startup_timeout,omitempty"`
	Interval       Duration `yaml:"interval,omitempty"`
	Retries        int      `yaml:"retries,omitempty"`
}

// RestartLimit is the crash/render-loop brake (F3).
type RestartLimit struct {
	Burst    int       `yaml:"burst,omitempty"`
	Interval Duration  `yaml:"interval,omitempty"`
	Cooldown *Duration `yaml:"cooldown,omitempty"`
}

// NotifyConfig selects which Discord webhooks (by name, from
// /etc/dzo/config.yaml's notify.discord) an instance uses. Discord being
// nil means "use the default webhook"; a non-nil empty slice means "send
// nothing" (Q14).
type NotifyConfig struct {
	Discord *[]string `yaml:"discord,omitempty"`
}

// ContainerConfig covers the generic per-instance container escape hatches
// (D9): extra env, extra mounts, resource limits.
type ContainerConfig struct {
	Env     map[string]string `yaml:"env,omitempty"`
	Mounts  []string          `yaml:"mounts,omitempty"`
	LogzDir string            `yaml:"logz_dir,omitempty"`
	CPUs    *string           `yaml:"cpus,omitempty"`
	Memory  *string           `yaml:"memory,omitempty"`
}

// HooksConfig lists the hook scripts for each hook point (§C11).
type HooksConfig struct {
	PreStart     []string `yaml:"pre_start,omitempty"`
	PostStop     []string `yaml:"post_stop,omitempty"`
	PreUpdate    []string `yaml:"pre_update,omitempty"`
	PostUpdate   []string `yaml:"post_update,omitempty"`
	PostDownload []string `yaml:"post_download,omitempty"`
	PostMerge    []string `yaml:"post_merge,omitempty"`
	PostRender   []string `yaml:"post_render,omitempty"`
	PostBackup   []string `yaml:"post_backup,omitempty"`
}

// ModRef is one entry in an instance's mod list. List order is the dzo
// merge/CE precedence order (later wins), independent of any in-game
// -mod= load order.
type ModRef struct {
	ID     uint64 `yaml:"id"`
	Server bool   `yaml:"server,omitempty"`
}

// Instance is one instances/<name>/instance.yaml (§C3 example).
type Instance struct {
	Name            string          `yaml:"name"`
	Product         string          `yaml:"product"`
	Map             string          `yaml:"map"`
	MissionSource   MissionSource   `yaml:"mission_source"`
	FallbackMission string          `yaml:"fallback_mission,omitempty"`
	Mission         MissionConfig   `yaml:"mission,omitempty"`
	Ports           Ports           `yaml:"ports"`
	Network         Network         `yaml:"network"`
	Params          Params          `yaml:"params,omitempty"`
	Mods            []ModRef        `yaml:"mods,omitempty"`
	Overlays        []string        `yaml:"overlays,omitempty"`
	Updates         UpdatesConfig   `yaml:"updates,omitempty"`
	Restarts        RestartsConfig  `yaml:"restarts,omitempty"`
	Health          HealthConfig    `yaml:"health,omitempty"`
	RestartLimit    RestartLimit    `yaml:"restart_limit,omitempty"`
	Notify          NotifyConfig    `yaml:"notify,omitempty"`
	Container       ContainerConfig `yaml:"container,omitempty"`
	Hooks           HooksConfig     `yaml:"hooks,omitempty"`
}

// Validate checks the invariants the rest of dzo relies on. It does not
// check cross-references (product exists in config.yaml, overlay/mod
// files exist) - that needs the loaded site tree and belongs to a later
// "site validate" pass.
func (i Instance) Validate() error {
	var errs []string
	if i.Name == "" {
		errs = append(errs, "name is required")
	}
	if i.Product == "" {
		errs = append(errs, "product is required")
	}
	if i.Map == "" {
		errs = append(errs, "map is required")
	}
	if err := i.MissionSource.Validate(); err != nil {
		errs = append(errs, err.Error())
	}
	if i.Network != NetworkHost && i.Network != NetworkPublish {
		errs = append(errs, fmt.Sprintf("network must be %q or %q, got %q", NetworkHost, NetworkPublish, i.Network))
	}
	if i.Ports.Game == 0 {
		errs = append(errs, "ports.game is required")
	}
	seen := map[uint64]bool{}
	for _, m := range i.Mods {
		if m.ID == 0 {
			errs = append(errs, "mods: entry with id 0")
			continue
		}
		if seen[m.ID] {
			errs = append(errs, fmt.Sprintf("mods: duplicate id %d", m.ID))
		}
		seen[m.ID] = true
	}
	switch i.Updates.Policy {
	case "", PolicyAuto, PolicyNotify, PolicyManual:
	default:
		errs = append(errs, fmt.Sprintf("updates.policy: invalid value %q", i.Updates.Policy))
	}
	if len(errs) > 0 {
		return fmt.Errorf("instance %q: %s", i.Name, joinErrs(errs))
	}
	return nil
}

func joinErrs(errs []string) string {
	out := errs[0]
	for _, e := range errs[1:] {
		out += "; " + e
	}
	return out
}

// FileSource is one entry in an Integration's Files map: where to fetch a
// mod integration file from.
type FileSource struct {
	Source string   `yaml:"source"` // local | url | mod
	Path   string   `yaml:"path,omitempty"`
	URL    string   `yaml:"url,omitempty"`
	SHA256 string   `yaml:"sha256,omitempty"`
	Maps   []string `yaml:"maps,omitempty"` // restrict this variant to specific maps
}

// Validate checks that Source is a known value and its required field is set.
func (f FileSource) Validate(key string) error {
	switch f.Source {
	case "local":
		if f.Path == "" {
			return fmt.Errorf("files.%s: source local requires path", key)
		}
	case "url":
		if f.URL == "" {
			return fmt.Errorf("files.%s: source url requires url", key)
		}
	case "mod":
		if f.Path == "" {
			return fmt.Errorf("files.%s: source mod requires path", key)
		}
	default:
		return fmt.Errorf("files.%s: unknown source %q (want local, url or mod)", key, f.Source)
	}
	return nil
}

// IntegrationHooks lists post-merge scripts for one mod integration.
type IntegrationHooks struct {
	PostMerge []string `yaml:"post_merge,omitempty"`
}

// Integration is one integrations/mods/<modid>/integration.yaml (§C3
// example): replaces the legacy xml.env/map.env.
type Integration struct {
	Mod       uint64                `yaml:"mod"`
	Name      string                `yaml:"name"`
	Files     map[string]FileSource `yaml:"files,omitempty"`
	Normalize []string              `yaml:"normalize,omitempty"`
	Hooks     IntegrationHooks      `yaml:"hooks,omitempty"`
	Aliases   []uint64              `yaml:"aliases,omitempty"`
}

// Validate checks the invariants Integration relies on.
func (i Integration) Validate() error {
	var errs []string
	if i.Mod == 0 {
		errs = append(errs, "mod is required")
	}
	if i.Name == "" {
		errs = append(errs, "name is required")
	}
	for key, f := range i.Files {
		if err := f.Validate(key); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("integration %d: %s", i.Mod, joinErrs(errs))
	}
	return nil
}

// OverlayManifest is a site/overlays/<name>/overlay.yaml: it declares which
// of the overlay's own files should be referenced from generated
// cfggameplay.json list keys (§C6 pipeline step 4).
type OverlayManifest struct {
	ObjectSpawners   []string `yaml:"object_spawners,omitempty"`
	SpawnGearPresets []string `yaml:"spawn_gear_presets,omitempty"`
	RestrictedAreas  []string `yaml:"restricted_areas,omitempty"`
}

// MapPreset is one integrations/maps/<name>.yaml: a reusable mission
// source, referenced from Instance.MissionSource.Preset.
type MapPreset struct {
	Git  string `yaml:"git"`
	Ref  string `yaml:"ref"`
	Path string `yaml:"path"`
}
