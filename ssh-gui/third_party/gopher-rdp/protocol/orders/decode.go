package orders

import (
	"encoding/binary"
	"fmt"
)

// DecodeOrders decodes an RDP order stream, calling fn for each order.
// state is maintained across calls (primary orders are stateful).
// data is the raw order bytes; numOrders is the count from the update header.
func DecodeOrders(state *DecoderState, data []byte, numOrders int, fn OrderCallback) {
	off := 0
	var ord Order
	decoded := 0

	for i := 0; i < numOrders && off < len(data); i++ {
		controlFlags := data[off]
		off++

		if controlFlags&TSStandard == 0 {
			// Alternate secondary order (MS-RDPEGDI 2.2.2.2.1.3.1.1).
			// orderLength = total size - 2. Skip the order.
			if off >= len(data) {
				state.DebugBailReason = "altsec: off >= len(data)"
				return
			}
			altLen := int(data[off])
			off++
			off += altLen
			decoded++
			continue
		}

		if controlFlags&TSSecondary != 0 {
			// Secondary order
			off = decodeSecondary(data, off, &ord, fn, state)
			if off < 0 {
				state.DebugBailReason = "secondary returned -1"
				return
			}
			decoded++
			continue
		}

		// Primary order
		if controlFlags&TSTypeChange != 0 {
			if off >= len(data) {
				state.DebugBailReason = "typechange: off >= len(data)"
				return
			}
			state.LastOrderType = data[off]
			off++
		}

		orderType := state.LastOrderType
		if int(orderType) >= len(orderFieldCount) {
			state.DebugBailReason = fmt.Sprintf("unknown orderType=%d at i=%d decoded=%d off=%d", orderType, i, decoded, off)
			return // unknown order type
		}
		nFields := int(orderFieldCount[orderType])
		if nFields == 0 {
			state.DebugBailReason = fmt.Sprintf("unsupported orderType=%d (nFields=0) at i=%d decoded=%d off=%d", orderType, i, decoded, off)
			return // unsupported order type
		}

		// Read field flags (variable length: 1-3 bytes based on nFields).
		// TS_ZERO_FIELD_BYTE_BIT0 (0x40): MSB is zero → reduce count by 1.
		// TS_ZERO_FIELD_BYTE_BIT1 (0x80): second MSB also zero → reduce by 2.
		// MS-RDPEGDI 2.2.2.2.1.1.2 field encoding flags.
		nFlagBytes := (nFields + 7) / 8
		if controlFlags&TSZeroFieldByte0 != 0 {
			nFlagBytes--
		}
		if controlFlags&TSZeroFieldByte1 != 0 {
			nFlagBytes -= 2
		}
		if nFlagBytes < 0 {
			nFlagBytes = 0
		}

		var fieldFlags uint32
		for fb := 0; fb < nFlagBytes; fb++ {
			if off >= len(data) {
				return
			}
			fieldFlags |= uint32(data[off]) << (fb * 8)
			off++
		}

		// Read bounds if TSBounds (0x04) is set.
		// If TSZeroBoundsDelta (0x20) is also set, reuse last bounds without parsing.
		if controlFlags&TSBounds != 0 {
			if controlFlags&TSZeroBoundsDelta == 0 {
				off = decodeBounds(data, off, &state.Bounds)
				if off < 0 {
					return
				}
			}
		}

		delta := controlFlags&TSDeltaCoordinates != 0

		// Decode order-type-specific fields
		switch orderType {
		case OrderDstBlt:
			off = decodeDstBlt(data, off, fieldFlags, delta, &state.DstBlt)
		case OrderPatBlt:
			off = decodePatBlt(data, off, fieldFlags, delta, &state.PatBlt)
		case OrderScrBlt:
			off = decodeScrBlt(data, off, fieldFlags, delta, &state.ScrBlt)
		case OrderOpaqueRect:
			off = decodeOpaqueRect(data, off, fieldFlags, delta, &state.OpaqueRect)
		case OrderPolygonSC:
			off = decodePolygonSC(data, off, fieldFlags, delta, &state.PolygonSC)
		case OrderPolygonCB:
			off = decodePolygonCB(data, off, fieldFlags, delta, &state.PolygonCB)
		case OrderEllipseCB:
			off = decodeEllipseCB(data, off, fieldFlags, delta, &state.EllipseCB)
		case OrderMemBlt:
			off = decodeMemBlt(data, off, fieldFlags, delta, &state.MemBlt)
		case OrderGlyphIndex:
			off = decodeGlyphIndex(data, off, fieldFlags, delta, &state.GlyphIndex)
		case OrderFastIndex:
			off = decodeFastIndex(data, off, fieldFlags, delta, &state.FastIndex)
		case OrderLineTo:
			off = decodeLineTo(data, off, fieldFlags, delta, &state.LineTo)
		case OrderMem3Blt:
			off = decodeMem3Blt(data, off, fieldFlags, delta, &state.Mem3Blt)
		case OrderFastGlyph:
			off = decodeFastIndex(data, off, fieldFlags, delta, &state.FastGlyph)
		case OrderPolyline:
			off = decodePolyline(data, off, fieldFlags, delta, &state.Polyline)
		case OrderEllipseSC:
			off = decodeEllipseSC(data, off, fieldFlags, delta, &state.EllipseSC)
		case OrderSaveBitmap:
			off = decodeSaveBitmap(data, off, fieldFlags, delta, &state.SaveBitmap)
		default:
			// Unsupported primary order — skip field data.
			// We can't know the exact byte length, so we stop processing.
			return
		}
		if off < 0 {
			return
		}

		ord.Type = orderType
		ord.FieldFlags = fieldFlags
		ord.IsSecondary = false
		ord.HasBounds = controlFlags&TSBounds != 0
		fn(state, &ord)
	}
}

