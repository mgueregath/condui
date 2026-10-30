package orders

import (
	"encoding/binary"
)

// RenderOpaqueRect renders an OpaqueRect order into an RGBA pixel buffer.
// serverBpp is needed when the server uses 15/16bpp and the three color bytes
// encode a 16-bit value rather than individual R/G/B components.
func RenderOpaqueRect(buf []byte, s *OpaqueRectState, serverBpp int, pal *[256][3]byte) (
	pixels []byte, x, y, w, h int, outBuf []byte) {
	w = int(s.Width)
	h = int(s.Height)
	if w <= 0 || h <= 0 {
		return nil, 0, 0, 0, 0, buf
	}
	need := w * h * 4
	if cap(buf) >= need {
		buf = buf[:need]
	} else {
		buf = make([]byte, need)
	}
	// Reconstruct packed color and convert via ColourToRGBA
	packed := uint32(s.ColorR) | uint32(s.ColorG)<<8 | uint32(s.ColorB)<<16
	cr, cg, cb := ColourToRGBA(packed, serverBpp, pal)
	for i := 0; i < need; i += 4 {
		buf[i] = cr
		buf[i+1] = cg
		buf[i+2] = cb
		buf[i+3] = 0xFF
	}
	return buf, int(s.Left), int(s.Top), w, h, buf
}

// RenderGlyphIndex renders a GlyphIndex primary order into an RGBA pixel buffer.
// buf is a reusable pixel buffer that may be grown. Returns the pixel data,
// position/size, and the (possibly grown) buffer for reuse.
// Transparent background pixels have alpha=0; use BlendRect to blit.
func RenderGlyphIndex(buf []byte, s *GlyphIndexState, cache *GlyphCache, serverBpp int, pal *[256][3]byte) (
	pixels []byte, x, y, w, h int, outBuf []byte) {
	// MS-RDPEGDI backColor/foreColor names are swapped from their actual usage
	// (field 4 = FGColour, field 5 = BGColour per MS-RDPEGDI 2.2.2.2.1.2.5).
	return renderGlyphs(buf, cache, s.CacheID, s.FlAccel, s.UlCharInc,
		s.ForeColor, s.BackColor,
		s.BkLeft, s.BkTop, s.BkRight, s.BkBottom,
		s.OpLeft, s.OpTop, s.OpRight, s.OpBottom,
		s.X, s.Y, s.VarBytes[:s.VarLen], serverBpp, pal, s.FOpRedundant)
}

// RenderFastIndex renders a FastIndex primary order into an RGBA pixel buffer.
// Transparent background pixels have alpha=0; use BlendRect to blit.
func RenderFastIndex(buf []byte, s *FastIndexState, cache *GlyphCache, serverBpp int, pal *[256][3]byte) (
	pixels []byte, x, y, w, h int, outBuf []byte) {
	ulCharInc := uint8(s.FDrawing)
	flAccel := uint8(s.FDrawing >> 8)
	return renderGlyphs(buf, cache, s.CacheID, flAccel, ulCharInc,
		s.ForeColor, s.BackColor,
		s.BkLeft, s.BkTop, s.BkRight, s.BkBottom,
		s.OpLeft, s.OpTop, s.OpRight, s.OpBottom,
		s.X, s.Y, s.VarBytes[:s.VarLen], serverBpp, pal, 0)
}

