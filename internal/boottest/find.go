// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package boottest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/woozymasta/pbo"
)

// SteamServer is a DayZ Server installed by the Steam client.
type SteamServer struct {
	Dir     string // steamapps/common/<installdir>
	Build   string // the build id of the manifest
	Library string // the Steam library that holds it
}

var (
	vdfPath    = regexp.MustCompile(`"path"\s+"([^"]+)"`)
	acfInstall = regexp.MustCompile(`"installdir"\s+"([^"]*)"`)
	acfBuild   = regexp.MustCompile(`"buildid"\s+"([^"]*)"`)
	acfState   = regexp.MustCompile(`"StateFlags"\s+"([^"]*)"`)
)

// SteamRoots are the places a Steam client keeps its libraries: $STEAM_ROOT,
// then the native, Debian, Flatpak and Snap locations.
func SteamRoots(home string) []string {
	var roots []string
	if r := os.Getenv("STEAM_ROOT"); r != "" {
		roots = append(roots, r)
	}
	return append(roots,
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".steam", "debian-installation"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(home, "snap", "steam", "common", ".local", "share", "Steam"),
	)
}

// Libraries lists the Steam libraries of the given roots, without duplicates.
func Libraries(roots []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		if real, err := filepath.EvalSymlinks(p); err == nil {
			p = real
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, r := range roots {
		if _, err := os.Stat(filepath.Join(r, "steamapps")); err != nil {
			continue
		}
		add(r)
		b, err := os.ReadFile(filepath.Join(r, "steamapps", "libraryfolders.vdf")) //nolint:gosec // Steam's own metadata
		if err != nil {
			continue
		}
		for _, m := range vdfPath.FindAllStringSubmatch(string(b), -1) {
			add(m[1])
		}
	}
	return out
}

// FindSteamServer finds the "DayZ Server" tool of an app id (223350, stable;
// 1042420, experimental) by asking Steam's own manifests, not by guessing
// paths. dzo never installs it and never writes into it.
func FindSteamServer(libs []string, appID uint32) (SteamServer, error) {
	for _, lib := range libs {
		b, err := os.ReadFile(filepath.Join(lib, "steamapps", fmt.Sprintf("appmanifest_%d.acf", appID))) //nolint:gosec // Steam's own metadata
		if err != nil {
			continue
		}
		in := acfInstall.FindStringSubmatch(string(b))
		if in == nil {
			continue
		}
		if st := acfState.FindStringSubmatch(string(b)); st != nil && st[1] != "4" {
			return SteamServer{}, fmt.Errorf("the DayZ Server (app %d) is still being installed or updated by Steam (StateFlags %s); wait until it has finished", appID, st[1])
		}
		s := SteamServer{Dir: filepath.Join(lib, "steamapps", "common", in[1]), Library: lib}
		if bi := acfBuild.FindStringSubmatch(string(b)); bi != nil {
			s.Build = bi[1]
		}
		return s, nil
	}
	return SteamServer{}, fmt.Errorf("the DayZ Server (Steam app %d) is not installed. In Steam, open the Library, enable the Tools filter and install \"DayZ Server\" (your account must own DayZ), or run: steam steam://install/%d; dzo does not install it", appID, appID)
}

// WorkshopMod finds a workshop item the Steam client downloaded. dzo never
// downloads into Steam directories: a missing mod must be subscribed to in the
// Workshop, and the launcher started once.
func WorkshopMod(libs []string, app uint32, id uint64) (string, error) {
	for _, lib := range libs {
		d := filepath.Join(lib, "steamapps", "workshop", "content", strconv.FormatUint(uint64(app), 10), strconv.FormatUint(id, 10))
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d, nil
		}
	}
	return "", fmt.Errorf("workshop item %d is not in the Steam client's workshop directory: subscribe to it in the Steam Workshop and start the DayZ launcher once", id)
}

// Guard refuses to run on a host that manages instances: a real DayZ server
// never runs next to live ones (§C22), and there is no override. Such a host
// has instance directories under paths.instances, or dzo quadlets for the
// user.
func Guard(instancesDir, quadletDir string) error {
	if es, err := os.ReadDir(instancesDir); err == nil && len(es) > 0 {
		return fmt.Errorf("dzo test boot is a development tool and never runs where dzo manages instances: %s is not empty", instancesDir)
	}
	if units, _ := filepath.Glob(filepath.Join(quadletDir, "dzo-*.container")); len(units) > 0 {
		return fmt.Errorf("dzo test boot is a development tool and never runs where dzo manages instances: %s exists", units[0])
	}
	return nil
}

// ModScripts counts the script files a mod ships per module (Game, World,
// Mission), from the PBOs in its addons directory. A file counts if its path
// has a 3_Game, 4_World or 5_Mission directory and ends in .c.
func ModScripts(modDir string) (map[string]int, error) {
	out := map[string]int{}
	pbos, err := filepath.Glob(filepath.Join(modDir, "addons", "*.pbo"))
	if err != nil {
		return nil, err
	}
	sort.Strings(pbos)
	for _, p := range pbos {
		es, err := pbo.ListEntries(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		for _, e := range es {
			path := strings.ToLower(strings.ReplaceAll(e.Path, `\`, "/"))
			if !strings.HasSuffix(path, ".c") {
				continue
			}
			for dir, module := range map[string]string{"3_game": "Game", "4_world": "World", "5_mission": "Mission"} {
				if strings.Contains("/"+path, "/"+dir+"/") {
					out[module]++
				}
			}
		}
	}
	return out, nil
}

// BaselinePath is where the vanilla baseline of a server build is cached. The
// build is identified by the size and time of the DayZServer binary, which
// changes with every update.
func BaselinePath(cacheDir, product, serverDir string) (string, error) {
	fi, err := os.Stat(filepath.Join(serverDir, "DayZServer"))
	if err != nil {
		return "", errors.New("no DayZServer in " + serverDir)
	}
	return filepath.Join(cacheDir, "boottest", "baseline", product, fmt.Sprintf("%d-%d.json", fi.Size(), fi.ModTime().Unix())), nil
}