// decodeSecondary parses a secondary order header and calls the callback.
// Returns new offset, or -1 on error.
func decodeSecondary(data []byte, off int, ord *Order, fn OrderCallback, state *DecoderState) int {
	if off+5 > len(data) {
		return -1
	}
	orderLength := int(int16(binary.LittleEndian.Uint16(data[off:])))
	extraFlags := binary.LittleEndian.Uint16(data[off+2:])
	orderType := data[off+4]
	off += 5

	// MS-RDPEGDI 2.2.2.2.1.2.1.1: total = orderLength + 13 (including controlFlags).
	// After controlFlags(1) + header(5), data payload = orderLength + 13 - 6 = orderLength + 7.
	dataLen := orderLength + 7
	if dataLen < 0 || off+dataLen > len(data) {
		return -1
	}

	ord.IsSecondary = true
	ord.SecondaryType = orderType
	ord.ExtraFlags = extraFlags
	ord.SecData = data[off : off+dataLen]
	fn(state, ord)

	return off + dataLen
}

// decodeBounds reads a bounds rectangle. Returns new offset or -1 on error.
func decodeBounds(data []byte, off int, b *Bounds) int {
	if off >= len(data) {
		return -1
	}
	flags := data[off]
	off++

	if flags&0x01 != 0 {
		if off+2 > len(data) {
			return -1
		}
		b.Left = int16(binary.LittleEndian.Uint16(data[off:]))
		off += 2
	} else if flags&0x10 != 0 {
		if off >= len(data) {
			return -1
		}
		b.Left += int16(int8(data[off]))
		off++
	}

	if flags&0x02 != 0 {
		if off+2 > len(data) {
			return -1
		}
		b.Top = int16(binary.LittleEndian.Uint16(data[off:]))
		off += 2
	} else if flags&0x20 != 0 {
		if off >= len(data) {
			return -1
		}
		b.Top += int16(int8(data[off]))
		off++
	}

	if flags&0x04 != 0 {
		if off+2 > len(data) {
			return -1
		}
		b.Right = int16(binary.LittleEndian.Uint16(data[off:]))
		off += 2
	} else if flags&0x40 != 0 {
		if off >= len(data) {
			return -1
		}
		b.Right += int16(int8(data[off]))
		off++
	}

	if flags&0x08 != 0 {
		if off+2 > len(data) {
			return -1
		}
		b.Bottom = int16(binary.LittleEndian.Uint16(data[off:]))
		off += 2
	} else if flags&0x80 != 0 {
		if off >= len(data) {
			return -1
		}
		b.Bottom += int16(int8(data[off]))
		off++
	}

	return off
}