// renderGlyphs renders glyph text into a pixel buffer.
// mixmode: 0=transparent (no bg fill unless opaque rect valid), 1=opaque (fill bk rect with bgColor).
// Only fills background when boxcx>1 or mixmode==opaque (MS-RDPEGDI 2.2.2.2.1.2.5).
func renderGlyphs(buf []byte, cache *GlyphCache,
	cacheID, flAccel, ulCharInc uint8,
	backColor, foreColor uint32,
	bkLeft, bkTop, bkRight, bkBottom int16,
	opLeft, opTop, opRight, opBottom int16,
	glyphX, glyphY int16, varData []byte, serverBpp int, pal *[256][3]byte, mixmode uint8) (
	pixels []byte, rx, ry, rw, rh int, outBuf []byte) {

	// MS-RDPEGDI 2.2.2.2.1.1.2.13/14: x=0x8000 sentinel → use bkLeft
	if glyphX == -32768 {
		glyphX = bkLeft
	}

	// Determine output rectangle from bk rect
	left, top := int(bkLeft), int(bkTop)
	right, bottom := int(bkRight), int(bkBottom)

	// If opaque rect is valid and larger, use it for output size
	opW := int(opRight) - int(opLeft)
	opH := int(opBottom) - int(opTop)
	if opW > 0 && opH > 0 {
		if opW > right-left || opH > bottom-top {
			left, top = int(opLeft), int(opTop)
			right, bottom = int(opRight), int(opBottom)
		}
	}

	w := right - left
	h := bottom - top
	if w <= 0 || h <= 0 {
		return nil, 0, 0, 0, 0, buf
	}

	// Ensure buffer is large enough
	need := w * h * 4
	if cap(buf) >= need {
		buf = buf[:need]
	} else {
		buf = make([]byte, need)
	}

	// Initialize buffer as fully transparent (alpha=0).
	clear(buf[:need])

	bgR, bgG, bgB := ColourToRGBA(backColor, serverBpp, pal)

	// Background fill (MS-RDPEGDI 2.2.2.2.1.2.5 opaque rectangle)
	if opW > 1 && opH > 0 {
		fillRect(buf, w, h, left, top,
			int(opLeft), int(opTop), int(opRight), int(opBottom),
			bgR, bgG, bgB)
	} else if mixmode == 1 {
		fillRect(buf, w, h, left, top,
			int(bkLeft), int(bkTop), int(bkRight), int(bkBottom),
			bgR, bgG, bgB)
	}

	// Walk variable bytes to render glyphs
	fgR, fgG, fgB := ColourToRGBA(foreColor, serverBpp, pal)

	curX := int(glyphX)
	curY := int(glyphY)

	curX = walkGlyphs(buf, w, h, left, top, cache, cacheID, flAccel, ulCharInc,
		fgR, fgG, fgB, varData, curX, curY, 0)

	return buf, left, top, w, h, buf
}

// walkGlyphs processes a glyph variable-data stream, handling individual glyph
// indices and fragment opcodes per MS-RDPEGDI 2.2.2.2.1.1.2.13:
//   - USE (0xFE): replay a cached fragment
//   - ADD (0xFF): trailing marker — cache the preceding glyph bytes
//
// depth limits recursion to prevent loops from malformed fragment references.
// Returns the updated curX position.
func walkGlyphs(buf []byte, w, h, left, top int, cache *GlyphCache,
	cacheID, flAccel, ulCharInc uint8,
	fgR, fgG, fgB uint8, varData []byte, curX, curY int, depth int) int {

	if depth > 1 {
		return curX
	}

	varPitch := ulCharInc == 0 && flAccel&0x20 == 0 // SO_CHAR_INC_EQUAL_BM_BASE

	vi := 0
	vlen := len(varData)

	for vi < vlen {
		b := varData[vi]
		vi++

		if b == 0xFE {
			// USE — replay a cached fragment, then read optional delta
			if vi >= vlen {
				break
			}
			fragIndex := varData[vi]
			vi++
			frag := cache.GetFragment(fragIndex)
			if frag != nil {
				curX = walkGlyphs(buf, w, h, left, top, cache, cacheID,
					flAccel, ulCharInc, fgR, fgG, fgB, frag, curX, curY, depth+1)
			}
			// Per spec: USE index is followed by a delta if variable-pitch
			if varPitch && vi < vlen {
				vi, curX = readGlyphDelta(varData, vi, vlen, curX)
			}
			continue
		}

		if b == 0xFF {
			// ADD — trailing marker: cache the preceding glyph bytes.
			// Format: <already-processed glyph data> 0xFF index(1) size(1)
			// The data was already rendered; we just store it.
			if vi+1 >= vlen {
				break
			}
			fragIndex := varData[vi]
			vi++
			fragSize := int(varData[vi])
			vi++
			// The 0xFF was at vi-3; fragment data is the fragSize bytes before it
			addPos := vi - 3
			start := addPos - fragSize
			if start >= 0 && start < addPos {
				cache.SetFragment(fragIndex, varData[start:addPos])
			}
			continue
		}

		// Regular glyph index (0x00–0xFD)
		glyph := cache.Get(cacheID, b)

		// Read delta BEFORE drawing when variable-pitch (MS-RDPEGDI 2.2.2.2.1.2.5.1).
		if varPitch && vi < vlen {
			vi, curX = readGlyphDelta(varData, vi, vlen, curX)
		}

		if glyph == nil {
			// Advance even for missing glyphs (consume delta above, advance by ulCharInc)
			if ulCharInc > 0 {
				curX += int(ulCharInc)
			}
			continue
		}

		// Blit the glyph
		dstX := curX + int(glyph.X)
		dstY := curY + int(glyph.Y)
		blitGlyph1bpp(buf, w, h, left, top, dstX, dstY,
			glyph.Data, int(glyph.CX), int(glyph.CY),
			fgR, fgG, fgB)

		// Advance cursor (MS-RDPEGDI 2.2.2.2.1.2.5.1)
		if flAccel&0x20 != 0 {
			// SO_CHAR_INC_EQUAL_BM_BASE / Text2ImplicitX: advance by glyph width
			curX += int(glyph.CX)
		} else if ulCharInc > 0 {
			curX += int(ulCharInc)
		}
	}

	return curX
}

