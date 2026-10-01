// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bzed-ai/dayz-server-operator/internal/servermods"
)

// newServermodsCmd is the build-time tool behind `make servermods`.
func newServermodsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "servermods", Short: "Build dzo's own servermods (development and packaging)"}
	var src, out string
	build := &cobra.Command{
		Use:   "build",
		Short: "Pack servermods/*/src/* into PBOs, reproducibly",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := servermods.Build(ctxOf(cmd), src, out)
			if err != nil {
				return err
			}
			for _, r := range res {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  (prefix %s)\n", r.SHA256, r.PBO, r.Prefix)
			}
			return nil
		},
	}
	build.Flags().StringVar(&src, "src", "servermods", "directory holding the servermod checkouts")
	build.Flags().StringVar(&out, "out", "dist/servermods", "output directory")
	cmd.AddCommand(build)
	return cmd
}
