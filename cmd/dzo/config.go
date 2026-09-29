// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bzed-ai/dayz-server-operator/internal/config"
	"github.com/bzed-ai/dayz-server-operator/internal/resolve"
	"github.com/bzed-ai/dayz-server-operator/internal/site"
)

// validateSite resolves every instance of the site checkout, if there is
// one. A checkout that is not there yet is not an error: config.yaml is
// valid on its own before `dzo setup` has cloned the site repo.
func validateSite(cmd *cobra.Command, c *config.Config) error {
	out := cmd.OutOrStdout()
	if _, err := os.Stat(c.Paths.Site); err != nil {
		_, _ = fmt.Fprintf(out, "  site:      %s not present, instances not checked\n", c.Paths.Site)
		return nil
	}
	tree, err := site.LoadTree(c.Paths.Site)
	if err != nil {
		return err
	}
	all, err := resolve.ResolveAll(c, tree)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "  site:      %d instances resolve\n", len(all))
	for _, inst := range all {
		for _, m := range inst.Missing {
			_, _ = fmt.Fprintf(out, "  note: %s: %s\n", inst.Name, m)
		}
	}
	return nil
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and validate the operator configuration",
	}
	cmd.AddCommand(newConfigValidateCmd())
	return cmd
}

func newConfigValidateCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Load and validate config.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Load(path)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "config %s is valid\n", path)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  data:      %s\n", c.Paths.Data)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  instances: %s\n", c.Paths.Instances)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  products:  %d configured\n", len(c.Products))
			return validateSite(cmd, c)
		},
	}
	cmd.Flags().StringVar(&path, "config", defaultConfigPath, "path to config.yaml")
	return cmd
}
