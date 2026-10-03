// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bzed/dayz-server-operator/internal/a2s"
)

// Status is a Monitoring Plugins API (Nagios) check status.
type Status int

const (
	StatusOK Status = iota
	StatusWarning
	StatusCritical
	StatusUnknown
)

// ExitCode is the process exit code Icinga/Nagios expect for this status.
func (s Status) ExitCode() int { return int(s) }

func (s Status) String() string {
	switch s {
	case StatusOK:
		return "OK"
	case StatusWarning:
		return "WARNING"
	case StatusCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// PerfDatum is one Monitoring Plugins performance data value
// ('label'=value[UOM];[warn];[crit];[min];[max]).
type PerfDatum struct {
	Label string
	Value float64
	UOM   string
	Warn  *float64
	Crit  *float64
}

func fmtFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

func (p PerfDatum) String() string {
	warn, crit := "", ""
	if p.Warn != nil {
		warn = fmtFloat(*p.Warn)
	}
	if p.Crit != nil {
		crit = fmtFloat(*p.Crit)
	}
	return fmt.Sprintf("'%s'=%s%s;%s;%s", p.Label, fmtFloat(p.Value), p.UOM, warn, crit)
}

// CheckResult is one check's outcome, ready to print and exit with.
type CheckResult struct {
	Status  Status
	Message string
	Perf    []PerfDatum
}

// Output formats r as Nagios plugins expect: "STATUS: message | perfdata".
func (r CheckResult) Output() string {
	out := fmt.Sprintf("%s: %s", r.Status, r.Message)
	if len(r.Perf) == 0 {
		return out
	}
	parts := make([]string, len(r.Perf))
	for i, p := range r.Perf {
		parts[i] = p.String()
	}
	return out + " | " + strings.Join(parts, " ")
}

func sumInts(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

// EvaluateInstance turns one instance's status into a check result. Order
// of checks matches severity: down/unhealthy first, then a failed render,
// then plain OK.
func EvaluateInstance(inst InstanceStatus) CheckResult {
	perf := []PerfDatum{
		{Label: "players", Value: float64(inst.Players)},
		{Label: "uptime", Value: inst.UptimeSeconds, UOM: "s"},
		{Label: "restarts", Value: float64(sumInts(inst.RestartsTotal))},
		{Label: "mission_drift_files", Value: float64(inst.MissionDriftFiles)},
	}

	switch {
	case !inst.Up:
		return CheckResult{Status: StatusCritical, Message: fmt.Sprintf("%s is down", inst.Name), Perf: perf}
	case inst.Health == HealthUnhealthy:
		return CheckResult{Status: StatusCritical, Message: fmt.Sprintf("%s health check failing", inst.Name), Perf: perf}
	case inst.Health == HealthStarting:
		return CheckResult{Status: StatusWarning, Message: fmt.Sprintf("%s is still starting", inst.Name), Perf: perf}
	case !inst.LastRenderSuccess:
		return CheckResult{Status: StatusWarning, Message: fmt.Sprintf("%s last render failed", inst.Name), Perf: perf}
	default:
		msg := fmt.Sprintf("%s healthy, %d/%d players", inst.Name, inst.Players, inst.MaxPlayers)
		return CheckResult{Status: StatusOK, Message: msg, Perf: perf}
	}
}

// EvaluateGlobal turns host-wide status into a check result.
func EvaluateGlobal(g GlobalStatus) CheckResult {
	perf := []PerfDatum{
		{Label: "updates_pending", Value: float64(g.UpdatesPending)},
		{Label: "jobs_failed", Value: float64(sumInts(g.JobsFailedTotal))},
		{Label: "disk_free", Value: float64(g.DiskFreeBytes), UOM: "B"},
	}
	if g.SteamAuthRequired {
		return CheckResult{Status: StatusWarning, Message: "Steam login required", Perf: perf}
	}
	if !g.SteamSessionValid {
		return CheckResult{Status: StatusWarning, Message: "Steam session invalid", Perf: perf}
	}
	return CheckResult{Status: StatusOK, Message: "operator healthy", Perf: perf}
}

// httpClient is a seam for tests.
var httpClient = http.DefaultClient

// CheckRemoteInstance fetches an instance's /status/<name> JSON from url
// and evaluates it (§C9: "dzo check remote --url ... --instance ...").
func CheckRemoteInstance(ctx context.Context, url string, timeout time.Duration) (CheckResult, error) {
	var inst InstanceStatus
	if err := fetchJSON(ctx, url, timeout, &inst); err != nil {
		return CheckResult{}, err
	}
	return EvaluateInstance(inst), nil
}

// CheckRemoteGlobal fetches the host-wide /status JSON from url and
// evaluates it.
func CheckRemoteGlobal(ctx context.Context, url string, timeout time.Duration) (CheckResult, error) {
	var snap Snapshot
	if err := fetchJSON(ctx, url, timeout, &snap); err != nil {
		return CheckResult{}, err
	}
	return EvaluateGlobal(snap.Global), nil
}

func fetchJSON(ctx context.Context, url string, timeout time.Duration, v any) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("monitor: build request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("monitor: fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("monitor: fetch %s: status %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("monitor: decode response from %s: %w", url, err)
	}
	return nil
}

// CheckA2S is a pure network check of the game server itself (independent
// of the operator, so it catches host/network outages the operator
// couldn't - §C9 item 2).
func CheckA2S(addr string, timeout time.Duration) CheckResult {
	start := time.Now()
	info, err := a2s.Query(addr, timeout)
	rtt := time.Since(start).Seconds()
	if err != nil {
		return CheckResult{Status: StatusCritical, Message: fmt.Sprintf("A2S query to %s failed: %v", addr, err)}
	}
	return CheckResult{
		Status:  StatusOK,
		Message: fmt.Sprintf("%s answering (%d/%d players)", addr, info.Players, info.MaxPlayers),
		Perf: []PerfDatum{
			{Label: "rtt", Value: rtt, UOM: "s"},
			{Label: "players", Value: float64(info.Players)},
		},
	}
}