// readGlyphDelta reads a variable-length delta from the glyph stream.
// If the first byte is 0x80, the next two bytes are a LE uint16 distance.
// Returns the updated offset and curX.
func readGlyphDelta(data []byte, off, dlen, curX int) (int, int) {
	dx := int(int8(data[off]))
	off++
	if dx&0x80 != 0 && off+1 < dlen {
		dx = int(int16(binary.LittleEndian.Uint16(data[off:])))
		off += 2
	}
	return off, curX + dx
}

// blitGlyph1bpp renders a 1bpp glyph bitmap onto an RGBA destination buffer.
// The destination is bottom-up (row 0 = bottom scanline).
// src is MSB-first, byte-aligned rows.
func blitGlyph1bpp(dst []byte, dstW, dstH, dstOriginX, dstOriginY, dstX, dstY int,
	src []byte, srcW, srcH int, fgR, fgG, fgB uint8) {

	srcRowBytes := (srcW + 7) / 8

	for sy := 0; sy < srcH; sy++ {
		// Destination pixel Y (top-down logical) → bottom-up buffer row
		py := dstY + sy - dstOriginY
		if py < 0 || py >= dstH {
			continue
		}
		// Bottom-up: row 0 in buffer is the bottom row
		bufRow := (dstH - 1 - py) * dstW * 4

		srcRowOff := sy * srcRowBytes

		for sx := 0; sx < srcW; sx++ {
			px := dstX + sx - dstOriginX
			if px < 0 || px >= dstW {
				continue
			}

			// MSB-first bit order
			byteIdx := srcRowOff + sx/8
			if byteIdx >= len(src) {
				continue
			}
			bitIdx := 7 - (sx & 7)
			if src[byteIdx]&(1<<bitIdx) != 0 {
				off := bufRow + px*4
				dst[off] = fgR
				dst[off+1] = fgG
				dst[off+2] = fgB
				dst[off+3] = 0xFF
			}
		}
	}
}

// fillRect fills a rectangle within the destination buffer.
func fillRect(dst []byte, dstW, dstH, originX, originY int,
	rectLeft, rectTop, rectRight, rectBottom int,
	r, g, b uint8) {
	for py := rectTop - originY; py < rectBottom-originY; py++ {
		if py < 0 || py >= dstH {
			continue
		}
		bufRow := (dstH - 1 - py) * dstW * 4
		for px := rectLeft - originX; px < rectRight-originX; px++ {
			if px < 0 || px >= dstW {
				continue
			}
			off := bufRow + px*4
			dst[off] = r
			dst[off+1] = g
			dst[off+2] = b
			dst[off+3] = 0xFF
		}
	}
}

