// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package battleye

import "testing"

func TestParseEventConnect(t *testing.T) {
	e := ParseEvent("Player #3 Survivor123 (203.0.113.5:2304) connected")
	if e.Kind != EventConnect {
		t.Fatalf("Kind = %v, want EventConnect", e.Kind)
	}
	if e.PlayerID != 3 || e.PlayerName != "Survivor123" || e.Address != "203.0.113.5:2304" {
		t.Errorf("Event = %+v", e)
	}
}

func TestParseEventGUID(t *testing.T) {
	e := ParseEvent("Player #3 Survivor123 - GUID: 0123456789abcdef0123456789abcdef")
	if e.Kind != EventGUID {
		t.Fatalf("Kind = %v, want EventGUID", e.Kind)
	}
	if e.PlayerID != 3 || e.PlayerName != "Survivor123" || e.GUID != "0123456789abcdef0123456789abcdef" {
		t.Errorf("Event = %+v", e)
	}
}

func TestParseEventVerifiedGUID(t *testing.T) {
	e := ParseEvent("Verified GUID (0123456789abcdef0123456789abcdef) for player #3 Survivor123")
	if e.Kind != EventVerifiedGUID {
		t.Fatalf("Kind = %v, want EventVerifiedGUID", e.Kind)
	}
	if e.PlayerID != 3 || e.PlayerName != "Survivor123" || e.GUID != "0123456789abcdef0123456789abcdef" {
		t.Errorf("Event = %+v", e)
	}
}

func TestParseEventDisconnect(t *testing.T) {
	e := ParseEvent("Player #3 Survivor123 disconnected")
	if e.Kind != EventDisconnect {
		t.Fatalf("Kind = %v, want EventDisconnect", e.Kind)
	}
	if e.PlayerID != 3 || e.PlayerName != "Survivor123" {
		t.Errorf("Event = %+v", e)
	}
}

func TestParseEventChat(t *testing.T) {
	e := ParseEvent("(Side) Survivor123: hello there")
	if e.Kind != EventChat {
		t.Fatalf("Kind = %v, want EventChat", e.Kind)
	}
	if e.Channel != "Side" || e.PlayerName != "Survivor123" || e.Message != "hello there" {
		t.Errorf("Event = %+v", e)
	}
}

func TestParseEventKick(t *testing.T) {
	e := ParseEvent("Player #3 Survivor123 has been kicked by BattlEye: Admin Kick")
	if e.Kind != EventKick {
		t.Fatalf("Kind = %v, want EventKick", e.Kind)
	}
	if e.PlayerID != 3 || e.PlayerName != "Survivor123" || e.Message != "Admin Kick" {
		t.Errorf("Event = %+v", e)
	}
}

func TestParseEventUnknown(t *testing.T) {
	e := ParseEvent("something we've never seen before")
	if e.Kind != EventUnknown {
		t.Fatalf("Kind = %v, want EventUnknown", e.Kind)
	}
	if e.Raw != "something we've never seen before" {
		t.Errorf("Raw = %q", e.Raw)
	}
}
