// Package tpkt implements the TPKT transport protocol (RFC 1006).
// TPKT provides a way to transport ISO 8073 (X.224) packets over TCP.
//
// TPKT Header format:
//
//	+--------+--------+--------+--------+
//	| version| reserved|    length      |
//	+--------+--------+--------+--------+
//	|             TPDU data             |
//	|               ...                 |
//	+-----------------------------------+
//
// version:  8 bits, must be 3
// reserved: 8 bits, must be 0
// length:   16 bits, big-endian, total packet length including header
package tpkt

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/bouncyball-git/gopher-rdp/util"
)

const (
	// Version is the TPKT protocol version (always 3)
	Version = 3

	// HeaderSize is the size of the TPKT header in bytes
	HeaderSize = 4

	// MaxPacketSize is the maximum TPKT packet size (64KB)
	MaxPacketSize = 65535
)

// PacketType distinguishes TPKT (slow-path) from fast-path framing.
type PacketType int

const (
	PacketTPKT     PacketType = iota // TPKT-framed slow-path PDU
	PacketFastPath                   // Fast-path output PDU
)

// Conn wraps a TCP connection and provides TPKT framing
type Conn struct {
	conn    net.Conn
	header  [HeaderSize]byte // reusable header buffer for reads
	readBuf []byte           // reusable payload buffer for ReadPacket (grows as needed)
	log     *slog.Logger
}

// NewConn creates a new TPKT connection wrapper
func NewConn(conn net.Conn, log *slog.Logger) *Conn {
	return &Conn{conn: conn, log: log}
}

// Read reads a complete TPKT packet, returning the payload (without header)
func (c *Conn) Read() ([]byte, error) {
	// Read TPKT header (4 bytes) into reusable buffer
	if _, err := io.ReadFull(c.conn, c.header[:]); err != nil {
		return nil, fmt.Errorf("failed to read TPKT header: %w", err)
	}

	// Validate version
	if c.header[0] != Version {
		return nil, fmt.Errorf("invalid TPKT version: got %d, expected %d", c.header[0], Version)
	}

	// Parse length (big-endian 16-bit)
	length := binary.BigEndian.Uint16(c.header[2:4])
	if length < HeaderSize {
		return nil, fmt.Errorf("invalid TPKT length: %d (minimum is %d)", length, HeaderSize)
	}

	// Read payload
	payloadSize := int(length) - HeaderSize
	if payloadSize == 0 {
		return []byte{}, nil
	}

	payload := make([]byte, payloadSize)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return nil, fmt.Errorf("failed to read TPKT payload: %w", err)
	}

	c.log.LogAttrs(context.Background(), slog.LevelDebug, "TPKT read", slog.Int("payloadLen", payloadSize))
	return payload, nil
}

