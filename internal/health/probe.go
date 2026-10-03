// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package health

import (
	"context"
	"fmt"
	"time"

	"github.com/bzed/dayz-server-operator/internal/a2s"
	"github.com/bzed/dayz-server-operator/internal/battleye"
)

// RConProbe optionally adds a BattlEye "version" round trip to a liveness
// check (§C9: "off by default, A8.10" - RCon flakiness caused restart
// loops in the legacy tooling, so this stays opt-in).
type RConProbe struct {
	Addr     string
	Password string
	Timeout  time.Duration
}

// Options configures Probe. QueryAddr and ProcRoot have no default: the
// caller (the CLI) is expected to always set them from real instance
// config, so a missing value fails loudly rather than silently probing
// the wrong thing.
type Options struct {
	QueryAddr    string
	Timeout      time.Duration
	ProcRoot     string
	ProcessNames []string
	RCon         *RConProbe // nil disables the RCon probe
}

func (o Options) timeout() time.Duration {
	if o.Timeout <= 0 {
		return 10 * time.Second // matches HealthTimeout in the §C5 quadlet sketch
	}
	return o.Timeout
}

func (o Options) processNames() []string {
	if len(o.ProcessNames) == 0 {
		return DefaultProcessNames
	}
	return o.ProcessNames
}

// Probe is the shared logic behind both "dzo health startup" and
// "dzo health live": the game process must exist, and its query port must
// answer A2S_INFO. The two commands differ only in how systemd schedules
// and interprets repeated failures (HealthStartupRetries vs.
// HealthRetries), not in what "healthy" means, so one function covers
// both.
func Probe(opts Options) error {
	if opts.QueryAddr == "" {
		return fmt.Errorf("health: QueryAddr is required")
	}
	if opts.ProcRoot == "" {
		return fmt.Errorf("health: ProcRoot is required")
	}

	running, err := ProcessRunning(opts.ProcRoot, opts.processNames())
	if err != nil {
		return err
	}
	if !running {
		return fmt.Errorf("health: server process not found (looked for %v under %s)", opts.processNames(), opts.ProcRoot)
	}

	if _, err := a2s.Query(opts.QueryAddr, opts.timeout()); err != nil {
		return fmt.Errorf("health: query port %s not answering: %w", opts.QueryAddr, err)
	}

	if opts.RCon != nil {
		if err := probeRCon(*opts.RCon); err != nil {
			return err
		}
	}
	return nil
}

func probeRCon(p RConProbe) error {
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client, err := battleye.Dial(p.Addr, p.Password, battleye.WithLoginTimeout(timeout))
	if err != nil {
		return fmt.Errorf("health: rcon login failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if _, err := client.Command(ctx, "version"); err != nil {
		return fmt.Errorf("health: rcon version probe failed: %w", err)
	}
	return nil
}
