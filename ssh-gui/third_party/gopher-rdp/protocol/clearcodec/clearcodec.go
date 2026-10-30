// Package clearcodec implements the ClearCodec bitmap decoder (MS-RDPEGFX 2.2.4).
//
// ClearCodec uses three composited layers (residual, bands, subcodec) with
// vbar caching for efficient encoding of repeated vertical column patterns.
// Output is top-down RGBA 32bpp.
//
// Implements MS-RDPEGFX 2.2.4.
package clearcodec

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"

	"github.com/bouncyball-git/gopher-rdp/protocol/nscodec"
)

const levelTrace = slog.LevelDebug - 4

// Glyph flags.
const (
	flagGlyphIndex = 0x01
	flagGlyphHit   = 0x02
	flagCacheReset = 0x04
)

// Cache sizes per MS-RDPEGFX 2.2.4.1.
const (
	vbarCacheSize      = 32768
	shortVBarCacheSize = 16384
	glyphCacheSize     = 4000
	maxVBarHeight      = 52
)

type vbarEntry struct {
	pixels []byte // BGRX column data (4 bytes per pixel)
	count  int    // pixel count (height)
}

type glyphEntry struct {
	data   []byte
	width  int
	height int
}

// Decoder maintains ClearCodec decompression state including the vbar caches.
type Decoder struct {
	log              *slog.Logger
	seqNumber        byte
	vbarStorage      [vbarCacheSize]vbarEntry
	vbarCursor       uint32
	shortVBarStorage [shortVBarCacheSize]vbarEntry
	shortVBarCursor  uint32
	glyphCache       [glyphCacheSize]glyphEntry
	nscBuf           []byte // reusable NSCodec pixel output buffer
	nscPlanesBuf     []byte // reusable NSCodec planes buffer
	vbarTmpPixels    []byte // reusable buffer for constructFullVBar temporary pixels
}

// New creates a ClearCodec decoder with the given logger.
func New(log *slog.Logger) *Decoder {
	return &Decoder{log: log}
}

// ResetState resets the ClearCodec sequence number.
// Called on ResetGraphics per MS-RDPEGFX 3.1.8.1.1.
func (d *Decoder) ResetState() {
	d.seqNumber = 0
}

