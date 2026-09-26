// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package a2s

import (
	"fmt"
	"net"
	"time"
)

// ReadBufferSize is large enough for any A2S_INFO reply DayZ sends
// (typically well under 512 bytes; no Extra Data Flag fields are parsed,
// but the buffer must still hold them).
const ReadBufferSize = 4096

// Query sends one A2S_INFO request to addr ("host:port", the game's query
// port) and returns the parsed response. It always performs the protocol's
// two-round-trip challenge handshake (send, get a challenge, resend,
// get the info): modern Source-engine servers require it, and the plan's
// HealthTimeout (10s) already accounts for it.
func Query(addr string, timeout time.Duration) (InfoResponse, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return InfoResponse{}, fmt.Errorf("a2s: resolve %s: %w", addr, err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return InfoResponse{}, fmt.Errorf("a2s: dial %s: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()

	deadline := time.Now().Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return InfoResponse{}, fmt.Errorf("a2s: set deadline: %w", err)
	}

	packet, err := roundTrip(conn, InfoRequest(nil))
	if err != nil {
		return InfoResponse{}, err
	}

	kind, respBody, err := PacketKind(packet)
	if err != nil {
		return InfoResponse{}, err
	}

	switch kind {
	case responseKindInfo:
		return ParseInfoResponse(respBody)
	case responseKindChallenge:
		challenge, err := ChallengeNumber(respBody)
		if err != nil {
			return InfoResponse{}, err
		}
		packet, err = roundTrip(conn, InfoRequest(challenge))
		if err != nil {
			return InfoResponse{}, err
		}
		kind, respBody, err := PacketKind(packet)
		if err != nil {
			return InfoResponse{}, err
		}
		if kind != responseKindInfo {
			return InfoResponse{}, fmt.Errorf("a2s: expected an info response after the challenge, got type %#x", kind)
		}
		return ParseInfoResponse(respBody)
	default:
		return InfoResponse{}, fmt.Errorf("a2s: unexpected response type %#x", kind)
	}
}

func roundTrip(conn *net.UDPConn, request []byte) ([]byte, error) {
	if _, err := conn.Write(request); err != nil {
		return nil, fmt.Errorf("a2s: send request: %w", err)
	}
	buf := make([]byte, ReadBufferSize)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("a2s: read response: %w", err)
	}
	return buf[:n], nil
}
