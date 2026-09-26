// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package battleye implements the BattlEye RCon protocol natively (D30):
// no external tools (bercon-cli, dayz_restart) and no external RCon
// library. This file holds the pure wire-format encode/decode logic; it has
// no network dependency, so it is fully unit-tested (§C15).
//
// Wire format (all integers little-endian):
//
//	'B' 'E' <CRC32 of everything from 0xFF on> 0xFF <packet type> [payload]
//
// Packet types: 0x00 Login, 0x01 Command, 0x02 (unsolicited) Server Message.
package battleye

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

// PacketType identifies the third byte of a BattlEye RCon packet.
type PacketType byte

const (
	PacketLogin   PacketType = 0x00
	PacketCommand PacketType = 0x01
	PacketMessage PacketType = 0x02
)

const packetMarker = 0xFF

// crc computes the packet CRC32 over the bytes starting at the 0xFF marker
// (i.e. everything except the 'B','E' header and the CRC field itself).
func crc(body []byte) uint32 {
	return crc32.ChecksumIEEE(body)
}

// wrap builds a full packet from its body (everything from the 0xFF marker
// onward, including the packet type byte).
func wrap(body []byte) []byte {
	buf := make([]byte, 2+4+len(body))
	buf[0], buf[1] = 'B', 'E'
	binary.LittleEndian.PutUint32(buf[2:6], crc(body))
	copy(buf[6:], body)
	return buf
}

// unwrap validates the 'BE' header and CRC and returns the body (from the
// 0xFF marker onward).
func unwrap(packet []byte) ([]byte, error) {
	if len(packet) < 7 {
		return nil, fmt.Errorf("battleye: packet too short (%d bytes)", len(packet))
	}
	if packet[0] != 'B' || packet[1] != 'E' {
		return nil, fmt.Errorf("battleye: bad header %q", packet[0:2])
	}
	body := packet[6:]
	if body[0] != packetMarker {
		return nil, fmt.Errorf("battleye: missing 0xFF marker")
	}
	want := binary.LittleEndian.Uint32(packet[2:6])
	if got := crc(body); got != want {
		return nil, fmt.Errorf("battleye: CRC mismatch (got %#x, want %#x)", got, want)
	}
	return body, nil
}

// Type returns the packet type of a validated packet body (as returned by
// unwrap), i.e. body[1].
func bodyType(body []byte) (PacketType, error) {
	if len(body) < 2 {
		return 0, fmt.Errorf("battleye: packet body too short")
	}
	return PacketType(body[1]), nil
}

// EncodeLogin builds a Login packet (client -> server).
func EncodeLogin(password string) []byte {
	body := append([]byte{packetMarker, byte(PacketLogin)}, []byte(password)...)
	return wrap(body)
}

// DecodeLoginResponse parses a server Login response packet and reports
// whether the login succeeded.
func DecodeLoginResponse(packet []byte) (bool, error) {
	body, err := unwrap(packet)
	if err != nil {
		return false, err
	}
	t, err := bodyType(body)
	if err != nil {
		return false, err
	}
	if t != PacketLogin {
		return false, fmt.Errorf("battleye: expected a Login packet, got type %#x", t)
	}
	if len(body) < 3 {
		return false, fmt.Errorf("battleye: Login response missing result byte")
	}
	return body[2] == 0x01, nil
}

// EncodeCommand builds a Command packet (client -> server). seq wraps
// modulo 256 and must match the sequence number the response is correlated
// with.
func EncodeCommand(seq byte, command string) []byte {
	body := append([]byte{packetMarker, byte(PacketCommand), seq}, []byte(command)...)
	return wrap(body)
}

// MultipartInfo describes a Command response that was split across several
// UDP packets because it did not fit into one.
type MultipartInfo struct {
	Total int
	Index int
}

// DecodeCommandResponse parses a server Command response packet. multipart
// is nil for a single-packet response.
func DecodeCommandResponse(packet []byte) (seq byte, multipart *MultipartInfo, payload []byte, err error) {
	body, err := unwrap(packet)
	if err != nil {
		return 0, nil, nil, err
	}
	t, err := bodyType(body)
	if err != nil {
		return 0, nil, nil, err
	}
	if t != PacketCommand {
		return 0, nil, nil, fmt.Errorf("battleye: expected a Command packet, got type %#x", t)
	}
	if len(body) < 3 {
		return 0, nil, nil, fmt.Errorf("battleye: Command response missing sequence number")
	}
	seq = body[2]
	rest := body[3:]
	if len(rest) >= 3 && rest[0] == 0x00 {
		multipart = &MultipartInfo{Total: int(rest[1]), Index: int(rest[2])}
		rest = rest[3:]
	}
	return seq, multipart, rest, nil
}

// DecodeServerMessage parses an unsolicited Server Message packet
// (connect/GUID/chat/kick notifications).
func DecodeServerMessage(packet []byte) (seq byte, message []byte, err error) {
	body, err := unwrap(packet)
	if err != nil {
		return 0, nil, err
	}
	t, err := bodyType(body)
	if err != nil {
		return 0, nil, err
	}
	if t != PacketMessage {
		return 0, nil, fmt.Errorf("battleye: expected a Message packet, got type %#x", t)
	}
	if len(body) < 3 {
		return 0, nil, fmt.Errorf("battleye: Message packet missing sequence number")
	}
	return body[2], body[3:], nil
}

// EncodeMessageAck builds the empty acknowledgement the client must send
// back for every Server Message, or the server keeps resending it.
func EncodeMessageAck(seq byte) []byte {
	body := []byte{packetMarker, byte(PacketMessage), seq}
	return wrap(body)
}

// PeekType returns the packet type of any valid packet, without knowing its
// kind in advance. Used by the client's read loop to dispatch.
func PeekType(packet []byte) (PacketType, error) {
	body, err := unwrap(packet)
	if err != nil {
		return 0, err
	}
	return bodyType(body)
}