// Decompress decodes ClearCodec-compressed data into top-down BGRX pixels.
// dst is a reusable buffer that is grown as needed.
func (d *Decoder) Decompress(dst []byte, width, height int, src []byte) ([]byte, error) {
	if len(src) < 2 {
		return dst, errShortHeader
	}

	glyphFlags := src[0]
	seqNumber := src[1]
	off := 2

	// Parse optional glyph index
	var glyphIndex uint16
	if glyphFlags&flagGlyphIndex != 0 {
		if off+2 > len(src) {
			return dst, errShortHeader
		}
		glyphIndex = binary.LittleEndian.Uint16(src[off : off+2])
		off += 2
	}

	// Validate sequence number. On mismatch, abort the tile (matching
	// FreeRDP clear.c). We still advance seqNumber so subsequent tiles
	// can re-sync.
	if d.seqNumber != 0 && seqNumber != d.seqNumber {
		d.log.LogAttrs(context.Background(), slog.LevelWarn, "sequence number mismatch",
			slog.Int("expected", int(d.seqNumber)), slog.Int("got", int(seqNumber)))
		d.seqNumber = (seqNumber + 1) & 0xFF
		return dst, fmt.Errorf("clearcodec: sequence number mismatch (expected %d, got %d)", d.seqNumber-1, seqNumber)
	}
	d.seqNumber = (seqNumber + 1) & 0xFF

	// Handle cache reset
	if glyphFlags&flagCacheReset != 0 {
		d.vbarCursor = 0
		d.shortVBarCursor = 0
	}

	// Prepare output buffer
	dstSize := width * height * 4
	if cap(dst) >= dstSize {
		dst = dst[:dstSize]
		clear(dst)
	} else {
		dst = make([]byte, dstSize)
	}

	// Glyph hit — copy from cache
	if glyphFlags&flagGlyphHit != 0 {
		if glyphFlags&flagGlyphIndex == 0 {
			return dst, errBadGlyphFlags
		}
		if int(glyphIndex) >= glyphCacheSize {
			return dst, fmt.Errorf("clearcodec: glyph index %d out of range", glyphIndex)
		}
		g := &d.glyphCache[glyphIndex]
		if g.data == nil {
			return dst, fmt.Errorf("clearcodec: glyph cache miss at index %d", glyphIndex)
		}
		copy(dst, g.data)
		d.log.LogAttrs(context.Background(), slog.LevelDebug, "glyph hit", slog.Int("idx", int(glyphIndex)), slog.Int("width", int(g.width)), slog.Int("height", int(g.height)))
		return dst, nil
	}

	if off+12 > len(src) {
		return dst, errShortHeader
	}

	residualLen := int(binary.LittleEndian.Uint32(src[off : off+4]))
	bandsLen := int(binary.LittleEndian.Uint32(src[off+4 : off+8]))
	subcodecLen := int(binary.LittleEndian.Uint32(src[off+8 : off+12]))
	off += 12

	totalPayload := residualLen + bandsLen + subcodecLen
	if off+totalPayload > len(src) {
		return dst, errTruncated
	}

	d.log.Log(context.Background(), levelTrace, "decompress",
		"width", width, "height", height,
		"glyph", fmt.Sprintf("0x%02X", glyphFlags), "seq", seqNumber,
		"residual", residualLen, "bands", bandsLen, "subcodec", subcodecLen)

	// Decode residual layer
	if residualLen > 0 {
		if err := d.decodeResidual(dst, width, height, src[off:off+residualLen]); err != nil {
			return dst, err
		}
	}
	off += residualLen

	// Decode band layer
	if bandsLen > 0 {
		if err := d.decodeBands(dst, width, height, src[off:off+bandsLen]); err != nil {
			return dst, err
		}
	}
	off += bandsLen

	// Decode subcodec layer
	if subcodecLen > 0 {
		if err := d.decodeSubcodec(dst, width, src[off:off+subcodecLen]); err != nil {
			return dst, err
		}
	}

	// Cache glyph if glyph index was provided
	if glyphFlags&flagGlyphIndex != 0 && int(glyphIndex) < glyphCacheSize {
		g := &d.glyphCache[glyphIndex]
		if cap(g.data) >= len(dst) {
			g.data = g.data[:len(dst)]
		} else {
			g.data = make([]byte, len(dst))
		}
		copy(g.data, dst)
		g.width = width
		g.height = height
	}

	return dst, nil
}

// decodeResidual decodes the residual layer.
// Residual layer: B, G, R bytes + cascading runLengthFactor (u8 → u16 → u32 replacement).
func (d *Decoder) decodeResidual(dst []byte, width, height int, src []byte) error {
	totalPixels := width * height
	di := 0 // pixel index
	si := 0

	for si < len(src) && di < totalPixels {
		if si+4 > len(src) {
			return errTruncated
		}
		b := src[si]
		g := src[si+1]
		r := src[si+2]

		// Cascading run-length factor (replace semantics)
		runLen := uint32(src[si+3])
		si += 4

		if runLen >= 0xFF {
			if si+2 > len(src) {
				return errTruncated
			}
			runLen = uint32(binary.LittleEndian.Uint16(src[si : si+2]))
			si += 2
			if runLen >= 0xFFFF {
				if si+4 > len(src) {
					return errTruncated
				}
				runLen = binary.LittleEndian.Uint32(src[si : si+4])
				si += 4
			}
		}

		if di+int(runLen) > totalPixels {
			return errOverflow
		}

		for j := uint32(0); j < runLen; j++ {
			off := di * 4
			dst[off] = r
			dst[off+1] = g
			dst[off+2] = b
			dst[off+3] = 0xFF
			di++
		}
	}
	return nil
}

