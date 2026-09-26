// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package battleye

import (
	"bytes"
	"testing"
)

// serverEncodeLoginResponse and friends below build server-side packets by
// hand (mirroring wrap()) so the tests don't rely on the client-side
// encoders to validate the client-side decoders.
func serverPacket(typ PacketType, rest ...byte) []byte {
	body := append([]byte{packetMarker, byte(typ)}, rest...)
	return wrap(body)
}

func TestEncodeLoginAndPeekType(t *testing.T) {
	pkt := EncodeLogin("s3cret")
	if pkt[0] != 'B' || pkt[1] != 'E' {
		t.Fatalf("bad header: %v", pkt[:2])
	}
	typ, err := PeekType(pkt)
	if err != nil {
		t.Fatalf("PeekType: %v", err)
	}
	if typ != PacketLogin {
		t.Errorf("type = %#x, want Login", typ)
	}
}

func TestDecodeLoginResponseSuccess(t *testing.T) {
	pkt := serverPacket(PacketLogin, 0x01)
	ok, err := DecodeLoginResponse(pkt)
	if err != nil {
		t.Fatalf("DecodeLoginResponse: %v", err)
	}
	if !ok {
		t.Error("expected login success")
	}
}

func TestDecodeLoginResponseFailure(t *testing.T) {
	pkt := serverPacket(PacketLogin, 0x00)
	ok, err := DecodeLoginResponse(pkt)
	if err != nil {
		t.Fatalf("DecodeLoginResponse: %v", err)
	}
	if ok {
		t.Error("expected login failure")
	}
}

func TestDecodeLoginResponseWrongType(t *testing.T) {
	pkt := serverPacket(PacketCommand, 0x00, 0x01)
	if _, err := DecodeLoginResponse(pkt); err == nil {
		t.Fatal("expected an error for a mistyped packet")
	}
}

func TestEncodeCommandAndSinglePacketResponse(t *testing.T) {
	pkt := EncodeCommand(7, "players")
	typ, err := PeekType(pkt)
	if err != nil || typ != PacketCommand {
		t.Fatalf("PeekType = %v, %v", typ, err)
	}

	resp := serverPacket(PacketCommand, 7, 'o', 'k')
	seq, multipart, payload, err := DecodeCommandResponse(resp)
	if err != nil {
		t.Fatalf("DecodeCommandResponse: %v", err)
	}
	if seq != 7 {
		t.Errorf("seq = %d, want 7", seq)
	}
	if multipart != nil {
		t.Errorf("multipart = %+v, want nil", multipart)
	}
	if string(payload) != "ok" {
		t.Errorf("payload = %q, want %q", payload, "ok")
	}
}

func TestEncodeCommandEmptyIsKeepAlive(t *testing.T) {
	pkt := EncodeCommand(1, "")
	body, err := unwrap(pkt)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if len(body) != 3 {
		t.Fatalf("expected a 3-byte body (marker, type, seq), got %d bytes", len(body))
	}
}

func TestDecodeCommandResponseMultipart(t *testing.T) {
	// part 1 of 2
	resp := serverPacket(PacketCommand, 9, 0x00, 2, 0, 'h', 'e')
	seq, multipart, payload, err := DecodeCommandResponse(resp)
	if err != nil {
		t.Fatalf("DecodeCommandResponse: %v", err)
	}
	if seq != 9 {
		t.Errorf("seq = %d, want 9", seq)
	}
	if multipart == nil || multipart.Total != 2 || multipart.Index != 0 {
		t.Fatalf("multipart = %+v", multipart)
	}
	if string(payload) != "he" {
		t.Errorf("payload = %q", payload)
	}

	// part 2 of 2
	resp = serverPacket(PacketCommand, 9, 0x00, 2, 1, 'l', 'l', 'o')
	_, multipart, payload, err = DecodeCommandResponse(resp)
	if err != nil {
		t.Fatalf("DecodeCommandResponse: %v", err)
	}
	if multipart == nil || multipart.Index != 1 {
		t.Fatalf("multipart = %+v", multipart)
	}
	if string(payload) != "llo" {
		t.Errorf("payload = %q", payload)
	}
}

func TestDecodeCommandResponseWrongType(t *testing.T) {
	resp := serverPacket(PacketLogin, 0x01)
	if _, _, _, err := DecodeCommandResponse(resp); err == nil {
		t.Fatal("expected an error for a mistyped packet")
	}
}

func TestServerMessageAndAck(t *testing.T) {
	msg := serverPacket(PacketMessage, 3, 'h', 'i')
	seq, payload, err := DecodeServerMessage(msg)
	if err != nil {
		t.Fatalf("DecodeServerMessage: %v", err)
	}
	if seq != 3 || string(payload) != "hi" {
		t.Errorf("seq=%d payload=%q", seq, payload)
	}

	ack := EncodeMessageAck(3)
	body, err := unwrap(ack)
	if err != nil {
		t.Fatalf("unwrap(ack): %v", err)
	}
	if len(body) != 3 || body[1] != byte(PacketMessage) || body[2] != 3 {
		t.Errorf("ack body = %v", body)
	}
}

func TestDecodeServerMessageWrongType(t *testing.T) {
	pkt := serverPacket(PacketCommand, 1, 'x')
	if _, _, err := DecodeServerMessage(pkt); err == nil {
		t.Fatal("expected an error for a mistyped packet")
	}
}

func TestUnwrapRejectsShortPacket(t *testing.T) {
	if _, err := unwrap([]byte{'B', 'E', 0, 0}); err == nil {
		t.Fatal("expected an error for a too-short packet")
	}
}

func TestUnwrapRejectsBadHeader(t *testing.T) {
	pkt := serverPacket(PacketLogin, 0x01)
	pkt[0] = 'X'
	if _, err := unwrap(pkt); err == nil {
		t.Fatal("expected an error for a bad 'BE' header")
	}
}

func TestUnwrapRejectsMissingMarker(t *testing.T) {
	pkt := serverPacket(PacketLogin, 0x01)
	pkt[6] = 0x00 // corrupt the 0xFF marker
	// Recompute nothing: this must be caught before CRC even matters, but
	// since the marker is part of the CRC input, corrupting it also breaks
	// the CRC. Rebuild with a bad marker but correct CRC to isolate the check.
	body := []byte{0x00, byte(PacketLogin), 0x01}
	bad := wrap(body)
	if _, err := unwrap(bad); err == nil {
		t.Fatal("expected an error for a missing 0xFF marker")
	}
}

func TestUnwrapRejectsCRCMismatch(t *testing.T) {
	pkt := serverPacket(PacketLogin, 0x01)
	pkt[2] ^= 0xFF // flip bits in the CRC field
	if _, err := unwrap(pkt); err == nil {
		t.Fatal("expected a CRC mismatch error")
	}
}

func TestBodyTypeRejectsShortBody(t *testing.T) {
	if _, err := bodyType([]byte{packetMarker}); err == nil {
		t.Fatal("expected an error for a body with no type byte")
	}
}

func TestWrapUnwrapRoundTrip(t *testing.T) {
	body := []byte{packetMarker, byte(PacketCommand), 42, 'a', 'b', 'c'}
	pkt := wrap(body)
	got, err := unwrap(pkt)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("unwrap(wrap(body)) = %v, want %v", got, body)
	}
}
