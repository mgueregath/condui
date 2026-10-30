package mcs

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
)

// PDUType returns the MCS domain PDU type from the first byte (top 6 bits << 2).
func PDUType(data []byte) byte {
	if len(data) == 0 {
		return 0
	}
	return data[0] & 0xFC
}

// DecodeDisconnectProviderUltimatum parses an MCS Disconnect Provider Ultimatum.
// Returns the reason code (0=domain-not-hierarchical, 1=user-requested,
// 2=token-purged, 3=provider-initiated).
// The DPU is 2 bytes: byte0[1:0] || byte1[7] = 3-bit reason.
func DecodeDisconnectProviderUltimatum(log *slog.Logger, data []byte) (reason int, err error) {
	if len(data) < 2 {
		return 0, fmt.Errorf("disconnect provider ultimatum too short: %d bytes", len(data))
	}
	reason = int(data[0]&0x03)<<1 | int(data[1]>>7)
	log.LogAttrs(context.Background(), slog.LevelDebug, "MCS Disconnect Provider", slog.Int("reason", reason))
	return reason, nil
}

// DecodeSendDataIndication parses an MCS Send Data Indication PDU (PER-encoded).
// Returns the channel ID and user data payload.
//
// PER layout:
//   - Byte 0: type (0x68, top 6 bits = 011010)
//   - Bytes 1-2: initiator (uint16 BE, offset by 1001) — skipped
//   - Bytes 3-4: channelID (uint16 BE)
//   - Byte 5: dataPriority + segmentation
//   - Remaining: PER length determinant + user data
func DecodeSendDataIndication(log *slog.Logger, data []byte) (channelID uint16, userData []byte, err error) {
	if len(data) < 7 {
		return 0, nil, fmt.Errorf("send data indication too short: %d bytes", len(data))
	}

	// Verify PDU type (top 6 bits)
	if data[0]>>2 != DomainMCSPDUSendDataIndication>>2 {
		return 0, nil, fmt.Errorf("expected Send Data Indication (0x68), got 0x%02X", data[0])
	}

	// Skip initiator (bytes 1-2), read channel ID (bytes 3-4)
	channelID = binary.BigEndian.Uint16(data[3:5])

	// Skip dataPriority + segmentation (byte 5)
	off := 6

	// Read PER length determinant
	if off >= len(data) {
		return 0, nil, fmt.Errorf("send data indication truncated at length")
	}

	length, bytesRead, err := readPERLength(data[off:])
	if err != nil {
		return 0, nil, fmt.Errorf("reading PER length: %w", err)
	}
	off += bytesRead

	if off+length > len(data) {
		return 0, nil, fmt.Errorf("send data indication truncated: need %d bytes, have %d", off+length, len(data))
	}

	log.LogAttrs(context.Background(), slog.LevelDebug, "MCS Send Data Indication", slog.Int("channelID", int(channelID)), slog.Int("userDataLen", length))
	return channelID, data[off : off+length], nil
}

// readPERLength reads a PER length determinant and returns the length
// and the number of bytes consumed.
func readPERLength(data []byte) (int, int, error) {
	if len(data) < 1 {
		return 0, 0, fmt.Errorf("PER length: no data")
	}

	b0 := data[0]
	if b0 < 0x80 {
		// 1-byte length
		return int(b0), 1, nil
	}

	// 2-byte length: (b0 & 0x3F) << 8 | b1
	if len(data) < 2 {
		return 0, 0, fmt.Errorf("PER length: need 2 bytes, have 1")
	}
	length := int(b0&0x3F)<<8 | int(data[1])
	return length, 2, nil
}
