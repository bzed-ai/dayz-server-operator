// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bzed-ai/dayz-server-operator/internal/monitor"
)

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Icinga/Nagios-style checks (Monitoring Plugins API, §C9)",
		// A Nagios plugin's contract is exit code + one stdout line; cobra's
		// default "Error: ..." stderr line on a non-nil RunE error would
		// just be noise on top of the already-printed plugin output below.
		SilenceErrors: true,
	}
	cmd.AddCommand(newCheckRemoteCmd(), newCheckA2SCmd())
	return cmd
}

// checkExitErr lets a check command set the process exit code to the
// Nagios status (0-3) instead of cobra's generic 1-on-any-error, while
// still printing the plugin-formatted output on cmd.OutOrStdout().
type checkExitErr struct{ code int }

func (e *checkExitErr) Error() string { return fmt.Sprintf("check exited %d", e.code) }

func newCheckRemoteCmd() *cobra.Command {
	var url, instance string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Fetch /status from a remote dzo-exporter and evaluate it",
		RunE: func(cmd *cobra.Command, args []string) error {
			var result monitor.CheckResult
			var err error
			if instance != "" {
				result, err = monitor.CheckRemoteInstance(context.Background(), url, timeout)
			} else {
				result, err = monitor.CheckRemoteGlobal(context.Background(), url, timeout)
			}
			if err != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "UNKNOWN: %v\n", err)
				return &checkExitErr{code: monitor.StatusUnknown.ExitCode()}
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.Output())
			if result.Status != monitor.StatusOK {
				return &checkExitErr{code: result.Status.ExitCode()}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "status URL (required), e.g. http://host:9464/status/deerisle")
	cmd.Flags().StringVar(&instance, "instance", "", "instance name (fetches /status/<name>'s InstanceStatus instead of the global status)")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "request timeout")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

func newCheckA2SCmd() *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "a2s <host:port>",
		Short: "Pure A2S network check of a game server's query port",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result := monitor.CheckA2S(args[0], timeout)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.Output())
			if result.Status != monitor.StatusOK {
				return &checkExitErr{code: result.Status.ExitCode()}
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "probe timeout")
	return cmd
}