// dirtyRect tracks the bounding box of modified pixels.
type dirtyRect struct {
	x0, y0, x1, y1 int
	valid           bool
}

func (d *dirtyRect) add(x, y, w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	rx1 := x + w
	ry1 := y + h
	if !d.valid {
		d.x0, d.y0, d.x1, d.y1 = x, y, rx1, ry1
		d.valid = true
		return
	}
	if x < d.x0 {
		d.x0 = x
	}
	if y < d.y0 {
		d.y0 = y
	}
	if rx1 > d.x1 {
		d.x1 = rx1
	}
	if ry1 > d.y1 {
		d.y1 = ry1
	}
}

// RenderGlyphIndexFB renders a GlyphIndex order directly into the framebuffer.
// Returns the dirty bounding rectangle for onBitmap readback.
func RenderGlyphIndexFB(fb []byte, fbW, fbH int, s *GlyphIndexState, cache *GlyphCache, serverBpp int, pal *[256][3]byte) (
	x, y, w, h int) {
	return renderGlyphsFB(fb, fbW, fbH, cache, s.CacheID, s.FlAccel, s.UlCharInc,
		s.ForeColor, s.BackColor,
		s.BkLeft, s.BkTop, s.BkRight, s.BkBottom,
		s.OpLeft, s.OpTop, s.OpRight, s.OpBottom,
		s.X, s.Y, s.VarBytes[:s.VarLen], serverBpp, pal, s.FOpRedundant)
}

// RenderFastIndexFB renders a FastIndex order directly into the framebuffer.
func RenderFastIndexFB(fb []byte, fbW, fbH int, s *FastIndexState, cache *GlyphCache, serverBpp int, pal *[256][3]byte) (
	x, y, w, h int) {
	ulCharInc := uint8(s.FDrawing)
	flAccel := uint8(s.FDrawing >> 8)
	return renderGlyphsFB(fb, fbW, fbH, cache, s.CacheID, flAccel, ulCharInc,
		s.ForeColor, s.BackColor,
		s.BkLeft, s.BkTop, s.BkRight, s.BkBottom,
		s.OpLeft, s.OpTop, s.OpRight, s.OpBottom,
		s.X, s.Y, s.VarBytes[:s.VarLen], serverBpp, pal, 0)
}

// RenderFastGlyphFB renders a FastGlyph order directly into the framebuffer.
func RenderFastGlyphFB(fb []byte, fbW, fbH int, s *FastIndexState, cache *GlyphCache, serverBpp int, pal *[256][3]byte) (
	x, y, w, h int) {
	cacheID := s.CacheID
	ulCharInc := uint8(s.FDrawing)
	flAccel := uint8(s.FDrawing >> 8)

	varData := s.VarBytes[:s.VarLen]
	if len(varData) == 0 {
		return 0, 0, 0, 0
	}

	glyphIdx := varData[0]

	// If inline glyph data present (varLen > 1), cache it
	if len(varData) > 1 {
		gd := varData[1:]
		if len(gd) >= 8 {
			g := CachedGlyph{
				X:  int16(binary.LittleEndian.Uint16(gd[0:])),
				Y:  int16(binary.LittleEndian.Uint16(gd[2:])),
				CX: binary.LittleEndian.Uint16(gd[4:]),
				CY: binary.LittleEndian.Uint16(gd[6:]),
			}
			bitmapLen := int((g.CX+7)/8) * int(g.CY)
			if len(gd) >= 8+bitmapLen {
				g.Data = gd[8 : 8+bitmapLen]
				cache.Set(cacheID, glyphIdx, &g)
			}
		}
	}

	var singleGlyph [1]byte
	singleGlyph[0] = glyphIdx

	return renderGlyphsFB(fb, fbW, fbH, cache, cacheID, flAccel, ulCharInc,
		s.ForeColor, s.BackColor,
		s.BkLeft, s.BkTop, s.BkRight, s.BkBottom,
		s.OpLeft, s.OpTop, s.OpRight, s.OpBottom,
		s.X, s.Y, singleGlyph[:], serverBpp, pal, 0)
}

