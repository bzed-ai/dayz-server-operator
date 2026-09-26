// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package a2s

import (
	"bytes"
	"testing"
)

func TestInfoRequestNoChallenge(t *testing.T) {
	req := InfoRequest(nil)
	if !bytes.HasPrefix(req, []byte{0xFF, 0xFF, 0xFF, 0xFF, 'T'}) {
		t.Fatalf("request header = %v", req[:5])
	}
	if !bytes.Contains(req, []byte("Source Engine Query\x00")) {
		t.Errorf("request missing query string: %v", req)
	}
}

func TestInfoRequestWithChallenge(t *testing.T) {
	req := InfoRequest([]byte{1, 2, 3, 4})
	if !bytes.HasSuffix(req, []byte{1, 2, 3, 4}) {
		t.Errorf("request should end with the challenge bytes: %v", req)
	}
}

func TestPacketKind(t *testing.T) {
	packet := append([]byte{0xFF, 0xFF, 0xFF, 0xFF, 'I'}, []byte("body")...)
	kind, body, err := PacketKind(packet)
	if err != nil {
		t.Fatalf("PacketKind: %v", err)
	}
	if kind != 'I' || string(body) != "body" {
		t.Errorf("kind=%c body=%q", kind, body)
	}
}

func TestPacketKindRejectsShort(t *testing.T) {
	if _, _, err := PacketKind([]byte{0xFF, 0xFF}); err == nil {
		t.Fatal("expected an error for a too-short packet")
	}
}

func TestPacketKindRejectsBadHeader(t *testing.T) {
	if _, _, err := PacketKind([]byte{0, 0, 0, 0, 'I'}); err == nil {
		t.Fatal("expected an error for a bad header")
	}
}

func TestChallengeNumber(t *testing.T) {
	c, err := ChallengeNumber([]byte{9, 8, 7, 6, 0xFF})
	if err != nil {
		t.Fatalf("ChallengeNumber: %v", err)
	}
	if !bytes.Equal(c, []byte{9, 8, 7, 6}) {
		t.Errorf("ChallengeNumber = %v", c)
	}
}

func TestChallengeNumberTooShort(t *testing.T) {
	if _, err := ChallengeNumber([]byte{1, 2}); err == nil {
		t.Fatal("expected an error for a too-short challenge body")
	}
}

func buildInfoBody(t *testing.T, info InfoResponse) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteByte(info.Protocol)
	b.WriteString(info.Name)
	b.WriteByte(0)
	b.WriteString(info.Map)
	b.WriteByte(0)
	b.WriteString(info.Folder)
	b.WriteByte(0)
	b.WriteString(info.Game)
	b.WriteByte(0)
	b.WriteByte(byte(info.AppID))
	b.WriteByte(byte(info.AppID >> 8))
	b.WriteByte(info.Players)
	b.WriteByte(info.MaxPlayers)
	b.WriteByte(info.Bots)
	b.WriteByte(info.ServerType)
	b.WriteByte(info.Environment)
	b.WriteByte(info.Visibility)
	b.WriteByte(info.VAC)
	b.WriteString(info.Version)
	b.WriteByte(0)
	return b.Bytes()
}

func TestParseInfoResponse(t *testing.T) {
	want := InfoResponse{
		Protocol: 17, Name: "dzo test server", Map: "dayzOffline.chernarusplus",
		Folder: "dayz", Game: "DayZ", AppID: 2211,
		Players: 3, MaxPlayers: 60, Bots: 0,
		ServerType: 'd', Environment: 'l', Visibility: 0, VAC: 1,
		Version: "1.29.123456",
	}
	got, err := ParseInfoResponse(buildInfoBody(t, want))
	if err != nil {
		t.Fatalf("ParseInfoResponse: %v", err)
	}
	if got != want {
		t.Errorf("ParseInfoResponse() = %+v, want %+v", got, want)
	}
}

func TestParseInfoResponseTruncated(t *testing.T) {
	full := buildInfoBody(t, InfoResponse{Name: "x", Map: "y", Folder: "z", Game: "w", Version: "1"})
	for cut := 0; cut < len(full); cut++ {
		if _, err := ParseInfoResponse(full[:cut]); err == nil {
			t.Errorf("cut=%d: expected an error for truncated input", cut)
		}
	}
}

func TestParseInfoResponseUnterminatedString(t *testing.T) {
	// Protocol byte + an unterminated name string.
	if _, err := ParseInfoResponse([]byte{17, 'h', 'i'}); err == nil {
		t.Fatal("expected an error for an unterminated string")
	}
}
