// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"sort"
	"sync/atomic"

	"github.com/spf13/cobra"

	"github.com/bzed/dayz-server-operator/internal/api"
	"github.com/bzed/dayz-server-operator/internal/config"
	"github.com/bzed/dayz-server-operator/internal/maptiles"
	"github.com/bzed/dayz-server-operator/internal/serve"
)

func newMapCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "map", Short: "Map imagery for the admin map"}
	tiles := &cobra.Command{Use: "tiles", Short: "Build and inspect map tiles from the game's own data"}
	tiles.AddCommand(newMapTilesBuildCmd(), newMapTilesStatusCmd())
	cmd.AddCommand(tiles)
	return cmd
}

func newMapTilesBuildCmd() *cobra.Command {
	var configPath string
	var force bool
	cmd := &cobra.Command{
		Use:   "build <map> [data.pbo]",
		Short: "Build the satellite tiles of a map from its data PBO",
		Long: "Reads the satellite tiles of the map's data PBO and builds an XYZ tile set that dzo serve " +
			"offers to the web interface. <map> is the world name (chernarusplus, enoch, sakhal, deerisle). " +
			"The PBO is the DayZ client's worlds_<map>_data.pbo (the dedicated server's copy only has " +
			"stubs), or a modded map's data.pbo; it defaults to map_tiles.maps.<map>.source. " +
			"A source that was built before is not built again unless --force is given.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			name := args[0]
			if !api.ValidTilePath(name, "metadata.json") {
				return fmt.Errorf("%q is not a map name (lower case letters, digits, - and _)", name)
			}
			src := cfg.MapTiles.Maps[name].Source
			if len(args) == 2 {
				src = args[1]
			}
			if src == "" {
				return fmt.Errorf("no data PBO for %s: give one as the second argument or set map_tiles.maps.%s.source", name, name)
			}
			var last atomic.Int64
			errw := cmd.ErrOrStderr()
			m, built, err := maptiles.Publish(ctxOf(cmd), serve.TilesDir(cfg), name, src, force, maptiles.Options{
				Progress: func(done, total int) {
					if p := int64(done * 10 / total); p > last.Swap(p) {
						_, _ = fmt.Fprintf(errw, "%d%%\n", p*10)
					}
				},
			})
			if err != nil {
				return err
			}
			verb := "built"
			if !built {
				verb = "already built, unchanged"
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s (%.0f x %.0f m, %.2f px/m, zoom 0 to %d)\n", name, verb, m.Hash, m.Width, m.Height, m.PxPerM, m.MaxZoom)
			return err
		},
	}
	configFlag(cmd, &configPath)
	cmd.Flags().BoolVar(&force, "force", false, "build again even if the source is unchanged")
	return cmd
}

func newMapTilesStatusCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "List the maps with tiles and whether their source file changed since",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			names := map[string]bool{}
			for n := range cfg.MapTiles.Maps {
				names[n] = true
			}
			if es, err := os.ReadDir(serve.TilesDir(cfg)); err == nil {
				for _, e := range es {
					names[e.Name()] = true
				}
			}
			sorted := make([]string, 0, len(names))
			for n := range names {
				sorted = append(sorted, n)
			}
			sort.Strings(sorted)
			out := cmd.OutOrStdout()
			for _, n := range sorted {
				state, hash := "no tiles", "-"
				if m, err := maptiles.Current(serve.TilesDir(cfg), n); err == nil {
					state, hash = "ready, built "+m.Built.Format("2006-01-02"), m.Hash
					if src := cfg.MapTiles.Maps[n].Source; src != "" {
						if h, err := maptiles.SourceHash(src); err != nil {
							state += ", source unreadable: " + err.Error()
						} else if h != m.Hash {
							state += ", STALE: the source file changed, run dzo map tiles build " + n
						}
					}
				}
				_, _ = fmt.Fprintf(out, "%-16s %-16s %s\n", n, hash, state)
			}
			return nil
		},
	}
	configFlag(cmd, &configPath)
	return cmd
}
