// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bzed-ai/dayz-server-operator/internal/battleye"
	"github.com/bzed-ai/dayz-server-operator/internal/instance"
)

// newInstanceCmd wires the lifecycle operations internal/instance
// implements (§C8). Resolving a real instance's mission/product config to
// build a full quadlet spec, and the announcement countdown before a
// graceful restart's lock/kick phase, are left to a future config-driven
// pass (see internal/instance's package doc comment); these commands
// operate directly on a unit/RCon target the operator names.
func newInstanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Instance lifecycle: start/stop/restart, the F3 failure gate (§C5/§C8)",
	}
	cmd.AddCommand(newInstanceStartCmd(), newInstanceStopCmd(), newInstanceRestartCmd(), newInstanceAckFailureCmd())
	return cmd
}

func lifecycleFlags(cmd *cobra.Command, lc *instance.Lifecycle) {
	cmd.Flags().StringVar(&lc.Command, "command", "systemctl", "systemctl binary to run")
	cmd.Flags().BoolVar(&lc.UserMode, "user", true, "pass --user to systemctl (the normal case: a per-user systemd instance)")
}

func newInstanceStartCmd() *cobra.Command {
	var lc instance.Lifecycle
	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "systemctl start dzo-<name>.service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return lc.Start(cmd.Context(), args[0])
		},
	}
	lifecycleFlags(cmd, &lc)
	return cmd
}

func newInstanceStopCmd() *cobra.Command {
	var lc instance.Lifecycle
	cmd := &cobra.Command{
		Use:   "stop <name>",
		Short: "systemctl stop dzo-<name>.service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return lc.Stop(cmd.Context(), args[0])
		},
	}
	lifecycleFlags(cmd, &lc)
	return cmd
}

func newInstanceRestartCmd() *cobra.Command {
	var lc instance.Lifecycle
	var graceful bool
	var rconAddr, rconPassword, reason string
	var delay, rconTimeout time.Duration
	var kickPasses int
	cmd := &cobra.Command{
		Use:   "restart <name>",
		Short: "Restart an instance, plainly or via the lock/kick/#kick sequence (--graceful)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !graceful {
				return lc.Restart(cmd.Context(), args[0])
			}
			if rconAddr == "" {
				return fmt.Errorf("--rcon-addr is required with --graceful")
			}
			client, err := battleye.Dial(rconAddr, rconPassword, battleye.WithLoginTimeout(rconTimeout))
			if err != nil {
				// §C8: if RCon is unreachable, log it and restart via
				// systemd immediately rather than blocking the restart on it.
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "RCon unreachable (%v), restarting immediately\n", err)
				return lc.Restart(cmd.Context(), args[0])
			}
			defer func() { _ = client.Close() }()
			return instance.GracefulRestart(cmd.Context(), client, lc, args[0], instance.GracefulRestartOptions{
				Reason: reason, KickPasses: kickPasses, Delay: delay,
			})
		},
	}
	lifecycleFlags(cmd, &lc)
	cmd.Flags().BoolVar(&graceful, "graceful", false, "lock, kick players, then restart (§C8) instead of an immediate restart")
	cmd.Flags().StringVar(&rconAddr, "rcon-addr", "", "RCon host:port (required with --graceful)")
	cmd.Flags().StringVar(&rconPassword, "rcon-password", "", "RCon password (with --graceful)")
	cmd.Flags().StringVar(&reason, "reason", "server restart", "kick reason shown to players")
	cmd.Flags().DurationVar(&delay, "delay", 3*time.Second, "wait after the final kick before restarting")
	cmd.Flags().IntVar(&kickPasses, "kick-passes", 3, "number of players+kick rounds before the final #kick -1")
	cmd.Flags().DurationVar(&rconTimeout, "rcon-timeout", 5*time.Second, "RCon login handshake timeout")
	return cmd
}

func newInstanceAckFailureCmd() *cobra.Command {
	var lc instance.Lifecycle
	var gateFile string
	cmd := &cobra.Command{
		Use:   "ack-failure <name>",
		Short: "Clear the F3 failed-render gate and systemd's reset-failed state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := lc.AckFailure(cmd.Context(), args[0]); err != nil {
				return err
			}
			if gateFile == "" {
				return nil
			}
			gate := instance.FailureGate{}
			gate.Ack()
			return gate.State.Save(gateFile)
		},
	}
	lifecycleFlags(cmd, &lc)
	cmd.Flags().StringVar(&gateFile, "gate-file", "", "path to the instance's persisted failed-render gate state (cleared if given)")
	return cmd
}