// --- Field reader helpers (inline, no alloc) ---

func readU8(data []byte, off int, flags uint32, bit int, dst *uint8) int {
	if flags&(1<<bit) == 0 {
		return off
	}
	if off >= len(data) {
		return -1
	}
	*dst = data[off]
	return off + 1
}

func readU16(data []byte, off int, flags uint32, bit int, dst *uint16) int {
	if flags&(1<<bit) == 0 {
		return off
	}
	if off+2 > len(data) {
		return -1
	}
	*dst = binary.LittleEndian.Uint16(data[off:])
	return off + 2
}

func readI16(data []byte, off int, flags uint32, bit int, delta bool, dst *int16) int {
	if flags&(1<<bit) == 0 {
		return off
	}
	if delta {
		if off >= len(data) {
			return -1
		}
		*dst += int16(int8(data[off]))
		return off + 1
	}
	if off+2 > len(data) {
		return -1
	}
	*dst = int16(binary.LittleEndian.Uint16(data[off:]))
	return off + 2
}

func readColor(data []byte, off int, flags uint32, bit int, dst *uint32) int {
	if flags&(1<<bit) == 0 {
		return off
	}
	if off+3 > len(data) {
		return -1
	}
	*dst = uint32(data[off]) | uint32(data[off+1])<<8 | uint32(data[off+2])<<16
	return off + 3
}

// ColourToRGBA converts a wire-format order color to R, G, B bytes based on
// the server's color depth (MS-RDPEGDI 2.2.2.2.1.1.2 color encoding).
//
// Wire format storage: readColor stores byte0 | byte1<<8 | byte2<<16.
//   - 24bpp: low byte = R, mid byte = G, high byte = B
//   - 16bpp (RGB565): 2 bytes in low 16 bits, byte2 ignored
//   - 15bpp (RGB555): 2 bytes in low 16 bits, byte2 ignored
func ColourToRGBA(c uint32, serverBpp int, pal *[256][3]byte) (r, g, b uint8) {
	switch serverBpp {
	case 8:
		// 8bpp: color value is a palette index
		if pal != nil {
			p := pal[c&0xFF]
			return p[0], p[1], p[2]
		}
		v := uint8(c & 0xFF)
		return v, v, v
	case 15:
		// RGB555: XRRRRRGGGGGBBBBB
		r = uint8(((c >> 7) & 0xF8) | ((c >> 12) & 0x7))
		g = uint8(((c >> 2) & 0xF8) | ((c >> 7) & 0x7))
		b = uint8(((c << 3) & 0xF8) | ((c >> 2) & 0x7))
	case 16:
		// RGB565: RRRRRGGGGGGBBBBB
		r = uint8(((c >> 8) & 0xF8) | ((c >> 13) & 0x7))
		g = uint8(((c >> 3) & 0xFC) | ((c >> 9) & 0x3))
		b = uint8(((c << 3) & 0xF8) | ((c >> 2) & 0x7))
	default:
		// 24/32bpp: low byte = R, high byte = B
		r = uint8(c & 0xFF)
		g = uint8((c >> 8) & 0xFF)
		b = uint8((c >> 16) & 0xFF)
	}
	return
}

