// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/bzed/dayz-server-operator/internal/cache"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Inspect and garbage-collect an immutable generation cache (§C2/§C7)",
	}
	cmd.AddCommand(newCacheListCmd(), newCacheGCCmd())
	return cmd
}

func newCacheListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <store-dir>",
		Short: "List generations (oldest first) and the current one",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s := cache.New(args[0])
			gens, err := s.Generations()
			if err != nil {
				return err
			}
			current, err := s.Current()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, g := range gens {
				marker := " "
				if g == current {
					marker = "*"
				}
				_, _ = fmt.Fprintf(out, "%s %s\n", marker, g)
			}
			return nil
		},
	}
}

func newCacheGCCmd() *cobra.Command {
	var keep int
	c := &cobra.Command{
		Use:   "gc <store-dir>",
		Short: "Remove generations beyond --keep, always preserving the current one",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s := cache.New(args[0])
			removed, err := s.GC(keep)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, r := range removed {
				_, _ = fmt.Fprintf(out, "removed %s\n", r)
			}
			_, _ = fmt.Fprintf(out, "removed %s generation(s), kept %s\n", strconv.Itoa(len(removed)), strconv.Itoa(keep))
			return nil
		},
	}
	c.Flags().IntVar(&keep, "keep", 3, "number of most recent generations to keep")
	return c
}
