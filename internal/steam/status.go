// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package steam

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status is the persisted state of the Steam account's login session:
// what `dzo steam status` prints, and (once internal/monitor's Source is
// wired to a real backend by internal/instance) the source of
// dzo_steam_session_valid/dzo_steam_auth_required.
type Status struct {
	Account           string        `json:"account"`
	AuthRequired      bool          `json:"auth_required"`
	LastLoginAt       *time.Time    `json:"last_login_at,omitempty"`
	LastSuccessAt     *time.Time    `json:"last_success_at,omitempty"`
	LastFailureAt     *time.Time    `json:"last_failure_at,omitempty"`
	LastFailureReason FailureReason `json:"last_failure_reason,omitempty"`
}

// LoadStatus reads path's persisted Status. A missing file is not an
// error: it returns a zero Status, matching an account that has never
// logged in from this installation.
func LoadStatus(path string) (Status, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the operator-configured steam status file, not external input
	if err != nil {
		if os.IsNotExist(err) {
			return Status{}, nil
		}
		return Status{}, fmt.Errorf("steam: read status %s: %w", path, err)
	}
	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		return Status{}, fmt.Errorf("steam: parse status %s: %w", path, err)
	}
	return s, nil
}

// Save writes s to path atomically (temp file + rename), mirroring
// internal/mission's manifest write pattern.
func (s Status) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("steam: marshal status: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".steam-status-*.tmp")
	if err != nil {
		return fmt.Errorf("steam: create temp status: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup; the rename below removes it on success

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("steam: write temp status: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("steam: close temp status: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("steam: activate status: %w", err)
	}
	return nil
}

// Record updates s in place from a completed Run result, observed at now.
func (s *Status) Record(account string, result Result, now time.Time) {
	s.Account = account
	s.LastLoginAt = &now
	if result.Success {
		s.AuthRequired = false
		s.LastSuccessAt = &now
		s.LastFailureReason = ""
		return
	}
	s.LastFailureAt = &now
	s.LastFailureReason = result.Reason
	if IsAuthFailure(result.Reason) {
		s.AuthRequired = true
	}
}

// IsAuthFailure reports whether reason means a human needs to
// re-authenticate (wrong password/code, an invalidated cached session), as
// opposed to a transient condition (rate limiting) that resolves on its
// own without a new login attempt. internal/product's job classifier
// reuses this to decide when to set the global steam_auth_required state
// (§C7) from a non-interactive steamcmd failure.
func IsAuthFailure(reason FailureReason) bool {
	switch reason {
	case FailureInvalidPassword, FailureGuardCodeMismatch, FailureAccountLogonDenied, FailureCachedCredentialsGone:
		return true
	default:
		return false
	}
}
