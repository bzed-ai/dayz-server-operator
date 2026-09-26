// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package instance

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadGateStateMissingFileReturnsZeroValue(t *testing.T) {
	s, err := LoadGateState(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("LoadGateState: %v", err)
	}
	if s.Failed {
		t.Errorf("s = %+v, want not failed", s)
	}
}

func TestGateStateSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gate.json")
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	want := GateState{Failed: true, InputHash: "sha256:abc", Reason: "render: bad XML", At: now}

	if err := want.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadGateState(path)
	if err != nil {
		t.Fatalf("LoadGateState: %v", err)
	}
	if got.Failed != want.Failed || got.InputHash != want.InputHash || got.Reason != want.Reason || !got.At.Equal(want.At) {
		t.Errorf("got = %+v, want %+v", got, want)
	}
}

func TestLoadGateStateInvalidJSONErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gate.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadGateState(path); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestGateStateSaveMissingParentDirErrors(t *testing.T) {
	s := GateState{Failed: true}
	if err := s.Save(filepath.Join(t.TempDir(), "missing-dir", "gate.json")); err == nil {
		t.Fatal("expected an error when the parent directory does not exist")
	}
}

func TestFailureGateBlockedSameHash(t *testing.T) {
	g := FailureGate{}
	g.RecordFailure("sha256:abc", "bad config", time.Now())
	if !g.Blocked("sha256:abc") {
		t.Error("expected Blocked to be true for the same input hash")
	}
}

func TestFailureGateNotBlockedDifferentHash(t *testing.T) {
	g := FailureGate{}
	g.RecordFailure("sha256:abc", "bad config", time.Now())
	if g.Blocked("sha256:def") {
		t.Error("expected Blocked to be false once the input hash changes")
	}
}

func TestFailureGateNotBlockedWhenNeverFailed(t *testing.T) {
	g := FailureGate{}
	if g.Blocked("sha256:anything") {
		t.Error("expected Blocked to be false with no prior failure")
	}
}

func TestFailureGateRecordSuccessClears(t *testing.T) {
	g := FailureGate{}
	g.RecordFailure("sha256:abc", "bad config", time.Now())
	g.RecordSuccess()
	if g.Blocked("sha256:abc") {
		t.Error("expected Blocked to be false after RecordSuccess")
	}
}

func TestFailureGateAckClears(t *testing.T) {
	g := FailureGate{}
	g.RecordFailure("sha256:abc", "bad config", time.Now())
	g.Ack()
	if g.Blocked("sha256:abc") {
		t.Error("expected Blocked to be false after Ack")
	}
}

func TestHashInputStable(t *testing.T) {
	a := HashInput([]byte("same input"))
	b := HashInput([]byte("same input"))
	c := HashInput([]byte("different input"))
	if a != b {
		t.Error("HashInput should be deterministic for the same bytes")
	}
	if a == c {
		t.Error("HashInput should differ for different bytes")
	}
}