func readVarBytes(data []byte, off int, flags uint32, bit int, dst []byte, dstLen *uint8) int {
	if flags&(1<<bit) == 0 {
		return off
	}
	if off >= len(data) {
		return -1
	}
	n := int(data[off])
	off++
	if n > len(dst) {
		n = len(dst)
	}
	if off+n > len(data) {
		return -1
	}
	copy(dst[:n], data[off:off+n])
	*dstLen = uint8(n)
	return off + n
}

func readVarBytes16(data []byte, off int, flags uint32, bit int, dst []byte, dstLen *uint16) int {
	if flags&(1<<bit) == 0 {
		return off
	}
	if off+2 > len(data) {
		return -1
	}
	n := int(binary.LittleEndian.Uint16(data[off:]))
	off += 2
	if n > len(dst) {
		n = len(dst)
	}
	if off+n > len(data) {
		return -1
	}
	copy(dst[:n], data[off:off+n])
	*dstLen = uint16(n)
	return off + n
}

func readBrush(data []byte, off int, flags uint32, startBit int,
	orgX, orgY, style, hatch *uint8, extra []byte) int {
	off = readU8(data, off, flags, startBit, orgX)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, startBit+1, orgY)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, startBit+2, style)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, startBit+3, hatch)
	if off < 0 {
		return -1
	}
	// Brush extra (7 bytes as a single field, bit startBit+4)
	if flags&(1<<(startBit+4)) != 0 {
		if off+7 > len(data) {
			return -1
		}
		copy(extra[:7], data[off:off+7])
		off += 7
	}
	return off
}

// --- Primary order decoders ---

// decodeDstBlt reads DstBlt fields (5 fields, 1 flag byte).
// Fields: 0:left 1:top 2:width 3:height 4:rop
func decodeDstBlt(data []byte, off int, flags uint32, delta bool, s *DstBltState) int {
	off = readI16(data, off, flags, 0, delta, &s.Left)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.Top)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 2, delta, &s.Width)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 3, delta, &s.Height)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 4, &s.Rop)
	return off
}

// decodePatBlt reads PatBlt fields (12 fields, 2 flag bytes).
// Fields: 0:left 1:top 2:width 3:height 4:rop 5:backColor 6:foreColor
//
//	7:brushOrgX 8:brushOrgY 9:brushStyle 10:brushHatch 11:brushExtra
func decodePatBlt(data []byte, off int, flags uint32, delta bool, s *PatBltState) int {
	off = readI16(data, off, flags, 0, delta, &s.Left)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.Top)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 2, delta, &s.Width)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 3, delta, &s.Height)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 4, &s.Rop)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 5, &s.BackColor)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 6, &s.ForeColor)
	if off < 0 {
		return -1
	}
	off = readBrush(data, off, flags, 7,
		&s.BrushOrgX, &s.BrushOrgY, &s.BrushStyle, &s.BrushHatch, s.BrushExtra[:])
	return off
}

// decodeScrBlt reads ScrBlt fields (7 fields, 1 flag byte).
// Fields: 0:left 1:top 2:width 3:height 4:rop 5:srcLeft 6:srcTop
func decodeScrBlt(data []byte, off int, flags uint32, delta bool, s *ScrBltState) int {
	off = readI16(data, off, flags, 0, delta, &s.Left)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.Top)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 2, delta, &s.Width)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 3, delta, &s.Height)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 4, &s.Rop)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 5, delta, &s.SrcLeft)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 6, delta, &s.SrcTop)
	return off
}

// decodeOpaqueRect reads OpaqueRect fields (7 fields, 1 flag byte).
// Fields: 0:left 1:top 2:width 3:height 4:colorR 5:colorG 6:colorB
func decodeOpaqueRect(data []byte, off int, flags uint32, delta bool, s *OpaqueRectState) int {
	off = readI16(data, off, flags, 0, delta, &s.Left)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.Top)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 2, delta, &s.Width)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 3, delta, &s.Height)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 4, &s.ColorR)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 5, &s.ColorG)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 6, &s.ColorB)
	return off
}