// renderGlyphsFB renders glyph text directly into the framebuffer.
// Background fill and glyph blitting happen at absolute screen coordinates
// with no intermediate buffer (MS-RDPEGDI 2.2.2.2.1.2.5).
func renderGlyphsFB(fb []byte, fbW, fbH int, cache *GlyphCache,
	cacheID, flAccel, ulCharInc uint8,
	backColor, foreColor uint32,
	bkLeft, bkTop, bkRight, bkBottom int16,
	opLeft, opTop, opRight, opBottom int16,
	glyphX, glyphY int16, varData []byte, serverBpp int, pal *[256][3]byte, mixmode uint8) (
	rx, ry, rw, rh int) {

	if fbW <= 0 || fbH <= 0 || len(fb) < fbW*fbH*4 {
		return 0, 0, 0, 0
	}

	// MS-RDPEGDI 2.2.2.2.1.1.2.13/14: x=0x8000 sentinel → use bkLeft
	if glyphX == -32768 {
		glyphX = bkLeft
	}

	// NOTE: opBottom=0x8000 sentinel is NOT converted here.
	// When opBottom is the sentinel, boxcy is very negative, so the
	// fillRect call draws nothing.

	bgR, bgG, bgB := ColourToRGBA(backColor, serverBpp, pal)
	boxW := int(opRight) - int(opLeft)
	boxH := int(opBottom) - int(opTop)

	var dirty dirtyRect

	// Background fill (MS-RDPEGDI 2.2.2.2.1.2.5 opaque rectangle).
	// Condition: boxcx > 1 (only checks width, not height;
	// negative boxH simply means fillRect draws nothing).
	if boxW > 1 {
		// Clamp to framebuffer width
		bw := boxW
		if int(opLeft)+bw > fbW {
			bw = fbW - int(opLeft)
		}
		if boxH > 0 {
			fillRectFB(fb, fbW, fbH, int(opLeft), int(opTop), bw, boxH, bgR, bgG, bgB)
			dirty.add(int(opLeft), int(opTop), bw, boxH)
		}
	} else if mixmode == 1 {
		clipW := int(bkRight) - int(bkLeft)
		clipH := int(bkBottom) - int(bkTop)
		if clipW > 0 && clipH > 0 {
			fillRectFB(fb, fbW, fbH, int(bkLeft), int(bkTop), clipW, clipH, bgR, bgG, bgB)
			dirty.add(int(bkLeft), int(bkTop), clipW, clipH)
		}
	}

	// Walk glyphs directly on framebuffer
	fgR, fgG, fgB := ColourToRGBA(foreColor, serverBpp, pal)
	curX := int(glyphX)
	curY := int(glyphY)

	curX = walkGlyphsFB(fb, fbW, fbH, cache, cacheID, flAccel, ulCharInc,
		fgR, fgG, fgB, varData, curX, curY, 0, &dirty)

	if !dirty.valid {
		return 0, 0, 0, 0
	}
	// Clamp dirty rect to framebuffer bounds
	if dirty.x0 < 0 {
		dirty.x0 = 0
	}
	if dirty.y0 < 0 {
		dirty.y0 = 0
	}
	if dirty.x1 > fbW {
		dirty.x1 = fbW
	}
	if dirty.y1 > fbH {
		dirty.y1 = fbH
	}
	return dirty.x0, dirty.y0, dirty.x1 - dirty.x0, dirty.y1 - dirty.y0
}

