// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package instance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GateState is the persisted failed-render gate for one instance (F3
// layer 2): once a start's pre-flight render fails, further starts with
// the same InputHash are refused without retrying the render, until the
// hash changes or an operator acks the failure.
type GateState struct {
	Failed    bool      `json:"failed"`
	InputHash string    `json:"input_hash,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	At        time.Time `json:"at,omitempty"`
}

// LoadGateState reads path's persisted GateState. A missing file is not
// an error: it returns a zero GateState (not failed), matching an
// instance that has never had a render failure recorded.
func LoadGateState(path string) (GateState, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the operator-configured instance state dir, not external input
	if err != nil {
		if os.IsNotExist(err) {
			return GateState{}, nil
		}
		return GateState{}, fmt.Errorf("instance: read gate state %s: %w", path, err)
	}
	var s GateState
	if err := json.Unmarshal(data, &s); err != nil {
		return GateState{}, fmt.Errorf("instance: parse gate state %s: %w", path, err)
	}
	return s, nil
}

// Save writes s to path atomically (temp file + rename), mirroring
// internal/mission's manifest write pattern.
func (s GateState) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("instance: marshal gate state: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gate-*.tmp")
	if err != nil {
		return fmt.Errorf("instance: create temp gate state: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup; the rename below removes it on success

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("instance: write temp gate state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("instance: close temp gate state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("instance: activate gate state: %w", err)
	}
	return nil
}

// HashInput hashes the bytes that decide whether a render is expected to
// fail the same way again (the resolved instance config, mission source
// ref, mod list, and anything else the caller considers part of "the
// inputs" - this package has no opinion on what belongs in the hash).
func HashInput(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// FailureGate wraps a GateState with the decisions Lifecycle needs. It is
// not itself persisted; callers Load a GateState, wrap it, decide, then
// Save whatever they end up with.
type FailureGate struct {
	State GateState
}

// Blocked reports whether a start with inputHash should be refused
// without attempting a render: the gate is set (a prior render failed)
// and the inputs have not changed since.
func (g FailureGate) Blocked(inputHash string) bool {
	return g.State.Failed && g.State.InputHash == inputHash
}

// RecordFailure sets the gate after a render failed for inputHash.
func (g *FailureGate) RecordFailure(inputHash, reason string, at time.Time) {
	g.State = GateState{Failed: true, InputHash: inputHash, Reason: reason, At: at}
}

// RecordSuccess clears the gate after a render (or an operator ack)
// succeeds.
func (g *FailureGate) RecordSuccess() {
	g.State = GateState{}
}

// Ack clears the gate unconditionally (`dzo instance ack-failure`): the
// operator has decided to allow starts again regardless of whether the
// underlying config was actually fixed.
func (g *FailureGate) Ack() {
	g.State = GateState{}
}
