// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bzed-ai/dayz-server-operator/internal/config"
)

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
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "config", defaultConfigPath, "path to config.yaml")
	return cmd
}