// walkGlyphsFB is like walkGlyphs but renders directly into the framebuffer.
func walkGlyphsFB(fb []byte, fbW, fbH int, cache *GlyphCache,
	cacheID, flAccel, ulCharInc uint8,
	fgR, fgG, fgB uint8, varData []byte, curX, curY int, depth int,
	dirty *dirtyRect) int {

	if depth > 1 {
		return curX
	}

	varPitch := ulCharInc == 0 && flAccel&0x20 == 0

	vi := 0
	vlen := len(varData)

	for vi < vlen {
		b := varData[vi]
		vi++

		if b == 0xFE {
			if vi >= vlen {
				break
			}
			fragIndex := varData[vi]
			vi++
			frag := cache.GetFragment(fragIndex)
			if frag != nil {
				curX = walkGlyphsFB(fb, fbW, fbH, cache, cacheID,
					flAccel, ulCharInc, fgR, fgG, fgB, frag, curX, curY, depth+1, dirty)
			}
			if varPitch && vi < vlen {
				vi, curX = readGlyphDelta(varData, vi, vlen, curX)
			}
			continue
		}

		if b == 0xFF {
			if vi+1 >= vlen {
				break
			}
			fragIndex := varData[vi]
			vi++
			fragSize := int(varData[vi])
			vi++
			addPos := vi - 3
			start := addPos - fragSize
			if start >= 0 && start < addPos {
				cache.SetFragment(fragIndex, varData[start:addPos])
			}
			continue
		}

		// Regular glyph index
		glyph := cache.Get(cacheID, b)

		if varPitch && vi < vlen {
			vi, curX = readGlyphDelta(varData, vi, vlen, curX)
		}

		if glyph == nil {
			if ulCharInc > 0 {
				curX += int(ulCharInc)
			}
			continue
		}

		// Blit glyph directly to framebuffer at absolute position
		dstX := curX + int(glyph.X)
		dstY := curY + int(glyph.Y)
		blitGlyph1bppFB(fb, fbW, fbH, dstX, dstY,
			glyph.Data, int(glyph.CX), int(glyph.CY),
			fgR, fgG, fgB, dirty)

		if flAccel&0x20 != 0 {
			curX += int(glyph.CX)
		} else if ulCharInc > 0 {
			curX += int(ulCharInc)
		}
	}

	return curX
}

// fillRectFB fills a rectangle directly in the framebuffer at absolute coords.
func fillRectFB(fb []byte, fbW, fbH, x, y, w, h int, r, g, b uint8) {
	stride := fbW * 4
	for py := y; py < y+h; py++ {
		if py < 0 || py >= fbH {
			continue
		}
		rowOff := (fbH - 1 - py) * stride
		for px := x; px < x+w; px++ {
			if px < 0 || px >= fbW {
				continue
			}
			off := rowOff + px*4
			fb[off] = r
			fb[off+1] = g
			fb[off+2] = b
			fb[off+3] = 0xFF
		}
	}
}

// blitGlyph1bppFB renders a 1bpp glyph directly into the framebuffer.
func blitGlyph1bppFB(fb []byte, fbW, fbH, dstX, dstY int,
	src []byte, srcW, srcH int, fgR, fgG, fgB uint8, dirty *dirtyRect) {

	srcRowBytes := (srcW + 7) / 8
	stride := fbW * 4

	// Add glyph bbox to dirty rect (clipped to screen)
	gx0 := dstX
	gy0 := dstY
	gx1 := dstX + srcW
	gy1 := dstY + srcH
	if gx0 < 0 {
		gx0 = 0
	}
	if gy0 < 0 {
		gy0 = 0
	}
	if gx1 > fbW {
		gx1 = fbW
	}
	if gy1 > fbH {
		gy1 = fbH
	}
	if gx0 < gx1 && gy0 < gy1 {
		dirty.add(gx0, gy0, gx1-gx0, gy1-gy0)
	}

	for sy := 0; sy < srcH; sy++ {
		py := dstY + sy
		if py < 0 || py >= fbH {
			continue
		}
		rowOff := (fbH - 1 - py) * stride
		srcRowOff := sy * srcRowBytes

		for sx := 0; sx < srcW; sx++ {
			px := dstX + sx
			if px < 0 || px >= fbW {
				continue
			}
			byteIdx := srcRowOff + sx/8
			if byteIdx >= len(src) {
				continue
			}
			bitIdx := 7 - (sx & 7)
			if src[byteIdx]&(1<<bitIdx) != 0 {
				off := rowOff + px*4
				fb[off] = fgR
				fb[off+1] = fgG
				fb[off+2] = fgB
				fb[off+3] = 0xFF
			}
		}
	}
}
