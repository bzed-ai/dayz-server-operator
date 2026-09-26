// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package battleye

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// KeepAliveInterval is how often the client sends an empty Command packet
// to keep the RCon connection alive. The server times the connection out
// after ~45s of silence; this stays comfortably under that.
const KeepAliveInterval = 30 * time.Second

// ReadBufferSize is large enough for any single BattlEye RCon UDP packet.
const ReadBufferSize = 4096

// ErrLoginFailed is returned by Dial when the server rejects the password.
var ErrLoginFailed = errors.New("battleye: login failed")

// ErrClosed is returned by Command when the client has been closed.
var ErrClosed = errors.New("battleye: client closed")

type commandResult struct {
	payload []byte
	err     error
}

type multipartAssembly struct {
	total  int
	chunks [][]byte
	got    int
}

// Client is a connected BattlEye RCon session.
type Client struct {
	conn *net.UDPConn

	mu        sync.Mutex
	nextSeq   byte
	pending   map[byte]chan commandResult
	multipart map[byte]*multipartAssembly
	closed    bool

	events chan Event

	keepAliveInterval time.Duration
	loginTimeout      time.Duration

	stop chan struct{}
	wg   sync.WaitGroup
}

// Option customises a Client created by Dial.
type Option func(*Client)

// WithKeepAliveInterval overrides KeepAliveInterval, mainly for tests.
func WithKeepAliveInterval(d time.Duration) Option {
	return func(c *Client) { c.keepAliveInterval = d }
}

// WithLoginTimeout overrides the 5s default login handshake timeout, mainly
// for tests.
func WithLoginTimeout(d time.Duration) Option {
	return func(c *Client) { c.loginTimeout = d }
}

// Dial connects to a BattlEye RCon endpoint and performs the login
// handshake. On success it starts the background read and keep-alive
// loops; the caller must call Close when done.
func Dial(addr, password string, opts ...Option) (*Client, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("battleye: resolve %s: %w", addr, err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("battleye: dial %s: %w", addr, err)
	}

	c := &Client{
		conn:              conn,
		pending:           map[byte]chan commandResult{},
		multipart:         map[byte]*multipartAssembly{},
		events:            make(chan Event, 64),
		keepAliveInterval: KeepAliveInterval,
		loginTimeout:      5 * time.Second,
		stop:              make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}

	if err := c.login(password); err != nil {
		conn.Close()
		return nil, err
	}

	c.wg.Add(2)
	go c.readLoop()
	go c.keepAliveLoop()
	return c, nil
}

func (c *Client) login(password string) error {
	if err := c.conn.SetReadDeadline(time.Now().Add(c.loginTimeout)); err != nil {
		return fmt.Errorf("battleye: set read deadline: %w", err)
	}
	defer c.conn.SetReadDeadline(time.Time{}) //nolint:errcheck // best-effort deadline reset

	if _, err := c.conn.Write(EncodeLogin(password)); err != nil {
		return fmt.Errorf("battleye: send login: %w", err)
	}

	buf := make([]byte, ReadBufferSize)
	n, err := c.conn.Read(buf)
	if err != nil {
		return fmt.Errorf("battleye: read login response: %w", err)
	}
	ok, err := DecodeLoginResponse(buf[:n])
	if err != nil {
		return fmt.Errorf("battleye: decode login response: %w", err)
	}
	if !ok {
		return ErrLoginFailed
	}
	return nil
}

// Events returns the channel of parsed unsolicited Server Messages
// (connect/GUID/chat/kick, FR-22). The client acknowledges each one
// automatically; callers only observe them.
func (c *Client) Events() <-chan Event {
	return c.events
}

// Command sends an RCon command and waits for its (possibly reassembled)
// response, or ctx's deadline/cancellation.
func (c *Client) Command(ctx context.Context, command string) (string, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return "", ErrClosed
	}
	seq := c.nextSeq
	c.nextSeq++
	result := make(chan commandResult, 1)
	c.pending[seq] = result
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, seq)
		delete(c.multipart, seq)
		c.mu.Unlock()
	}()

	if _, err := c.conn.Write(EncodeCommand(seq, command)); err != nil {
		return "", fmt.Errorf("battleye: send command: %w", err)
	}

	select {
	case res := <-result:
		if res.err != nil {
			return "", res.err
		}
		return string(res.payload), nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-c.stop:
		return "", ErrClosed
	}
}

// Close stops the background loops and closes the UDP socket.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	close(c.stop)
	err := c.conn.Close()
	c.wg.Wait()
	close(c.events)
	return err
}

func (c *Client) readLoop() {
	defer c.wg.Done()
	buf := make([]byte, ReadBufferSize)
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			select {
			case <-c.stop:
				return
			default:
			}
			c.failAllPending(fmt.Errorf("battleye: connection lost: %w", err))
			return
		}
		c.handlePacket(buf[:n])
	}
}

func (c *Client) handlePacket(packet []byte) {
	typ, err := PeekType(packet)
	if err != nil {
		return // malformed/corrupt packet: drop it, never crash the loop.
	}
	switch typ {
	case PacketCommand:
		c.handleCommandResponse(packet)
	case PacketMessage:
		c.handleServerMessage(packet)
	default:
		// Login responses after the handshake are unexpected; ignore.
	}
}

func (c *Client) handleCommandResponse(packet []byte) {
	seq, multipart, payload, err := DecodeCommandResponse(packet)
	if err != nil {
		return
	}
	// packet aliases readLoop's shared receive buffer, which the next
	// c.conn.Read overwrites as soon as this handler returns. Anything
	// kept beyond this call (multipart reassembly, the queued result) must
	// be copied out first.
	payload = append([]byte(nil), payload...)

	c.mu.Lock()
	ch, ok := c.pending[seq]
	if !ok {
		c.mu.Unlock()
		return // no one is waiting (e.g. a keep-alive), drop it.
	}

	var complete []byte
	if multipart == nil {
		complete = payload
	} else {
		asm, exists := c.multipart[seq]
		if !exists {
			asm = &multipartAssembly{total: multipart.Total, chunks: make([][]byte, multipart.Total)}
			c.multipart[seq] = asm
		}
		if multipart.Index < 0 || multipart.Index >= asm.total {
			c.mu.Unlock()
			return
		}
		if asm.chunks[multipart.Index] == nil {
			asm.chunks[multipart.Index] = payload
			asm.got++
		}
		if asm.got < asm.total {
			c.mu.Unlock()
			return
		}
		for _, chunk := range asm.chunks {
			complete = append(complete, chunk...)
		}
	}
	delete(c.pending, seq)
	c.mu.Unlock()

	ch <- commandResult{payload: complete}
}

func (c *Client) handleServerMessage(packet []byte) {
	seq, payload, err := DecodeServerMessage(packet)
	if err != nil {
		return
	}
	if _, err := c.conn.Write(EncodeMessageAck(seq)); err != nil {
		return
	}
	event := ParseEvent(string(payload))
	select {
	case c.events <- event:
	default:
		// Event channel is full; drop rather than block the read loop.
	}
}

func (c *Client) failAllPending(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for seq, ch := range c.pending {
		ch <- commandResult{err: err}
		delete(c.pending, seq)
	}
}

func (c *Client) keepAliveLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.keepAliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			if c.closed {
				c.mu.Unlock()
				return
			}
			seq := c.nextSeq
			c.nextSeq++
			c.mu.Unlock()
			_, _ = c.conn.Write(EncodeCommand(seq, ""))
		case <-c.stop:
			return
		}
	}
}
