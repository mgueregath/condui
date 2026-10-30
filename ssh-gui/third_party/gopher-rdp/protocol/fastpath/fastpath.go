// Package fastpath decodes Fast-Path Output Update PDUs (MS-RDPBCGR 2.2.9.1.2).
//
// Fast-path packets bypass TPKT/X.224/MCS/ShareControl/ShareData and use a
// compact framing: 1-byte header + 1-2 byte length, followed by an array of
// TS_FP_UPDATE entries.
package fastpath

import (
	"context"
	"encoding/binary"
	"log/slog"
)

// Update type codes (bits 0-3 of updateHeader).
const (
	UpdateOrders      = 0x0
	UpdateBitmap      = 0x1
	UpdatePalette     = 0x2
	UpdateSynchronize = 0x3
	UpdateSurfCmds    = 0x4
	UpdatePtrNull     = 0x5
	UpdatePtrDefault  = 0x6
	UpdatePtrPosition = 0x8
	UpdatePtrColor    = 0x9
	UpdatePtrCached   = 0xA
	UpdatePtrNew      = 0xB
	UpdatePtrLarge    = 0xC
)

// Fragmentation flags (bits 4-5 of updateHeader).
const (
	FragSingle = 0x0
	FragLast   = 0x1
	FragFirst  = 0x2
	FragNext   = 0x3
)

// Update is a single entry from the fpOutputUpdates array in a fast-path PDU.
type Update struct {
	Code             byte   // bits 0-3 of updateHeader
	Frag             byte   // bits 4-5 of updateHeader
	CompressionFlags byte   // compressionFlags byte (0 when not compressed)
	Data             []byte // sub-slice of input buffer (no copy)
}

// DecodeUpdates parses the fpOutputUpdates array from a fast-path PDU body
// (after the framing header/length have been stripped by tpkt.ReadPacket).
//
// Each TS_FP_UPDATE:
//   - updateHeader (1 byte): code = bits 0-3, frag = bits 4-5, compression = bits 6-7
//   - compressionFlags (1 byte): present only when compression bits == 0x2
//   - size (2 bytes LE): length of updateData
//   - updateData: size bytes
//
// Data slices reference the input buffer (no copy).
func DecodeUpdates(log *slog.Logger, data []byte) ([]Update, error) {
	// Stack-backed array avoids heap allocation for the common case (≤8 updates).
	var buf [8]Update
	updates := buf[:0]
	off := 0

	for off < len(data) {
		hdr := data[off]
		off++

		code := hdr & 0x0F
		frag := (hdr >> 4) & 0x03
		compression := (hdr >> 6) & 0x03

		// compressionFlags byte (present when FASTPATH_OUTPUT_COMPRESSION_USED bit is set)
		var compressionFlags byte
		if compression&0x2 != 0 {
			if off >= len(data) {
				return nil, errShortUpdate
			}
			compressionFlags = data[off]
			off++
		}

		// size (2 bytes LE)
		if off+2 > len(data) {
			return nil, errShortUpdate
		}
		size := int(binary.LittleEndian.Uint16(data[off : off+2]))
		off += 2

		// updateData
		if off+size > len(data) {
			return nil, errShortUpdate
		}
		updates = append(updates, Update{
			Code:             code,
			Frag:             frag,
			CompressionFlags: compressionFlags,
			Data:             data[off : off+size],
		})
		off += size
	}

	log.LogAttrs(context.Background(), slog.LevelDebug, "fast-path updates decoded", slog.Int("count", len(updates)))
	return updates, nil
}

type shortErr string

func (e shortErr) Error() string { return string(e) }

var errShortUpdate = shortErr("fast-path update data too short")