// decodeBands decodes the band layer with vbar caching.
// Decodes the band layer with vbar caching (MS-RDPEGFX 2.2.4.1.1).
func (d *Decoder) decodeBands(dst []byte, dstWidth, dstHeight int, src []byte) error {
	si := 0
	dstStride := dstWidth * 4

	for si < len(src) {
		// Band header: xStart(2) + xEnd(2) + yStart(2) + yEnd(2) + bgB(1) + bgG(1) + bgR(1) = 11
		if si+11 > len(src) {
			return errTruncated
		}
		xStart := int(binary.LittleEndian.Uint16(src[si : si+2]))
		xEnd := int(binary.LittleEndian.Uint16(src[si+2 : si+4]))
		yStart := int(binary.LittleEndian.Uint16(src[si+4 : si+6]))
		yEnd := int(binary.LittleEndian.Uint16(src[si+6 : si+8]))
		bgB := src[si+8]
		bgG := src[si+9]
		bgR := src[si+10]
		si += 11

		if xEnd < xStart || yEnd < yStart {
			return errBadBandRect
		}

		vBarCount := (xEnd - xStart) + 1
		vBarHeight := (yEnd - yStart) + 1

		if vBarHeight > maxVBarHeight {
			return errBadVBarHeight
		}

		for col := 0; col < vBarCount; col++ {
			if si+2 > len(src) {
				return errTruncated
			}
			vBarHeader := binary.LittleEndian.Uint16(src[si : si+2])
			si += 2

			var vbar *vbarEntry
			vBarUpdate := false
			var vBarYOn int
			var vBarShortPixelCount int

			if vBarHeader&0xC000 == 0x4000 {
				// SHORT_VBAR_CACHE_HIT
				idx := vBarHeader & 0x3FFF
				if int(idx) >= shortVBarCacheSize {
					return errBadVBarIndex
				}
				shortEntry := &d.shortVBarStorage[idx]

				if si >= len(src) {
					return errTruncated
				}
				vBarYOn = int(src[si])
				si++
				vBarShortPixelCount = shortEntry.count
				vBarUpdate = true
				vbar = d.constructFullVBar(vBarHeight, vBarYOn, vBarShortPixelCount,
					shortEntry, bgB, bgG, bgR)

			} else if vBarHeader&0xC000 == 0x0000 {
				// SHORT_VBAR_CACHE_MISS
				// vBarYOn = low byte, vBarYOff = bits 8-13
				vBarYOn = int(vBarHeader & 0xFF)
				vBarYOff := int((vBarHeader >> 8) & 0x3F)

				if vBarYOff < vBarYOn {
					return errBadVBarYOff
				}

				vBarShortPixelCount = vBarYOff - vBarYOn
				if vBarShortPixelCount > maxVBarHeight {
					return errBadVBarHeight
				}

				// Read BGR pixels for the short vbar
				pixBytes := vBarShortPixelCount * 3
				if si+pixBytes > len(src) {
					return errTruncated
				}

				if d.shortVBarCursor >= shortVBarCacheSize {
					return errCacheFull
				}

				shortEntry := &d.shortVBarStorage[d.shortVBarCursor]
				shortEntry.count = vBarShortPixelCount
				need := vBarShortPixelCount * 4
				if cap(shortEntry.pixels) >= need {
					shortEntry.pixels = shortEntry.pixels[:need]
				} else {
					shortEntry.pixels = make([]byte, need)
				}

				for i := 0; i < vBarShortPixelCount; i++ {
					off := i * 4
					shortEntry.pixels[off] = src[si+2]   // R
					shortEntry.pixels[off+1] = src[si+1] // G
					shortEntry.pixels[off+2] = src[si]   // B
					shortEntry.pixels[off+3] = 0xFF
					si += 3
				}

				d.shortVBarCursor = (d.shortVBarCursor + 1) % shortVBarCacheSize
				vBarUpdate = true
				vbar = d.constructFullVBar(vBarHeight, vBarYOn, vBarShortPixelCount,
					shortEntry, bgB, bgG, bgR)

			} else if vBarHeader&0x8000 == 0x8000 {
				// VBAR_CACHE_HIT
				idx := vBarHeader & 0x7FFF
				if int(idx) >= vbarCacheSize {
					return errBadVBarIndex
				}
				vbar = &d.vbarStorage[idx]

				// Fill dummy data if entry is empty (after cache reset)
				if vbar.count == 0 {
					vbar.count = vBarHeight
					need := vBarHeight * 4
					if cap(vbar.pixels) >= need {
						vbar.pixels = vbar.pixels[:need]
					} else {
						vbar.pixels = make([]byte, need)
					}
				}
			} else {
				return errBadVBarHeader
			}

			// Store newly constructed full vbar in the VBar cache
			if vBarUpdate {
				if d.vbarCursor >= vbarCacheSize {
					return errCacheFull
				}
				entry := &d.vbarStorage[d.vbarCursor]
				entry.count = vbar.count
				if cap(entry.pixels) >= len(vbar.pixels) {
					entry.pixels = entry.pixels[:len(vbar.pixels)]
				} else {
					entry.pixels = make([]byte, len(vbar.pixels))
				}
				copy(entry.pixels, vbar.pixels)
				vbar = entry // point to the stored entry for painting
				d.vbarCursor = (d.vbarCursor + 1) % vbarCacheSize
			}

			// Paint the vbar column into the output.
			// Use the minimum of vbar height and band height to avoid
			// reading past the vbar data or writing past the band.
			// Do NOT mutate the cached vbar entry — it may be referenced
			// again from bands with different heights.
			x := xStart + col
			if x >= dstWidth {
				continue
			}
			count := vBarHeight
			if vbar.count != vBarHeight {
				d.log.LogAttrs(context.Background(), slog.LevelWarn, "vbar count mismatch",
					slog.Int("vbarCount", vbar.count), slog.Int("vBarHeight", vBarHeight))
			}
			if vbar.count < count {
				count = vbar.count
			}
			if count > dstHeight-yStart {
				count = dstHeight - yStart
			}
			for row := 0; row < count; row++ {
				dstOff := (yStart+row)*dstStride + x*4
				srcOff := row * 4
				if dstOff+4 <= len(dst) && srcOff+4 <= len(vbar.pixels) {
					copy(dst[dstOff:dstOff+4], vbar.pixels[srcOff:srcOff+4])
				}
			}
		}
	}
	return nil
}

