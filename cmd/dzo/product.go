// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/bzed-ai/dayz-server-operator/internal/product"
)

// parseUint64Args parses each of args as a uint64, for a variadic list of
// numeric ids (workshop item ids).
func parseUint64Args(args []string) ([]uint64, error) {
	ids := make([]uint64, 0, len(args))
	for _, a := range args {
		id, err := strconv.ParseUint(a, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid id %q: %w", a, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

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
	cmd.AddCommand(newProductUpdateCmd())
	return cmd
}

func newModCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Workshop mod downloads and dependency checks (§C7)",
	}
	cmd.AddCommand(newModDownloadCmd(), newModCfgPatchesCmd(), newModDepsCmd())
	return cmd
}

func newModDownloadCmd() *cobra.Command {
	var opts product.WorkshopDownloadOptions
	cmd := &cobra.Command{
		Use:   "download <item-id> [item-id...]",
		Short: "Run steamcmd +workshop_download_item for one or more mods",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := parseUint64Args(args)
			if err != nil {
				return err
			}
			opts.ItemIDs = ids
			result, err := product.RunWorkshopDownload(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return printJobResult(cmd, result)
		},
	}
	cmd.Flags().StringVar(&opts.Command, "command", "steamcmd", "steamcmd binary to run")
	cmd.Flags().StringVar(&opts.Account, "account", "", "Steam account name (required; must already have a cached session, see `dzo steam login`)")
	cmd.Flags().Uint32Var(&opts.WorkshopAppID, "workshop-app-id", 221100, "Steam Workshop app id (221100 for DayZ)")
	cmd.Flags().BoolVar(&opts.Validate, "validate", true, "pass 'validate' to workshop_download_item")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

func newProductUpdateCmd() *cobra.Command {
	var opts product.AppUpdateOptions
	cmd := &cobra.Command{
		Use:   "update <app-id>",
		Short: "Run steamcmd +app_update for a product build",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid app id %q: %w", args[0], err)
			}
			opts.AppID = uint32(id)
			result, err := product.RunAppUpdate(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return printJobResult(cmd, result)
		},
	}
	cmd.Flags().StringVar(&opts.Command, "command", "steamcmd", "steamcmd binary to run")
	cmd.Flags().StringVar(&opts.Account, "account", "", "Steam account name (required; must already have a cached session, see `dzo steam login`)")
	cmd.Flags().StringVar(&opts.InstallDir, "install-dir", "", "+force_install_dir target (required)")
	cmd.Flags().StringVar(&opts.BetaBranch, "beta-branch", "", "optional steamcmd beta branch")
	cmd.Flags().StringVar(&opts.BetaPassword, "beta-password", "", "optional beta branch password")
	cmd.Flags().BoolVar(&opts.Validate, "validate", true, "pass 'validate' to app_update")
	_ = cmd.MarkFlagRequired("account")
	_ = cmd.MarkFlagRequired("install-dir")
	return cmd
}

func printJobResult(cmd *cobra.Command, result product.JobResult) error {
	if result.AuthRequired {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "AUTH REQUIRED: run `dzo steam login` and retry")
		return fmt.Errorf("product: steam authentication required")
	}
	if !result.Success {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "FAILED")
		for id, msg := range result.FailedItems {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  item %d: %s\n", id, msg)
		}
		return fmt.Errorf("product: job did not report success")
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "OK")
	return nil
}
