// Package zgfx implements the RDP8 bulk compression codec (MS-RDPEGFX 3.1).
//
// All RDPGFX data from the server arrives wrapped in RDP_SEGMENTED_DATA.
// This package handles both single-segment and multipart messages, applying
// Huffman + LZ77 decompression with a persistent 2.5MB history buffer.
//
// Implements MS-RDPEGFX 3.1.
package zgfx

import (
	"context"
	"encoding/binary"
	"log/slog"
)

const historySize = 2_500_000

// Decompressor maintains ZGFX decompression state including the history buffer.
type Decompressor struct {
	history    [historySize]byte
	historyIdx uint32

	// Bit reader state (reset per segment)
	bits       []byte
	bitPos     uint32 // current bit position in the segment
	bitsRemain uint32 // total available bits
	log        *slog.Logger
}

// Reset reinitializes the history buffer to all zeros and resets the write
// position, as required by MS-RDPEGFX 3.1.8.1.1 when a Reset Graphics PDU
// is processed.
func (d *Decompressor) Reset() {
	clear(d.history[:])
	d.historyIdx = 0
}

// Decompress decompresses RDP_SEGMENTED_DATA from src into dst.
// dst is a reusable buffer that is grown as needed; the (possibly reallocated)
// slice is returned for reuse by the caller.

// SetLogger sets the logger for debug output.
func (d *Decompressor) SetLogger(log *slog.Logger) { d.log = log }

func (d *Decompressor) Decompress(dst, src []byte) ([]byte, error) {
	if len(src) < 1 {
		return dst, errShortInput
	}

	descriptor := src[0]
	switch descriptor {
	case 0xE0: // Single segment
		if len(src) < 2 {
			return dst, errShortInput
		}
		if d.log != nil {
			d.log.LogAttrs(context.Background(), slog.LevelDebug, "zgfx decompress single", slog.Int("srcLen", len(src)))
		}
		return d.decompressSegment(dst, src[1:])

	case 0xE1: // Multipart
		if len(src) < 7 {
			return dst, errShortInput
		}
		segCount := binary.LittleEndian.Uint16(src[1:3])
		uncompSize := binary.LittleEndian.Uint32(src[3:7])
		if d.log != nil {
			d.log.LogAttrs(context.Background(), slog.LevelDebug, "zgfx decompress multi", slog.Int("segments", int(segCount)), slog.Int("uncompSize", int(uncompSize)), slog.Int("srcLen", len(src)))
		}
		off := 7

		// Ensure dst capacity
		need := int(uncompSize)
		if cap(dst) >= need {
			dst = dst[:0]
		} else {
			dst = make([]byte, 0, need)
		}

		for i := 0; i < int(segCount); i++ {
			if off+4 > len(src) {
				return dst, errTruncated
			}
			segSize := int(binary.LittleEndian.Uint32(src[off : off+4]))
			off += 4
			if off+segSize > len(src) {
				return dst, errTruncated
			}
			var err error
			dst, err = d.decompressSegment(dst, src[off:off+segSize])
			if err != nil {
				return dst, err
			}
			off += segSize
		}
		return dst, nil

	default:
		return dst, errBadDescriptor
	}
}

