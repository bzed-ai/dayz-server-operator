// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bzed-ai/dayz-server-operator/internal/notify"
)

func newNotifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Send notifications (Discord)",
	}
	cmd.AddCommand(newNotifyTestCmd())
	return cmd
}

func newNotifyTestCmd() *cobra.Command {
	var url, message string
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Send a test message to a Discord webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			ts, err := notify.NewTemplateSet()
			if err != nil {
				return err
			}
			n := &notify.DiscordNotifier{
				Webhooks:  []notify.Webhook{{Name: "test", URL: url}},
				Default:   "test",
				Templates: ts,
			}
			if err := ts.Override(notify.Kind("cli_test"), message); err != nil {
				return err
			}
			if err := n.Notify(context.Background(), notify.Event{Kind: notify.Kind("cli_test")}); err != nil {
				return fmt.Errorf("notify test: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "sent")
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "Discord webhook URL (required)")
	cmd.Flags().StringVar(&message, "message", "dzo notify test: it works 🎉", "message to send")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}
