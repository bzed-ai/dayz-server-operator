// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/bzed-ai/dayz-server-operator/internal/quadlet"
)

// quadletSpecFile mirrors quadlet.ContainerSpec with plain strings for
// durations, so it can be written by hand in YAML (§C4/§C5 sketches use
// this shape before the full site repo instance.yaml schema exists).
type quadletSpecFile struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Image       string             `yaml:"image"`
	Network     string             `yaml:"network"`
	Ports       []quadletPortFile  `yaml:"ports"`
	Volumes     []quadletVolFile   `yaml:"volumes"`
	Environment map[string]string  `yaml:"environment"`
	Exec        []string           `yaml:"exec"`
	Health      *quadletHealthFile `yaml:"health"`
	Memory      string             `yaml:"memory"`
	CPUs        string             `yaml:"cpus"`
	StopTimeout string             `yaml:"stop_timeout"`
}

type quadletPortFile struct {
	Host      int    `yaml:"host"`
	Container int    `yaml:"container"`
	Protocol  string `yaml:"protocol"`
}

type quadletVolFile struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
	ReadOnly    bool   `yaml:"read_only"`
}

type quadletHealthFile struct {
	Cmd             string `yaml:"cmd"`
	Interval        string `yaml:"interval"`
	StartupCmd      string `yaml:"startup_cmd"`
	StartupInterval string `yaml:"startup_interval"`
	StartupRetries  int    `yaml:"startup_retries"`
	Retries         int    `yaml:"retries"`
	OnFailure       string `yaml:"on_failure"`
}

func (f quadletSpecFile) toSpec() (quadlet.ContainerSpec, error) {
	spec := quadlet.ContainerSpec{
		Name:        f.Name,
		Description: f.Description,
		Image:       f.Image,
		Network:     quadlet.Network(f.Network),
		Environment: f.Environment,
		Exec:        f.Exec,
		Memory:      f.Memory,
		CPUs:        f.CPUs,
	}
	for _, p := range f.Ports {
		spec.Ports = append(spec.Ports, quadlet.Port{HostPort: p.Host, ContainerPort: p.Container, Protocol: p.Protocol})
	}
	for _, v := range f.Volumes {
		spec.Volumes = append(spec.Volumes, quadlet.Volume{Source: v.Source, Destination: v.Destination, ReadOnly: v.ReadOnly})
	}
	if f.Health != nil {
		interval, err := parseDurationOrEmpty(f.Health.Interval)
		if err != nil {
			return spec, fmt.Errorf("health.interval: %w", err)
		}
		startupInterval, err := parseDurationOrEmpty(f.Health.StartupInterval)
		if err != nil {
			return spec, fmt.Errorf("health.startup_interval: %w", err)
		}
		spec.Health = quadlet.Health{
			Cmd:             f.Health.Cmd,
			Interval:        interval,
			Retries:         f.Health.Retries,
			OnFailure:       f.Health.OnFailure,
			StartupCmd:      f.Health.StartupCmd,
			StartupInterval: startupInterval,
			StartupRetries:  f.Health.StartupRetries,
		}
	}
	stopTimeout, err := parseDurationOrEmpty(f.StopTimeout)
	if err != nil {
		return spec, fmt.Errorf("stop_timeout: %w", err)
	}
	spec.StopTimeout = stopTimeout
	return spec, nil
}

func parseDurationOrEmpty(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

func newQuadletCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quadlet",
		Short: "Render podman quadlet unit files (§C4/§C5)",
	}
	cmd.AddCommand(newQuadletRenderCmd())
	return cmd
}

func newQuadletRenderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "render <spec.yaml>",
		Short: "Render a .container quadlet from a container spec file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read %s: %w", args[0], err)
			}
			var file quadletSpecFile
			if err := yaml.Unmarshal(data, &file); err != nil {
				return fmt.Errorf("parse %s: %w", args[0], err)
			}
			spec, err := file.toSpec()
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			out, err := quadlet.RenderContainer(spec)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
}