// constructFullVBar builds a full vbar from a short vbar entry with background fill.
// Returns a temporary vbarEntry (caller stores it in VBarStorage).
func (d *Decoder) constructFullVBar(height, yOn, shortCount int,
	shortEntry *vbarEntry, bgB, bgG, bgR byte) *vbarEntry {

	need := height * 4
	if cap(d.vbarTmpPixels) < need {
		d.vbarTmpPixels = make([]byte, need)
	}
	d.vbarTmpPixels = d.vbarTmpPixels[:need]
	entry := &vbarEntry{count: height, pixels: d.vbarTmpPixels}

	// Fill background rows before yOn
	for row := 0; row < yOn && row < height; row++ {
		off := row * 4
		entry.pixels[off] = bgR
		entry.pixels[off+1] = bgG
		entry.pixels[off+2] = bgB
		entry.pixels[off+3] = 0xFF
	}

	// Copy short vbar pixels
	count := shortCount
	if yOn+count > height {
		count = height - yOn
		if count < 0 {
			count = 0
		}
	}
	for row := 0; row < count; row++ {
		dstOff := (yOn + row) * 4
		srcOff := row * 4
		if srcOff+4 <= len(shortEntry.pixels) {
			copy(entry.pixels[dstOff:dstOff+4], shortEntry.pixels[srcOff:srcOff+4])
		}
	}

	// Fill background rows after short vbar
	for row := yOn + shortCount; row < height; row++ {
		off := row * 4
		entry.pixels[off] = bgR
		entry.pixels[off+1] = bgG
		entry.pixels[off+2] = bgB
		entry.pixels[off+3] = 0xFF
	}

	return entry
}