func (d *Decompressor) decompressSegment(dst, seg []byte) ([]byte, error) {
	if len(seg) < 1 {
		return dst, errShortInput
	}

	flags := seg[0]
	if flags&0x20 == 0 {
		// Uncompressed segment — copy directly and add to history
		data := seg[1:]
		dst = append(dst, data...)
		d.historyWrite(data)
		return dst, nil
	}

	// Compressed segment: last byte = number of padding bits
	if len(seg) < 2 {
		return dst, errShortInput
	}
	lastByte := seg[len(seg)-1]
	d.bits = seg[1 : len(seg)-1] // strip flags byte and tail padding byte
	d.bitPos = 0
	d.bitsRemain = uint32(len(d.bits))*8 - uint32(lastByte)

	// Use direct indexing when dst has sufficient capacity (common after first frame).
	// pos tracks the write position within dst; we re-slice at the end.
	pos := len(dst)

	for d.bitPos < d.bitsRemain {
		// Huffman decode via prefix lookup
		token, err := d.readToken()
		if err != nil {
			dst = dst[:pos]
			return dst, err
		}

		if token.tokenType == 0 {
			// Literal byte
			val, err := d.readBits(int(token.valueBits))
			if err != nil {
				dst = dst[:pos]
				return dst, err
			}
			b := byte(token.valueBase + val)
			if pos < cap(dst) {
				dst = dst[:pos+1]
				dst[pos] = b
			} else {
				dst = append(dst, b)
			}
			pos++
			d.history[d.historyIdx] = b
			d.historyIdx++
			if d.historyIdx >= historySize {
				d.historyIdx = 0
			}
		} else {
			// Match token
			extra, err := d.readBits(int(token.valueBits))
			if err != nil {
				dst = dst[:pos]
				return dst, err
			}
			distance := int(token.valueBase) + int(extra)

			if distance == 0 {
				// Unencoded run: 15-bit count, byte-align, then raw bytes
				count, err := d.readBits(15)
				if err != nil {
					dst = dst[:pos]
					return dst, err
				}
				d.byteAlign()
				// Read raw bytes directly from bit stream
				n := int(count)
				for i := 0; i < n; i++ {
					val, err := d.readBits(8)
					if err != nil {
						dst = dst[:pos]
						return dst, err
					}
					b := byte(val)
					if pos < cap(dst) {
						dst = dst[:pos+1]
						dst[pos] = b
					} else {
						dst = append(dst, b)
					}
					pos++
					d.history[d.historyIdx] = b
					d.historyIdx++
					if d.historyIdx >= historySize {
						d.historyIdx = 0
					}
				}
			} else {
				// LZ77 match: decode length, copy from history
				length, err := d.readMatchLength()
				if err != nil {
					dst = dst[:pos]
					return dst, err
				}

				// Copy from history ring buffer (byte-at-a-time for overlap)
				srcIdx := int(d.historyIdx) - distance
				if srcIdx < 0 {
					srcIdx += historySize
				}
				for i := 0; i < length; i++ {
					b := d.history[srcIdx%historySize]
					if pos < cap(dst) {
						dst = dst[:pos+1]
						dst[pos] = b
					} else {
						dst = append(dst, b)
					}
					pos++
					d.history[d.historyIdx] = b
					d.historyIdx++
					if d.historyIdx >= historySize {
						d.historyIdx = 0
					}
					srcIdx++
				}
			}
		}
	}

	dst = dst[:pos]
	return dst, nil
}

// historyWrite adds data to the history ring buffer.
func (d *Decompressor) historyWrite(data []byte) {
	for _, b := range data {
		d.history[d.historyIdx] = b
		d.historyIdx++
		if d.historyIdx >= historySize {
			d.historyIdx = 0
		}
	}
}

// readMatchLength reads a variable-length match length.
// Read 1 bit; if 0 → length=3.
// Otherwise count=4, extra=2, then for each additional 1-bit: count*=2, extra++.
// Finally read extra bits and add to count.
func (d *Decompressor) readMatchLength() (int, error) {
	bit, err := d.readBits(1)
	if err != nil {
		return 0, err
	}
	if bit == 0 {
		return 3, nil
	}

	count := 4
	extra := 2
	for {
		bit, err := d.readBits(1)
		if err != nil {
			return 0, err
		}
		if bit == 0 {
			break
		}
		count *= 2
		extra++
	}

	extraVal, err := d.readBits(extra)
	if err != nil {
		return 0, err
	}
	return count + int(extraVal), nil
}

