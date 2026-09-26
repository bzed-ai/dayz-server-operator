// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package steam

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadStatusMissingFileReturnsZeroValue(t *testing.T) {
	s, err := LoadStatus(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("LoadStatus: %v", err)
	}
	if s.Account != "" || s.AuthRequired {
		t.Fatalf("expected a zero-value Status, got %+v", s)
	}
}

func TestStatusSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status.json")
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	want := Status{Account: "bob", AuthRequired: true, LastFailureAt: &now, LastFailureReason: FailureInvalidPassword}

	if err := want.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadStatus(path)
	if err != nil {
		t.Fatalf("LoadStatus: %v", err)
	}
	if got.Account != want.Account || got.AuthRequired != want.AuthRequired || got.LastFailureReason != want.LastFailureReason {
		t.Fatalf("got = %+v, want %+v", got, want)
	}
	if got.LastFailureAt == nil || !got.LastFailureAt.Equal(now) {
		t.Fatalf("LastFailureAt = %v, want %v", got.LastFailureAt, now)
	}
}

func TestLoadStatusInvalidJSONErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadStatus(path); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestStatusSaveMissingParentDirErrors(t *testing.T) {
	s := Status{Account: "bob"}
	err := s.Save(filepath.Join(t.TempDir(), "missing-dir", "status.json"))
	if err == nil {
		t.Fatal("expected an error when the parent directory does not exist")
	}
}

func TestStatusRecordSuccessClearsAuthRequired(t *testing.T) {
	s := Status{Account: "bob", AuthRequired: true, LastFailureReason: FailureInvalidPassword}
	now := time.Now()
	s.Record("bob", Result{Success: true}, now)

	if s.AuthRequired {
		t.Error("AuthRequired should be cleared on a successful login")
	}
	if s.LastFailureReason != "" {
		t.Errorf("LastFailureReason = %q, want cleared", s.LastFailureReason)
	}
	if s.LastSuccessAt == nil || !s.LastSuccessAt.Equal(now) {
		t.Errorf("LastSuccessAt = %v, want %v", s.LastSuccessAt, now)
	}
}

func TestStatusRecordAuthFailureSetsAuthRequired(t *testing.T) {
	s := Status{}
	now := time.Now()
	s.Record("bob", Result{Success: false, Reason: FailureInvalidPassword}, now)

	if !s.AuthRequired {
		t.Error("expected AuthRequired to be set on an auth failure")
	}
	if s.LastFailureAt == nil || !s.LastFailureAt.Equal(now) {
		t.Errorf("LastFailureAt = %v, want %v", s.LastFailureAt, now)
	}
	if s.LastFailureReason != FailureInvalidPassword {
		t.Errorf("LastFailureReason = %q, want invalid_password", s.LastFailureReason)
	}
}

func TestStatusRecordTransientFailureDoesNotSetAuthRequired(t *testing.T) {
	s := Status{}
	s.Record("bob", Result{Success: false, Reason: FailureRateLimited}, time.Now())
	if s.AuthRequired {
		t.Error("a rate-limit failure should not require re-authentication")
	}
}

func TestIsAuthFailure(t *testing.T) {
	authFailures := []FailureReason{
		FailureInvalidPassword, FailureGuardCodeMismatch, FailureAccountLogonDenied, FailureCachedCredentialsGone,
	}
	for _, r := range authFailures {
		if !IsAuthFailure(r) {
			t.Errorf("IsAuthFailure(%v) = false, want true", r)
		}
	}
	transient := []FailureReason{FailureRateLimited, FailureNoSubscription, FailureUnknown, ""}
	for _, r := range transient {
		if IsAuthFailure(r) {
			t.Errorf("IsAuthFailure(%v) = true, want false", r)
		}
	}
}