// decodeMemBlt reads MemBlt fields (9 fields, 2 flag bytes).
// Fields: 0:cacheId 1:left 2:top 3:width 4:height 5:rop 6:srcLeft 7:srcTop 8:cacheIndex
func decodeMemBlt(data []byte, off int, flags uint32, delta bool, s *MemBltState) int {
	off = readU16(data, off, flags, 0, &s.CacheID)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.Left)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 2, delta, &s.Top)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 3, delta, &s.Width)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 4, delta, &s.Height)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 5, &s.Rop)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 6, delta, &s.SrcLeft)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 7, delta, &s.SrcTop)
	if off < 0 {
		return -1
	}
	off = readU16(data, off, flags, 8, &s.CacheIndex)
	return off
}

// decodeGlyphIndex reads GlyphIndex fields per MS-RDPEGDI 2.2.2.2.1.2.5.
// Field order (22 fields, 3 flag bytes):
//
//	 0: cacheId       1: flAccel       2: ulCharInc     3: fOpRedundant
//	 4: backColor     5: foreColor
//	 6: bkLeft        7: bkTop         8: bkRight       9: bkBottom
//	10: opLeft       11: opTop        12: opRight      13: opBottom
//	14: brushOrgX    15: brushOrgY    16: brushStyle   17: brushHatch   18: brushExtra
//	19: x            20: y
//	21: varBytes
func decodeGlyphIndex(data []byte, off int, flags uint32, delta bool, s *GlyphIndexState) int {
	off = readU8(data, off, flags, 0, &s.CacheID)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 1, &s.FlAccel)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 2, &s.UlCharInc)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 3, &s.FOpRedundant)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 4, &s.BackColor)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 5, &s.ForeColor)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 6, delta, &s.BkLeft)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 7, delta, &s.BkTop)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 8, delta, &s.BkRight)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 9, delta, &s.BkBottom)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 10, delta, &s.OpLeft)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 11, delta, &s.OpTop)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 12, delta, &s.OpRight)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 13, delta, &s.OpBottom)
	if off < 0 {
		return -1
	}
	off = readBrush(data, off, flags, 14,
		&s.BrushOrgX, &s.BrushOrgY, &s.BrushStyle, &s.BrushHatch, s.BrushExtra[:])
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 19, delta, &s.X)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 20, delta, &s.Y)
	if off < 0 {
		return -1
	}
	off = readVarBytes(data, off, flags, 21, s.VarBytes[:], &s.VarLen)
	return off
}

// decodeFastIndex reads FastIndex fields per MS-RDPEGDI 2.2.2.2.1.2.3.
// Field order (15 fields, 2 flag bytes):
//
//	 0: cacheId       1: fDrawing(u16)
//	 2: backColor     3: foreColor
//	 4: bkLeft        5: bkTop         6: bkRight       7: bkBottom
//	 8: opLeft        9: opTop        10: opRight      11: opBottom
//	12: x            13: y
//	14: varBytes
func decodeFastIndex(data []byte, off int, flags uint32, delta bool, s *FastIndexState) int {
	off = readU8(data, off, flags, 0, &s.CacheID)
	if off < 0 {
		return -1
	}
	off = readU16(data, off, flags, 1, &s.FDrawing)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 2, &s.BackColor)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 3, &s.ForeColor)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 4, delta, &s.BkLeft)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 5, delta, &s.BkTop)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 6, delta, &s.BkRight)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 7, delta, &s.BkBottom)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 8, delta, &s.OpLeft)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 9, delta, &s.OpTop)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 10, delta, &s.OpRight)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 11, delta, &s.OpBottom)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 12, delta, &s.X)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 13, delta, &s.Y)
	if off < 0 {
		return -1
	}
	off = readVarBytes(data, off, flags, 14, s.VarBytes[:], &s.VarLen)
	return off
}

