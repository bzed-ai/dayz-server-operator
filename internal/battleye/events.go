// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package battleye

import (
	"regexp"
	"strconv"
)

// EventKind classifies an unsolicited BattlEye Server Message (FR-22: event
// stream connect/GUID/chat/kick).
type EventKind int

const (
	EventUnknown EventKind = iota
	EventConnect
	EventGUID
	EventDisconnect
	EventChat
	EventKick
	EventVerifiedGUID
)

// Event is a parsed Server Message.
type Event struct {
	Kind EventKind
	Raw  string

	PlayerID   int    // BE client slot number, when present
	PlayerName string // when present
	GUID       string // BattlEye GUID, when present
	Address    string // "ip:port", when present (connect)
	Channel    string // chat channel/scope, when present
	Message    string // chat text or kick reason, when present
}

// Patterns follow the documented BattlEye RCon message formats used by
// existing Arma/DayZ RCon clients. They are not yet verified against a real
// server (that capture is spike S4, §E); kept in one place so fixing them
// against real fixtures later only needs one edit.
var (
	reConnect       = regexp.MustCompile(`^Player #(\d+) (.+) \(([0-9.]+:\d+)\) connected$`)
	reGUIDComputing = regexp.MustCompile(`^Player #(\d+) (.+) - GUID: ([0-9a-f]+)$`)
	reGUIDVerified  = regexp.MustCompile(`^Verified GUID \(([0-9a-f]+)\) for player #(\d+) (.+)$`)
	reDisconnect    = regexp.MustCompile(`^Player #(\d+) (.+) disconnected$`)
	reChat          = regexp.MustCompile(`^\((\w+)\) (.+?): (.*)$`)
	reKick          = regexp.MustCompile(`^Player #(\d+) (.+) has been kicked by BattlEye: (.*)$`)
)

// ParseEvent classifies a Server Message payload into a structured Event.
// Anything that matches none of the known patterns is returned as
// EventUnknown with Raw set, rather than an error: unrecognised server
// chatter must never abort the RCon event loop.
func ParseEvent(payload string) Event {
	if m := reConnect.FindStringSubmatch(payload); m != nil {
		id, _ := strconv.Atoi(m[1])
		return Event{Kind: EventConnect, Raw: payload, PlayerID: id, PlayerName: m[2], Address: m[3]}
	}
	if m := reGUIDVerified.FindStringSubmatch(payload); m != nil {
		id, _ := strconv.Atoi(m[2])
		return Event{Kind: EventVerifiedGUID, Raw: payload, GUID: m[1], PlayerID: id, PlayerName: m[3]}
	}
	if m := reGUIDComputing.FindStringSubmatch(payload); m != nil {
		id, _ := strconv.Atoi(m[1])
		return Event{Kind: EventGUID, Raw: payload, PlayerID: id, PlayerName: m[2], GUID: m[3]}
	}
	if m := reKick.FindStringSubmatch(payload); m != nil {
		id, _ := strconv.Atoi(m[1])
		return Event{Kind: EventKick, Raw: payload, PlayerID: id, PlayerName: m[2], Message: m[3]}
	}
	if m := reDisconnect.FindStringSubmatch(payload); m != nil {
		id, _ := strconv.Atoi(m[1])
		return Event{Kind: EventDisconnect, Raw: payload, PlayerID: id, PlayerName: m[2]}
	}
	if m := reChat.FindStringSubmatch(payload); m != nil {
		return Event{Kind: EventChat, Raw: payload, Channel: m[1], PlayerName: m[2], Message: m[3]}
	}
	return Event{Kind: EventUnknown, Raw: payload}
}