// readBits reads n bits from the compressed stream (MSB first).
func (d *Decompressor) readBits(n int) (uint32, error) {
	if d.bitPos+uint32(n) > d.bitsRemain {
		return 0, errBitsExhausted
	}

	var val uint32
	for i := 0; i < n; i++ {
		byteIdx := d.bitPos >> 3
		bitIdx := 7 - (d.bitPos & 7) // MSB first
		val = (val << 1) | uint32((d.bits[byteIdx]>>bitIdx)&1)
		d.bitPos++
	}
	return val, nil
}

// byteAlign advances bitPos to the next byte boundary.
func (d *Decompressor) byteAlign() {
	rem := d.bitPos & 7
	if rem != 0 {
		d.bitPos += 8 - rem
	}
}

// readToken reads a Huffman token using the prefix lookup table.
func (d *Decompressor) readToken() (zgfxToken, error) {
	if d.bitPos >= d.bitsRemain {
		return zgfxToken{}, errBitsExhausted
	}

	// Peek at up to 9 bits for O(1) lookup
	avail := int(d.bitsRemain - d.bitPos)
	lookBits := 9
	if avail < lookBits {
		lookBits = avail
	}

	var prefix uint32
	for i := 0; i < lookBits; i++ {
		byteIdx := (d.bitPos + uint32(i)) >> 3
		bitIdx := 7 - ((d.bitPos + uint32(i)) & 7)
		prefix = (prefix << 1) | uint32((d.bits[byteIdx]>>bitIdx)&1)
	}
	// Left-justify to 9 bits
	prefix <<= uint(9 - lookBits)

	entry := prefixLookup[prefix]
	if entry.prefixBits == 0 {
		return zgfxToken{}, errBadToken
	}

	// Advance past the prefix bits
	d.bitPos += uint32(entry.prefixBits)
	return entry, nil
}

// zgfxToken represents one entry in the ZGFX Huffman table.
type zgfxToken struct {
	prefixBits uint8  // number of bits in the prefix code
	tokenType  uint8  // 0 = literal, 1 = match
	valueBase  uint32 // base value for literal byte or match distance
	valueBits  uint8  // extra bits to read after prefix
}

// ZGFX Huffman token table (MS-RDPEGFX 3.1.8.1.1).
// Fields: {prefixBits, tokenType, valueBase, valueBits}
var tokenTable = [40]zgfxToken{
	// idx  prefix           type  description
	{1, 0, 0x00, 8},          // 0:  0          literal (8 extra bits → any byte)
	{5, 1, 0, 5},             // 1:  10001      match, distance base=0
	{5, 1, 32, 7},            // 2:  10010      match, distance base=32
	{5, 1, 160, 9},           // 3:  10011      match, distance base=160
	{5, 1, 672, 10},          // 4:  10100      match, distance base=672
	{5, 1, 1696, 12},         // 5:  10101      match, distance base=1696
	{5, 0, 0x00, 0},          // 6:  11000      literal byte 0x00
	{5, 0, 0x01, 0},          // 7:  11001      literal byte 0x01
	{6, 1, 5792, 14},         // 8:  101100     match, distance base=5792
	{6, 1, 22176, 15},        // 9:  101101     match, distance base=22176
	{6, 0, 0x02, 0},          // 10: 110100     literal byte 0x02
	{6, 0, 0x03, 0},          // 11: 110101     literal byte 0x03
	{6, 0, 0xFF, 0},          // 12: 110110     literal byte 0xFF
	{7, 1, 54944, 18},        // 13: 1011100    match, distance base=54944
	{7, 1, 317088, 20},       // 14: 1011101    match, distance base=317088
	{7, 0, 0x04, 0},          // 15: 1101110    literal byte 0x04
	{7, 0, 0x05, 0},          // 16: 1101111    literal byte 0x05
	{7, 0, 0x06, 0},          // 17: 1110000    literal byte 0x06
	{7, 0, 0x07, 0},          // 18: 1110001    literal byte 0x07
	{7, 0, 0x08, 0},          // 19: 1110010    literal byte 0x08
	{7, 0, 0x09, 0},          // 20: 1110011    literal byte 0x09
	{7, 0, 0x0A, 0},          // 21: 1110100    literal byte 0x0A
	{7, 0, 0x0B, 0},          // 22: 1110101    literal byte 0x0B
	{7, 0, 0x3A, 0},          // 23: 1110110    literal byte 0x3A
	{7, 0, 0x3B, 0},          // 24: 1110111    literal byte 0x3B
	{7, 0, 0x3C, 0},          // 25: 1111000    literal byte 0x3C
	{7, 0, 0x3D, 0},          // 26: 1111001    literal byte 0x3D
	{7, 0, 0x3E, 0},          // 27: 1111010    literal byte 0x3E
	{7, 0, 0x3F, 0},          // 28: 1111011    literal byte 0x3F
	{7, 0, 0x40, 0},          // 29: 1111100    literal byte 0x40
	{7, 0, 0x80, 0},          // 30: 1111101    literal byte 0x80
	{8, 1, 1365664, 20},      // 31: 10111100   match, distance base=1365664
	{8, 1, 2414240, 21},      // 32: 10111101   match, distance base=2414240
	{8, 0, 0x0C, 0},          // 33: 11111100   literal byte 0x0C
	{8, 0, 0x38, 0},          // 34: 11111101   literal byte 0x38
	{8, 0, 0x39, 0},          // 35: 11111110   literal byte 0x39
	{8, 0, 0x66, 0},          // 36: 11111111   literal byte 0x66
	{9, 1, 4511392, 22},      // 37: 101111100  match, distance base=4511392
	{9, 1, 8705696, 23},      // 38: 101111101  match, distance base=8705696
	{9, 1, 17094304, 24},     // 39: 101111110  match, distance base=17094304
}