// decodeSubcodec decodes the subcodec layer (raw BGR, RLEX, or NSCodec rects).
func (d *Decoder) decodeSubcodec(dst []byte, dstWidth int, src []byte) error {
	si := 0
	dstStride := dstWidth * 4

	for si < len(src) {
		// Subcodec rect header: x(2) + y(2) + w(2) + h(2) + dataLen(4) + subcodecId(1) = 13
		if si+13 > len(src) {
			return errTruncated
		}
		x := int(binary.LittleEndian.Uint16(src[si : si+2]))
		y := int(binary.LittleEndian.Uint16(src[si+2 : si+4]))
		w := int(binary.LittleEndian.Uint16(src[si+4 : si+6]))
		h := int(binary.LittleEndian.Uint16(src[si+6 : si+8]))
		dataLen := int(binary.LittleEndian.Uint32(src[si+8 : si+12]))
		subcodecId := src[si+12]
		si += 13

		if si+dataLen > len(src) {
			return errTruncated
		}
		rectData := src[si : si+dataLen]
		si += dataLen

		switch subcodecId {
		case 0: // Raw BGR24
			d.decodeRawBGR(dst, dstStride, x, y, w, h, rectData)
		case 1: // NSCodec
			if err := d.decodeNSCodec(dst, dstStride, x, y, w, h, rectData); err != nil {
				return err
			}
		case 2: // RLEX
			if err := d.decodeRLEX(dst, dstStride, x, y, w, h, rectData); err != nil {
				return err
			}
		default:
			// Unknown subcodec
		}
	}
	return nil
}

// decodeRawBGR writes raw BGR24 data to the output buffer.
func (d *Decoder) decodeRawBGR(dst []byte, dstStride, x, y, w, h int, src []byte) {
	si := 0
	for row := 0; row < h; row++ {
		for col := 0; col < w; col++ {
			if si+3 > len(src) {
				return
			}
			dstOff := (y+row)*dstStride + (x+col)*4
			if dstOff+4 <= len(dst) {
				dst[dstOff] = src[si+2]   // R
				dst[dstOff+1] = src[si+1] // G
				dst[dstOff+2] = src[si]   // B
				dst[dstOff+3] = 0xFF
			}
			si += 3
		}
	}
}

// decodeNSCodec decodes an NSCodec-compressed subrect and writes it to the output buffer.
// No row flip — NSCodec output within ClearCodec
// is already top-down, matching ClearCodec's output format.
func (d *Decoder) decodeNSCodec(dst []byte, dstStride, x, y, w, h int, src []byte) error {
	var err error
	d.nscBuf, d.nscPlanesBuf, err = nscodec.Decompress(d.nscBuf[:0], d.nscPlanesBuf, w, h, src, d.log)
	if err != nil {
		return fmt.Errorf("clearcodec: NSCodec subcodec: %w", err)
	}
	srcStride := w * 4
	for row := 0; row < h; row++ {
		srcOff := row * srcStride
		dstOff := (y+row)*dstStride + x*4
		if srcOff+srcStride <= len(d.nscBuf) && dstOff+srcStride <= len(dst) {
			copy(dst[dstOff:dstOff+srcStride], d.nscBuf[srcOff:srcOff+srcStride])
		}
	}
	return nil
}

// RLEX bit masks for extracting stopIndex from packed byte.
// CLEAR_8BIT_MASKS[n] = (1 << n) - 1
var clear8BitMasks = [9]byte{0x00, 0x01, 0x03, 0x07, 0x0F, 0x1F, 0x3F, 0x7F, 0xFF}

// CLEAR_LOG2_FLOOR[n] = floor(log2(n)) for n >= 1
var clearLog2Floor [128]byte

func init() {
	for i := 1; i < 128; i++ {
		v := i
		for v > 1 {
			v >>= 1
			clearLog2Floor[i]++
		}
	}
}

