// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/bzed-ai/dayz-server-operator/internal/config"
	"github.com/bzed-ai/dayz-server-operator/internal/resolve"
	"github.com/bzed-ai/dayz-server-operator/internal/site"
	"github.com/spf13/cobra"
)

// loadSite loads config.yaml and the site checkout it points to.
func loadSite(configPath string) (*config.Config, *site.Tree, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, err
	}
	tree, err := site.LoadTree(cfg.Paths.Site)
	if err != nil {
		return nil, nil, err
	}
	return cfg, tree, nil
}

// loadInstance resolves one instance for the commands that act on it.
func loadInstance(configPath, name string) (*config.Config, *resolve.Instance, error) {
	cfg, tree, err := loadSite(configPath)
	if err != nil {
		return nil, nil, err
	}
	inst, err := resolve.Resolve(cfg, tree, name)
	return cfg, inst, err
}

func configFlag(cmd *cobra.Command, path *string) {
	cmd.Flags().StringVar(path, "config", defaultConfigPath, "path to config.yaml")
}