// decodeLineTo reads LineTo fields per MS-RDPEGDI 2.2.2.2.1.1.2.1.
// Field order (10 fields, 2 flag bytes):
//
//	0: backMode   1: startX   2: startY   3: endX   4: endY
//	5: backColor  6: rop2     7: penStyle 8: penWidth 9: penColor
func decodeLineTo(data []byte, off int, flags uint32, delta bool, s *LineToState) int {
	off = readU16(data, off, flags, 0, &s.BackMode)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.StartX)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 2, delta, &s.StartY)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 3, delta, &s.EndX)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 4, delta, &s.EndY)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 5, &s.BackColor)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 6, &s.Rop2)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 7, &s.PenStyle)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 8, &s.PenWidth)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 9, &s.PenColor)
	return off
}

// decodeMem3Blt reads Mem3Blt fields per MS-RDPEGDI 2.2.2.2.1.1.2.10.
// Field order (17 fields, 3 flag bytes):
//
//	 0: cacheId    1: left       2: top        3: width      4: height
//	 5: rop        6: srcLeft    7: srcTop     8: backColor  9: foreColor
//	10: brushOrgX 11: brushOrgY 12: brushStyle 13: brushHatch 14: brushExtra
//	15: cacheIndex 16: unknown
func decodeMem3Blt(data []byte, off int, flags uint32, delta bool, s *Mem3BltState) int {
	off = readU16(data, off, flags, 0, &s.CacheID)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.Left)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 2, delta, &s.Top)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 3, delta, &s.Width)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 4, delta, &s.Height)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 5, &s.Rop)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 6, delta, &s.SrcLeft)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 7, delta, &s.SrcTop)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 8, &s.BackColor)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 9, &s.ForeColor)
	if off < 0 {
		return -1
	}
	off = readBrush(data, off, flags, 10,
		&s.BrushOrgX, &s.BrushOrgY, &s.BrushStyle, &s.BrushHatch, s.BrushExtra[:])
	if off < 0 {
		return -1
	}
	off = readU16(data, off, flags, 15, &s.CacheIndex)
	if off < 0 {
		return -1
	}
	off = readU16(data, off, flags, 16, &s.Unknown)
	return off
}

// decodePolyline reads Polyline fields per MS-RDPEGDI 2.2.2.2.1.1.2.2.
// Field order (7 fields, 1 flag byte):
//
//	0: xStart  1: yStart  2: rop2  3: brushCacheEntry(u16)
//	4: penColor  5: numDeltaEntries  6: codedDeltaList(var)
func decodePolyline(data []byte, off int, flags uint32, delta bool, s *PolylineState) int {
	off = readI16(data, off, flags, 0, delta, &s.StartX)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.StartY)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 2, &s.Rop2)
	if off < 0 {
		return -1
	}
	off = readU16(data, off, flags, 3, &s.BrushCacheEntry)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 4, &s.PenColor)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 5, &s.NumDeltaEntries)
	if off < 0 {
		return -1
	}
	off = readVarBytes(data, off, flags, 6, s.VarBytes[:], &s.VarLen)
	return off
}

// decodePolygonSC reads PolygonSC fields per MS-RDPEGDI 2.2.2.2.1.1.2.16.
// Field order (7 fields, 1 flag byte):
//
//	0: x  1: y  2: rop2  3: fillMode  4: color  5: numDeltaEntries  6: codedDeltaList(var)
func decodePolygonSC(data []byte, off int, flags uint32, delta bool, s *PolygonSCState) int {
	off = readI16(data, off, flags, 0, delta, &s.X)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.Y)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 2, &s.Rop2)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 3, &s.FillMode)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 4, &s.Color)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 5, &s.NumDeltaEntries)
	if off < 0 {
		return -1
	}
	off = readVarBytes(data, off, flags, 6, s.VarBytes[:], &s.VarLen)
	return off
}

