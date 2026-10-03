// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/bzed/dayz-server-operator/internal/health"
)

func newHealthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "In-container health probes for HealthStartupCmd/HealthCmd (§C9)",
	}
	cmd.AddCommand(newHealthStartupCmd(), newHealthLiveCmd())
	return cmd
}

func addCommonHealthFlags(cmd *cobra.Command, opts *health.Options) {
	cmd.Flags().StringVar(&opts.QueryAddr, "query", "", "A2S query host:port (required)")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 10*time.Second, "probe timeout")
	cmd.Flags().StringVar(&opts.ProcRoot, "proc-root", "/proc", "procfs root (for tests)")
	_ = cmd.MarkFlagRequired("query")
}

func newHealthStartupCmd() *cobra.Command {
	var opts health.Options
	cmd := &cobra.Command{
		Use:   "startup",
		Short: "HealthStartupCmd: is the server up yet?",
		RunE: func(cmd *cobra.Command, args []string) error {
			return health.Probe(opts)
		},
	}
	addCommonHealthFlags(cmd, &opts)
	return cmd
}

func newHealthLiveCmd() *cobra.Command {
	var opts health.Options
	var rcon health.RConProbe
	var useRCon bool
	cmd := &cobra.Command{
		Use:   "live",
		Short: "HealthCmd: is the server still alive?",
		RunE: func(cmd *cobra.Command, args []string) error {
			if useRCon {
				rcon.Timeout = opts.Timeout
				opts.RCon = &rcon
			}
			return health.Probe(opts)
		},
	}
	addCommonHealthFlags(cmd, &opts)
	cmd.Flags().BoolVar(&useRCon, "rcon", false, "also probe RCon with a 'version' command (off by default, A8.10)")
	cmd.Flags().StringVar(&rcon.Addr, "rcon-addr", "", "RCon host:port (with --rcon)")
	cmd.Flags().StringVar(&rcon.Password, "rcon-password", "", "RCon password (with --rcon)")
	return cmd
}