// decodeRLEX decodes RLEX-compressed data (palette + suite sweep run-length encoding).
// Decodes RLEX-compressed subcodec data (MS-RDPEGFX 2.2.4.1.3).
func (d *Decoder) decodeRLEX(dst []byte, dstStride, rectX, rectY, rectW, rectH int, src []byte) error {
	if len(src) < 1 {
		return fmt.Errorf("clearcodec: RLEX data empty")
	}

	paletteCount := int(src[0])
	si := 1

	if paletteCount < 1 || paletteCount > 127 {
		return fmt.Errorf("clearcodec: RLEX paletteCount %d out of range [1,127]", paletteCount)
	}

	// Read palette (BGR on wire → RGBA output order)
	if si+paletteCount*3 > len(src) {
		return errTruncated
	}
	type color struct{ r, g, b byte }
	palette := make([]color, paletteCount)
	for i := 0; i < paletteCount; i++ {
		palette[i] = color{src[si+2], src[si+1], src[si]}
		si += 3
	}

	// Determine bits per index: floor(log2(paletteCount - 1)) + 1
	numBits := int(clearLog2Floor[paletteCount-1]) + 1

	totalPixels := rectW * rectH
	pixelIndex := 0
	x, y := 0, 0

	for si < len(src) && pixelIndex < totalPixels {
		// Read 2 bytes per entry: packed tmp + runLengthFactor
		if si+2 > len(src) {
			return errTruncated
		}
		tmp := src[si]
		runLengthFactor := uint32(src[si+1])
		si += 2

		// Extract suiteDepth and stopIndex from tmp
		suiteDepth := int((tmp >> uint(numBits)) & clear8BitMasks[8-numBits])
		stopIndex := int(tmp & clear8BitMasks[numBits])
		startIndex := stopIndex - suiteDepth

		// Cascading run length factor (replace semantics, same as residual)
		if runLengthFactor >= 0xFF {
			if si+2 > len(src) {
				return errTruncated
			}
			runLengthFactor = uint32(binary.LittleEndian.Uint16(src[si : si+2]))
			si += 2
			if runLengthFactor >= 0xFFFF {
				if si+4 > len(src) {
					return errTruncated
				}
				runLengthFactor = binary.LittleEndian.Uint32(src[si : si+4])
				si += 4
			}
		}

		if startIndex < 0 || startIndex >= paletteCount || stopIndex >= paletteCount {
			return fmt.Errorf("clearcodec: RLEX palette index out of range (start=%d stop=%d count=%d)", startIndex, stopIndex, paletteCount)
		}

		// Paint runLengthFactor pixels at palette[startIndex]
		c := palette[startIndex]
		rlf := int(runLengthFactor)
		if pixelIndex+rlf > totalPixels {
			return errOverflow
		}
		for i := 0; i < rlf; i++ {
			dstOff := (rectY+y)*dstStride + (rectX+x)*4
			if dstOff+4 <= len(dst) {
				dst[dstOff] = c.r
				dst[dstOff+1] = c.g
				dst[dstOff+2] = c.b
				dst[dstOff+3] = 0xFF
			}
			x++
			if x >= rectW {
				x = 0
				y++
			}
		}
		pixelIndex += rlf

		// Suite sweep: paint suiteDepth+1 pixels sweeping palette[startIndex..stopIndex]
		suiteLen := suiteDepth + 1
		if pixelIndex+suiteLen > totalPixels {
			return errOverflow
		}
		suiteIdx := startIndex
		for i := 0; i < suiteLen; i++ {
			if suiteIdx >= paletteCount {
				return fmt.Errorf("clearcodec: RLEX suite index %d out of range (count=%d)", suiteIdx, paletteCount)
			}
			sc := palette[suiteIdx]
			suiteIdx++
			dstOff := (rectY+y)*dstStride + (rectX+x)*4
			if dstOff+4 <= len(dst) {
				dst[dstOff] = sc.r
				dst[dstOff+1] = sc.g
				dst[dstOff+2] = sc.b
				dst[dstOff+3] = 0xFF
			}
			x++
			if x >= rectW {
				x = 0
				y++
			}
		}
		pixelIndex += suiteLen
	}
	return nil
}

// Error sentinels
var (
	errShortHeader   = clearcodecError("clearcodec: header too short")
	errTruncated     = clearcodecError("clearcodec: data truncated")
	errBadVBarIndex  = clearcodecError("clearcodec: vbar index out of range")
	errBadVBarHeader = clearcodecError("clearcodec: invalid vbar header")
	errBadVBarYOff   = clearcodecError("clearcodec: vBarYOff < vBarYOn")
	errBadVBarHeight = clearcodecError("clearcodec: vbar height exceeds limit")
	errBadBandRect   = clearcodecError("clearcodec: invalid band rectangle")
	errBadGlyphFlags = clearcodecError("clearcodec: GLYPH_HIT without GLYPH_INDEX")
	errOverflow      = clearcodecError("clearcodec: pixel count overflow")
	errCacheFull     = clearcodecError("clearcodec: cache cursor overflow")
)

type clearcodecError string

func (e clearcodecError) Error() string { return string(e) }