// ReadPacket reads either a TPKT or fast-path PDU from the connection.
// It peeks at the first byte: 0x03 means TPKT (returns PacketTPKT with the
// X.224 payload and action=0x03), anything else is fast-path framing (returns
// PacketFastPath with the PDU body after the length field and the action byte,
// which contains security flags in bits 6-7).
//
// The returned slice is backed by an internal buffer and is only valid until
// the next call to ReadPacket.
func (c *Conn) ReadPacket() (PacketType, byte, []byte, error) {
	// Read first byte to determine framing type.
	if _, err := io.ReadFull(c.conn, c.header[0:1]); err != nil {
		return 0, 0, nil, fmt.Errorf("failed to read packet header: %w", err)
	}

	if c.header[0] == Version {
		// TPKT: read remaining 3 header bytes.
		if _, err := io.ReadFull(c.conn, c.header[1:4]); err != nil {
			return 0, 0, nil, fmt.Errorf("failed to read TPKT header: %w", err)
		}
		length := binary.BigEndian.Uint16(c.header[2:4])
		if length < HeaderSize {
			return 0, 0, nil, fmt.Errorf("invalid TPKT length: %d (minimum is %d)", length, HeaderSize)
		}
		payloadSize := int(length) - HeaderSize
		payload := c.growReadBuf(payloadSize)
		if payloadSize == 0 {
			return PacketTPKT, Version, payload, nil
		}
		if _, err := io.ReadFull(c.conn, payload); err != nil {
			return 0, 0, nil, fmt.Errorf("failed to read TPKT payload: %w", err)
		}
		c.log.LogAttrs(context.Background(), slog.LevelDebug, "TPKT packet", slog.Int("len", payloadSize))
		return PacketTPKT, Version, payload, nil
	}

	// Fast-path: first byte is the action/flags byte (already read).
	actionByte := c.header[0]

	// Read second byte for length.
	if _, err := io.ReadFull(c.conn, c.header[1:2]); err != nil {
		return 0, 0, nil, fmt.Errorf("failed to read fast-path length: %w", err)
	}

	var pduLen int
	var consumed int
	if c.header[1]&0x80 == 0 {
		// 1-byte length form: entire length in this byte.
		pduLen = int(c.header[1])
		consumed = 2
	} else {
		// 2-byte length form: high 7 bits of header[1] + next byte.
		if _, err := io.ReadFull(c.conn, c.header[2:3]); err != nil {
			return 0, 0, nil, fmt.Errorf("failed to read fast-path length byte 2: %w", err)
		}
		pduLen = int(c.header[1]&0x7F)<<8 | int(c.header[2])
		consumed = 3
	}

	remaining := pduLen - consumed
	if remaining < 0 {
		return 0, 0, nil, fmt.Errorf("invalid fast-path length: %d (consumed %d)", pduLen, consumed)
	}
	payload := c.growReadBuf(remaining)
	if remaining == 0 {
		return PacketFastPath, actionByte, payload, nil
	}
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return 0, 0, nil, fmt.Errorf("failed to read fast-path payload: %w", err)
	}
	c.log.LogAttrs(context.Background(), slog.LevelDebug, "fast-path packet", util.Hex2("action", actionByte), slog.Int("len", remaining))
	return PacketFastPath, actionByte, payload, nil
}

// growReadBuf returns a slice of length n backed by c.readBuf,
// growing the buffer if necessary. Zero allocations in steady state.
func (c *Conn) growReadBuf(n int) []byte {
	if cap(c.readBuf) < n {
		c.readBuf = make([]byte, n)
	}
	return c.readBuf[:n]
}

// Write writes data as a TPKT packet
func (c *Conn) Write(data []byte) error {
	totalLen := HeaderSize + len(data)
	if totalLen > MaxPacketSize {
		return fmt.Errorf("packet too large: %d bytes (max %d)", totalLen, MaxPacketSize)
	}

	// Build packet: header + data
	packet := make([]byte, totalLen)
	packet[0] = Version
	packet[1] = 0 // reserved
	binary.BigEndian.PutUint16(packet[2:4], uint16(totalLen))
	copy(packet[HeaderSize:], data)

	// Write complete packet in a single syscall
	if _, err := c.conn.Write(packet); err != nil {
		return fmt.Errorf("failed to write TPKT packet: %w", err)
	}

	c.log.LogAttrs(context.Background(), slog.LevelDebug, "TPKT write", slog.Int("payloadLen", len(data)))
	return nil
}

// WriteDirect writes a pre-assembled TPKT packet (header already included)
// directly to the TCP connection. No allocation, no copy.
func (c *Conn) WriteDirect(packet []byte) error {
	if _, err := c.conn.Write(packet); err != nil {
		return fmt.Errorf("failed to write TPKT packet: %w", err)
	}
	return nil
}

// Close closes the underlying connection
func (c *Conn) Close() error {
	return c.conn.Close()
}

// LocalAddr returns the local network address
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// GetTCPConn returns the underlying TCP connection for TLS upgrade
func (c *Conn) GetTCPConn() net.Conn {
	return c.conn
}

// SetTCPConn replaces the underlying connection (used after TLS upgrade)
func (c *Conn) SetTCPConn(conn net.Conn) {
	c.conn = conn
}
