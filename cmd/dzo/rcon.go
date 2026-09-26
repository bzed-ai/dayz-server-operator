// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bzed-ai/dayz-server-operator/internal/battleye"
)

func newRconCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rcon",
		Short: "Talk to an instance's built-in BattlEye RCon client (D30)",
	}
	cmd.AddCommand(newRconExecCmd())
	return cmd
}

func newRconExecCmd() *cobra.Command {
	var addr, password string
	var timeout time.Duration
	c := &cobra.Command{
		Use:   "exec <command...>",
		Short: "Send one RCon command and print its response",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := battleye.Dial(addr, password)
			if err != nil {
				return fmt.Errorf("rcon: connect: %w", err)
			}
			defer client.Close()

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			command := args[0]
			for _, a := range args[1:] {
				command += " " + a
			}
			resp, err := client.Command(ctx, command)
			if err != nil {
				return fmt.Errorf("rcon: command: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), resp)
			return nil
		},
	}
	c.Flags().StringVar(&addr, "addr", "", "RCon host:port")
	c.Flags().StringVar(&password, "password", "", "RCon password")
	c.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "command timeout")
	_ = c.MarkFlagRequired("addr")
	_ = c.MarkFlagRequired("password")
	return c
}
