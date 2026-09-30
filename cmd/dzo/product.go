// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/spf13/cobra"
)

// parseUint64Args parses each of args as a uint64, for a variadic list of
// numeric ids (workshop item ids).
// newProductCmd wires the two steamcmd job types internal/product
// implements (§C7). Resolving a product/mod list from config.yaml and
// site instances, detecting build ids via app_info_print, and snapshotting
// a successful download into a cache generation (internal/cache) are left
// to internal/instance, which knows what instances actually reference -
// see internal/product's package doc comment. These commands run a job
// directly against an operator-given install directory.
func newProductCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "product",
		Short: "Manual server build downloads (§C7); never automatic (D7)",
	}
	cmd.AddCommand(newProductInstallCmd(), newProductUpdateCmd())
	return cmd
}

func newModCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Workshop mod downloads and dependency checks (§C7)",
	}
	cmd.AddCommand(newModCfgPatchesCmd(), newModDepsCmd(),
		newModListCmd(), newModAddCmd(), newModUpdateCmd(), newModRefreshCmd())
	return cmd
}
