// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bzed/dayz-server-operator/internal/servercfg"
)

func newServerCfgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "servercfg",
		Short: "Inspect and diff serverDZ.cfg files",
	}
	cmd.AddCommand(newServerCfgShowCmd(), newServerCfgDiffCmd())
	return cmd
}

func loadServerCfg(path string) (*servercfg.File, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is a CLI argument the operator passes by design
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	f, err := servercfg.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return f, nil
}

func newServerCfgShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <serverDZ.cfg>",
		Short: "Parse a serverDZ.cfg file and print its effective keys",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := loadServerCfg(args[0])
			if err != nil {
				return err
			}
			for _, key := range f.Keys() {
				e, _ := f.Get(key)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), e.String())
			}
			return nil
		},
	}
}

func newServerCfgDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <old.cfg> <new.cfg>",
		Short: "Show the effective key differences before adopting a new serverDZ.cfg (FR-12)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldFile, err := loadServerCfg(args[0])
			if err != nil {
				return err
			}
			newFile, err := loadServerCfg(args[1])
			if err != nil {
				return err
			}
			d := servercfg.DiffFiles(oldFile, newFile)
			out := cmd.OutOrStdout()
			if d.Empty() {
				_, _ = fmt.Fprintln(out, "no differences")
				return nil
			}
			for _, k := range d.Added {
				_, _ = fmt.Fprintf(out, "+ %s\n", k)
			}
			for _, k := range d.Removed {
				_, _ = fmt.Fprintf(out, "- %s\n", k)
			}
			for _, c := range d.Changed {
				_, _ = fmt.Fprintf(out, "~ %s: %s -> %s\n", c.Key, c.Old, c.New)
			}
			return nil
		},
	}
}