// Prefix codes matching ZGFX_TOKEN_TABLE.prefixCode values.
var tokenPrefixCodes = [40]uint32{
	0,   // 0:  0
	17,  // 1:  10001
	18,  // 2:  10010
	19,  // 3:  10011
	20,  // 4:  10100
	21,  // 5:  10101
	24,  // 6:  11000
	25,  // 7:  11001
	44,  // 8:  101100
	45,  // 9:  101101
	52,  // 10: 110100
	53,  // 11: 110101
	54,  // 12: 110110
	92,  // 13: 1011100
	93,  // 14: 1011101
	110, // 15: 1101110
	111, // 16: 1101111
	112, // 17: 1110000
	113, // 18: 1110001
	114, // 19: 1110010
	115, // 20: 1110011
	116, // 21: 1110100
	117, // 22: 1110101
	118, // 23: 1110110
	119, // 24: 1110111
	120, // 25: 1111000
	121, // 26: 1111001
	122, // 27: 1111010
	123, // 28: 1111011
	124, // 29: 1111100
	125, // 30: 1111101
	188, // 31: 10111100
	189, // 32: 10111101
	252, // 33: 11111100
	253, // 34: 11111101
	254, // 35: 11111110
	255, // 36: 11111111
	380, // 37: 101111100
	381, // 38: 101111101
	382, // 39: 101111110
}

// prefixLookup is a 512-entry table for O(1) Huffman prefix lookup.
// Indexed by the first 9 bits of the stream (MSB-first, left-justified).
var prefixLookup [512]zgfxToken

func init() {
	for i := range tokenTable {
		tk := tokenTable[i]
		prefix := tokenPrefixCodes[i]
		padBits := 9 - int(tk.prefixBits)
		base := int(prefix) << padBits
		count := 1 << padBits
		for j := 0; j < count; j++ {
			prefixLookup[base+j] = tk
		}
	}
}

// Error sentinels
var (
	errShortInput    = zgfxError("zgfx: input too short")
	errTruncated     = zgfxError("zgfx: truncated data")
	errBadDescriptor = zgfxError("zgfx: invalid descriptor byte")
	errBitsExhausted = zgfxError("zgfx: bits exhausted")
	errBadToken      = zgfxError("zgfx: invalid Huffman token")
)

type zgfxError string

func (e zgfxError) Error() string { return string(e) }