// decodePolygonCB reads PolygonCB fields per MS-RDPEGDI 2.2.2.2.1.1.2.17.
// Field order (13 fields, 2 flag bytes):
//
//	0: x  1: y  2: rop2  3: fillMode  4: backColor  5: foreColor
//	6-10: brush (orgX, orgY, style, hatch, extra)  11: numDeltaEntries  12: codedDeltaList(var)
func decodePolygonCB(data []byte, off int, flags uint32, delta bool, s *PolygonCBState) int {
	off = readI16(data, off, flags, 0, delta, &s.X)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.Y)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 2, &s.Rop2)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 3, &s.FillMode)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 4, &s.BackColor)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 5, &s.ForeColor)
	if off < 0 {
		return -1
	}
	off = readBrush(data, off, flags, 6,
		&s.BrushOrgX, &s.BrushOrgY, &s.BrushStyle, &s.BrushHatch, s.BrushExtra[:])
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 11, &s.NumDeltaEntries)
	if off < 0 {
		return -1
	}
	off = readVarBytes(data, off, flags, 12, s.VarBytes[:], &s.VarLen)
	return off
}

// decodeEllipseCB reads EllipseCB fields per MS-RDPEGDI 2.2.2.2.1.1.2.19.
// Field order (13 fields, 2 flag bytes):
//
//	0: left  1: top  2: right  3: bottom  4: rop2  5: fillMode
//	6: backColor  7: foreColor  8-12: brush (orgX, orgY, style, hatch, extra)
func decodeEllipseCB(data []byte, off int, flags uint32, delta bool, s *EllipseCBState) int {
	off = readI16(data, off, flags, 0, delta, &s.Left)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.Top)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 2, delta, &s.Right)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 3, delta, &s.Bottom)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 4, &s.Rop2)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 5, &s.FillMode)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 6, &s.BackColor)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 7, &s.ForeColor)
	if off < 0 {
		return -1
	}
	off = readBrush(data, off, flags, 8,
		&s.BrushOrgX, &s.BrushOrgY, &s.BrushStyle, &s.BrushHatch, s.BrushExtra[:])
	return off
}

// decodeEllipseSC reads EllipseSC fields per MS-RDPEGDI 2.2.2.2.1.1.2.8.
// Field order (7 fields, 1 flag byte):
//
//	0: left  1: top  2: right  3: bottom  4: rop2  5: fillMode  6: color
func decodeEllipseSC(data []byte, off int, flags uint32, delta bool, s *EllipseSCState) int {
	off = readI16(data, off, flags, 0, delta, &s.Left)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 1, delta, &s.Top)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 2, delta, &s.Right)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 3, delta, &s.Bottom)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 4, &s.Rop2)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 5, &s.FillMode)
	if off < 0 {
		return -1
	}
	off = readColor(data, off, flags, 6, &s.Color)
	return off
}

// decodeSaveBitmap reads SaveBitmap/DesktopSave fields per MS-RDPEGDI 2.2.2.2.1.1.2.11.
// Field order (6 fields, 1 flag byte):
//
//	0: offset(u32) 1: left 2: top 3: right 4: bottom 5: action(u8)
func decodeSaveBitmap(data []byte, off int, flags uint32, delta bool, s *SaveBitmapState) int {
	if flags&(1<<0) != 0 {
		if off+4 > len(data) {
			return -1
		}
		s.Offset = binary.LittleEndian.Uint32(data[off:])
		off += 4
	}
	off = readI16(data, off, flags, 1, delta, &s.Left)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 2, delta, &s.Top)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 3, delta, &s.Right)
	if off < 0 {
		return -1
	}
	off = readI16(data, off, flags, 4, delta, &s.Bottom)
	if off < 0 {
		return -1
	}
	off = readU8(data, off, flags, 5, &s.Action)
	return off
}
