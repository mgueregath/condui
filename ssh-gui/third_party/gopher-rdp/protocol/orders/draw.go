package orders

import "encoding/binary"

// DrawLineTo draws a line on a bottom-up RGBA framebuffer using Bresenham's algorithm.
// Returns the bounding box (x, y, w, h) of modified pixels.
func DrawLineTo(fb []byte, fbW, fbH int, s *LineToState, serverBpp int, pal *[256][3]byte) (int, int, int, int) {
	x0, y0 := int(s.StartX), int(s.StartY)
	x1, y1 := int(s.EndX), int(s.EndY)

	// Bounding box
	bx0, by0 := x0, y0
	bx1, by1 := x1, y1
	if bx0 > bx1 {
		bx0, bx1 = bx1, bx0
	}
	if by0 > by1 {
		by0, by1 = by1, by0
	}
	bx1++
	by1++

	penR, penG, penB := ColourToRGBA(s.PenColor, serverBpp, pal)

	bresenham(fb, fbW, fbH, x0, y0, x1, y1, penR, penG, penB, s.Rop2)

	return bx0, by0, bx1 - bx0, by1 - by0
}

// DrawPolyline draws connected line segments on a bottom-up RGBA framebuffer.
// deltaBuf is a reusable scratch buffer for decoded deltas (grown if needed, returned).
// Returns the bounding box (x, y, w, h) and the (possibly grown) deltaBuf.
func DrawPolyline(fb []byte, fbW, fbH int, s *PolylineState, deltaBuf []int, serverBpp int, pal *[256][3]byte) (int, int, int, int, []int) {
	n := int(s.NumDeltaEntries)
	if n == 0 {
		return 0, 0, 0, 0, deltaBuf
	}

	penR, penG, penB := ColourToRGBA(s.PenColor, serverBpp, pal)
	rop := s.Rop2

	// Decode coded delta list into reusable buffer
	need := n * 2
	if cap(deltaBuf) >= need {
		deltaBuf = deltaBuf[:need]
	} else {
		deltaBuf = make([]int, need)
	}
	data := s.VarBytes[:s.VarLen]
	if !decodeDeltaPoints(data, n, deltaBuf) {
		return 0, 0, 0, 0, deltaBuf
	}

	// Bounding box tracking
	curX, curY := int(s.StartX), int(s.StartY)
	bx0, by0 := curX, curY
	bx1, by1 := curX, curY

	for i := 0; i < need; i += 2 {
		nextX := curX + deltaBuf[i]
		nextY := curY + deltaBuf[i+1]

		bresenham(fb, fbW, fbH, curX, curY, nextX, nextY, penR, penG, penB, rop)

		if nextX < bx0 {
			bx0 = nextX
		}
		if nextX > bx1 {
			bx1 = nextX
		}
		if nextY < by0 {
			by0 = nextY
		}
		if nextY > by1 {
			by1 = nextY
		}

		curX, curY = nextX, nextY
	}

	return bx0, by0, bx1 - bx0 + 1, by1 - by0 + 1, deltaBuf
}

// DrawEllipseSC draws a solid-color ellipse on a bottom-up RGBA framebuffer.
// Returns the bounding box (x, y, w, h) of modified pixels.
func DrawEllipseSC(fb []byte, fbW, fbH int, s *EllipseSCState, serverBpp int, pal *[256][3]byte) (int, int, int, int) {
	left := int(s.Left)
	top := int(s.Top)
	right := int(s.Right)
	bottom := int(s.Bottom)
	if right <= left || bottom <= top {
		return 0, 0, 0, 0
	}

	r, g, b := ColourToRGBA(s.Color, serverBpp, pal)
	rop := s.Rop2

	// Center and radii (doubled to avoid floating point)
	cx2 := left + right // 2*centerX
	cy2 := top + bottom // 2*centerY
	rx := right - left  // 2*radiusX
	ry := bottom - top  // 2*radiusY

	if s.FillMode != 0 {
		midpointEllipseFill(fb, fbW, fbH, cx2, cy2, rx, ry, r, g, b, rop)
	} else {
		midpointEllipseOutline(fb, fbW, fbH, cx2, cy2, rx, ry, r, g, b, rop)
	}

	return left, top, right - left, bottom - top
}

// DrawPolygonSC draws a solid-color filled polygon on a bottom-up RGBA framebuffer.
// deltaBuf is a reusable scratch buffer for decoded deltas (grown if needed, returned).
// Returns the bounding box and the (possibly grown) deltaBuf.
func DrawPolygonSC(fb []byte, fbW, fbH int, s *PolygonSCState, deltaBuf []int, serverBpp int, pal *[256][3]byte) (int, int, int, int, []int) {
	n := int(s.NumDeltaEntries)
	if n == 0 {
		return 0, 0, 0, 0, deltaBuf
	}

	r, g, b := ColourToRGBA(s.Color, serverBpp, pal)
	rop := s.Rop2

	// Decode coded delta list
	need := n * 2
	if cap(deltaBuf) >= need {
		deltaBuf = deltaBuf[:need]
	} else {
		deltaBuf = make([]int, need)
	}
	data := s.VarBytes[:s.VarLen]
	if !decodePolygonDeltas(data, n, deltaBuf) {
		return 0, 0, 0, 0, deltaBuf
	}

	// Build absolute points: points[0] = start, then accumulate deltas
	npts := n + 1
	pts := make([]polyPt, npts)
	pts[0] = polyPt{int(s.X), int(s.Y)}
	for i := 0; i < need; i += 2 {
		pts[i/2+1] = polyPt{pts[i/2].x + deltaBuf[i], pts[i/2].y + deltaBuf[i+1]}
	}

	// Bounding box
	bx0, by0 := pts[0].x, pts[0].y
	bx1, by1 := bx0, by0
	for i := 1; i < npts; i++ {
		if pts[i].x < bx0 {
			bx0 = pts[i].x
		}
		if pts[i].x > bx1 {
			bx1 = pts[i].x
		}
		if pts[i].y < by0 {
			by0 = pts[i].y
		}
		if pts[i].y > by1 {
			by1 = pts[i].y
		}
	}

	fillPolygon(fb, fbW, fbH, pts, r, g, b, rop)

	return bx0, by0, bx1 - bx0 + 1, by1 - by0 + 1, deltaBuf
}

// DrawPolygonCB draws a brush-filled polygon on a bottom-up RGBA framebuffer.
// brushMono/brushColor/colorBrush describe the resolved brush (same as PatBlt).
// deltaBuf is a reusable scratch buffer.
func DrawPolygonCB(fb []byte, fbW, fbH int, s *PolygonCBState, deltaBuf []int, serverBpp int,
	brushMono [8]byte, brushColorData [256]byte, colorBrush bool,
	fgR, fgG, fgB, bgR, bgG, bgB uint8,
	brushOrgX, brushOrgY int, rop2 uint8) (int, int, int, int, []int) {

	n := int(s.NumDeltaEntries)
	if n == 0 {
		return 0, 0, 0, 0, deltaBuf
	}

	// Decode coded delta list
	need := n * 2
	if cap(deltaBuf) >= need {
		deltaBuf = deltaBuf[:need]
	} else {
		deltaBuf = make([]int, need)
	}
	data := s.VarBytes[:s.VarLen]
	if !decodePolygonDeltas(data, n, deltaBuf) {
		return 0, 0, 0, 0, deltaBuf
	}

	// Build absolute points
	npts := n + 1
	pts := make([]polyPt, npts)
	pts[0] = polyPt{int(s.X), int(s.Y)}
	for i := 0; i < need; i += 2 {
		pts[i/2+1] = polyPt{pts[i/2].x + deltaBuf[i], pts[i/2].y + deltaBuf[i+1]}
	}

	// Bounding box
	bx0, by0 := pts[0].x, pts[0].y
	bx1, by1 := bx0, by0
	for i := 1; i < npts; i++ {
		if pts[i].x < bx0 {
			bx0 = pts[i].x
		}
		if pts[i].x > bx1 {
			bx1 = pts[i].x
		}
		if pts[i].y < by0 {
			by0 = pts[i].y
		}
		if pts[i].y > by1 {
			by1 = pts[i].y
		}
	}

	fillPolygonBrush(fb, fbW, fbH, pts,
		brushMono, brushColorData, colorBrush,
		fgR, fgG, fgB, bgR, bgG, bgB,
		brushOrgX, brushOrgY, rop2)

	return bx0, by0, bx1 - bx0 + 1, by1 - by0 + 1, deltaBuf
}

// DrawEllipseCB draws a brush-filled or brush-outlined ellipse.
func DrawEllipseCB(fb []byte, fbW, fbH int, s *EllipseCBState, serverBpp int,
	brushMono [8]byte, brushColorData [256]byte, colorBrush bool,
	fgR, fgG, fgB, bgR, bgG, bgB uint8,
	brushOrgX, brushOrgY int, rop2 uint8) (int, int, int, int) {

	left := int(s.Left)
	top := int(s.Top)
	right := int(s.Right)
	bottom := int(s.Bottom)
	if right <= left || bottom <= top {
		return 0, 0, 0, 0
	}

	cx2 := left + right
	cy2 := top + bottom
	rx := right - left
	ry := bottom - top

	if s.FillMode != 0 {
		midpointEllipseFillBrush(fb, fbW, fbH, cx2, cy2, rx, ry,
			brushMono, brushColorData, colorBrush,
			fgR, fgG, fgB, bgR, bgG, bgB,
			brushOrgX, brushOrgY, rop2)
	} else {
		// Outline only — use foreground color
		midpointEllipseOutline(fb, fbW, fbH, cx2, cy2, rx, ry, fgR, fgG, fgB, rop2)
	}

	return left, top, right - left, bottom - top
}

// polyPt is a 2D point for polygon fill.
type polyPt struct{ x, y int }

// decodePolygonDeltas decodes a polygon coded delta list.
// The format differs from polyline: flags pack 4 points per byte (2 bits each),
// bit clear = delta present (MS-RDPEGDI 2.2.2.2.1.4.7).
func decodePolygonDeltas(data []byte, numEntries int, dst []int) bool {
	if len(data) < 1 || numEntries <= 0 {
		return false
	}

	// Flags bytes come first, then delta values
	numFlagBytes := (numEntries-1)/4 + 1
	if len(data) < numFlagBytes {
		return false
	}
	flagData := data[:numFlagBytes]
	doff := numFlagBytes

	flagIdx := 0
	var flags byte

	for i := 0; i < numEntries; i++ {
		if i%4 == 0 {
			if flagIdx < len(flagData) {
				flags = flagData[flagIdx]
				flagIdx++
			}
		}

		// Bit 7 clear = X delta present (inverted test per MS-RDPEGDI 2.2.2.2.1.4.7)
		if ^flags&0x80 != 0 {
			var v int
			v, doff = parsePolygonDelta(data, doff)
			dst[i*2] = v
		} else {
			dst[i*2] = 0
		}

		// Bit 6 clear = Y delta present
		if ^flags&0x40 != 0 {
			var v int
			v, doff = parsePolygonDelta(data, doff)
			dst[i*2+1] = v
		} else {
			dst[i*2+1] = 0
		}

		flags <<= 2
	}
	return true
}

// parsePolygonDelta reads a 1 or 2 byte signed delta value.
// MS-RDPEGDI 2.2.2.2.1.4.7 coded delta encoding.
func parsePolygonDelta(data []byte, off int) (int, int) {
	if off >= len(data) {
		return 0, off
	}
	v := int(data[off])
	off++
	twoByte := v & 0x80

	if v&0x40 != 0 {
		v |= ^0x3F // sign extend
	} else {
		v &= 0x3F
	}

	if twoByte != 0 {
		if off >= len(data) {
			return int(int16(v)), off
		}
		v = (v << 8) | int(data[off])
		off++
	}

	return int(int16(v)), off
}

// fillPolygon fills a polygon using scanline rasterization with ROP2.
// Uses the even-odd (alternate) fill rule.
func fillPolygon(fb []byte, fbW, fbH int, pts []polyPt, r, g, b, rop2 uint8) {
	if len(pts) < 3 {
		return
	}

	// Find Y range
	minY, maxY := pts[0].y, pts[0].y
	for _, p := range pts[1:] {
		if p.y < minY {
			minY = p.y
		}
		if p.y > maxY {
			maxY = p.y
		}
	}

	nEdges := len(pts)
	var xIntersections [256]int

	for y := minY; y <= maxY; y++ {
		count := 0
		j := nEdges - 1
		for i := 0; i < nEdges; i++ {
			yi := pts[i].y
			yj := pts[j].y
			if (yi <= y && yj > y) || (yj <= y && yi > y) {
				// X intersection using integer math
				xi := pts[i].x + (y-yi)*(pts[j].x-pts[i].x)/(yj-yi)
				if count < len(xIntersections) {
					xIntersections[count] = xi
					count++
				}
			}
			j = i
		}

		// Sort intersections (insertion sort — typically very few)
		for a := 1; a < count; a++ {
			key := xIntersections[a]
			c := a - 1
			for c >= 0 && xIntersections[c] > key {
				xIntersections[c+1] = xIntersections[c]
				c--
			}
			xIntersections[c+1] = key
		}

		// Fill between pairs
		for a := 0; a+1 < count; a += 2 {
			x0, x1 := xIntersections[a], xIntersections[a+1]
			hline(fb, fbW, fbH, x0, x1, y, r, g, b, rop2)
		}
	}
}

// fillPolygonBrush fills a polygon with a brush pattern using scanline rasterization.
func fillPolygonBrush(fb []byte, fbW, fbH int, pts []polyPt,
	brushMono [8]byte, brushColorData [256]byte, colorBrush bool,
	fgR, fgG, fgB, bgR, bgG, bgB uint8,
	brushOrgX, brushOrgY int, rop2 uint8) {

	if len(pts) < 3 {
		return
	}

	minY, maxY := pts[0].y, pts[0].y
	for _, p := range pts[1:] {
		if p.y < minY {
			minY = p.y
		}
		if p.y > maxY {
			maxY = p.y
		}
	}

	nEdges := len(pts)
	var xIntersections [256]int

	for y := minY; y <= maxY; y++ {
		count := 0
		j := nEdges - 1
		for i := 0; i < nEdges; i++ {
			yi := pts[i].y
			yj := pts[j].y
			if (yi <= y && yj > y) || (yj <= y && yi > y) {
				xi := pts[i].x + (y-yi)*(pts[j].x-pts[i].x)/(yj-yi)
				if count < len(xIntersections) {
					xIntersections[count] = xi
					count++
				}
			}
			j = i
		}

		for a := 1; a < count; a++ {
			key := xIntersections[a]
			c := a - 1
			for c >= 0 && xIntersections[c] > key {
				xIntersections[c+1] = xIntersections[c]
				c--
			}
			xIntersections[c+1] = key
		}

		for a := 0; a+1 < count; a += 2 {
			x0, x1 := xIntersections[a], xIntersections[a+1]
			hlineBrush(fb, fbW, fbH, x0, x1, y,
				brushMono, brushColorData, colorBrush,
				fgR, fgG, fgB, bgR, bgG, bgB,
				brushOrgX, brushOrgY, rop2)
		}
	}
}

// hlineBrush draws a horizontal line with brush pattern.
func hlineBrush(fb []byte, fbW, fbH, x0, x1, y int,
	brushMono [8]byte, brushColorData [256]byte, colorBrush bool,
	fgR, fgG, fgB, bgR, bgG, bgB uint8,
	brushOrgX, brushOrgY int, rop2 uint8) {

	if uint(y) >= uint(fbH) {
		return
	}
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= fbW {
		x1 = fbW - 1
	}
	if x0 > x1 {
		return
	}

	row := (fbH - 1 - y) * fbW * 4

	if colorBrush {
		brushRowBase := ((y - brushOrgY) & 7) * 32
		for x := x0; x <= x1; x++ {
			bi := brushRowBase + ((x-brushOrgX)&7)*4
			setPixelROP2(fb, row+x*4, brushColorData[bi], brushColorData[bi+1], brushColorData[bi+2], rop2)
		}
	} else {
		patRow := brushMono[(y-brushOrgY)&7]
		for x := x0; x <= x1; x++ {
			var pr, pg, pb uint8
			if patRow&(0x80>>uint((x-brushOrgX)&7)) != 0 {
				pr, pg, pb = bgR, bgG, bgB
			} else {
				pr, pg, pb = fgR, fgG, fgB
			}
			setPixelROP2(fb, row+x*4, pr, pg, pb, rop2)
		}
	}
}

// midpointEllipseFillBrush draws a filled ellipse with brush pattern.
func midpointEllipseFillBrush(fb []byte, fbW, fbH, cx2, cy2, rx, ry int,
	brushMono [8]byte, brushColorData [256]byte, colorBrush bool,
	fgR, fgG, fgB, bgR, bgG, bgB uint8,
	brushOrgX, brushOrgY int, rop2 uint8) {

	a := rx
	bb := ry
	if a <= 0 || bb <= 0 {
		return
	}

	a2 := int64(a) * int64(a)
	b2 := int64(bb) * int64(bb)

	x := 0
	y := bb / 2
	if ry%2 != 0 {
		y = ry / 2
	}

	d1 := b2 - a2*int64(y) + a2/4
	lastY := y + 1

	for b2*int64(x) <= a2*int64(y) {
		if y != lastY {
			lx := (cx2 - 2*x + 1) / 2
			rx2 := (cx2 + 2*x) / 2
			ty := (cy2 - 2*y + 1) / 2
			by := (cy2 + 2*y) / 2
			hlineBrush(fb, fbW, fbH, lx, rx2, ty,
				brushMono, brushColorData, colorBrush,
				fgR, fgG, fgB, bgR, bgG, bgB, brushOrgX, brushOrgY, rop2)
			if ty != by {
				hlineBrush(fb, fbW, fbH, lx, rx2, by,
					brushMono, brushColorData, colorBrush,
					fgR, fgG, fgB, bgR, bgG, bgB, brushOrgX, brushOrgY, rop2)
			}
			lastY = y
		}
		x++
		if d1 < 0 {
			d1 += b2 * int64(2*x+1)
		} else {
			y--
			d1 += b2*int64(2*x+1) - 2*a2*int64(y) - a2
		}
	}

	d2 := b2*int64(x*x+x) + a2*int64(y*y-2*y+1) - a2*b2/4
	for y >= 0 {
		lx := (cx2 - 2*x + 1) / 2
		rx2 := (cx2 + 2*x) / 2
		ty := (cy2 - 2*y + 1) / 2
		by := (cy2 + 2*y) / 2
		hlineBrush(fb, fbW, fbH, lx, rx2, ty,
			brushMono, brushColorData, colorBrush,
			fgR, fgG, fgB, bgR, bgG, bgB, brushOrgX, brushOrgY, rop2)
		if ty != by {
			hlineBrush(fb, fbW, fbH, lx, rx2, by,
				brushMono, brushColorData, colorBrush,
				fgR, fgG, fgB, bgR, bgG, bgB, brushOrgX, brushOrgY, rop2)
		}
		y--
		if d2 > 0 {
			d2 -= a2 * int64(2*y+1)
		} else {
			x++
			d2 += b2*int64(2*x+1) - a2*int64(2*y+1)
		}
	}
}

// RenderFastGlyph renders a FastGlyph primary order.
// It caches the inline glyph (if present) then renders the single glyph.
func RenderFastGlyph(buf []byte, s *FastIndexState, cache *GlyphCache, serverBpp int, pal *[256][3]byte) (
	pixels []byte, x, y, w, h int, outBuf []byte) {
	cacheID := s.CacheID
	ulCharInc := uint8(s.FDrawing)
	flAccel := uint8(s.FDrawing >> 8)

	varData := s.VarBytes[:s.VarLen]
	if len(varData) == 0 {
		return nil, 0, 0, 0, 0, buf
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

	// Render the single glyph
	var singleGlyph [1]byte
	singleGlyph[0] = glyphIdx

	return renderGlyphs(buf, cache, cacheID, flAccel, ulCharInc,
		s.ForeColor, s.BackColor,
		s.BkLeft, s.BkTop, s.BkRight, s.BkBottom,
		s.OpLeft, s.OpTop, s.OpRight, s.OpBottom,
		s.X, s.Y, singleGlyph[:], serverBpp, pal, 0)
}

// bresenham draws a line from (x0,y0) to (x1,y1) on a bottom-up RGBA framebuffer.
func bresenham(fb []byte, fbW, fbH, x0, y0, x1, y1 int, penR, penG, penB, rop2 uint8) {
	dx := x1 - x0
	dy := y1 - y0
	sx := 1
	sy := 1
	if dx < 0 {
		dx = -dx
		sx = -1
	}
	if dy < 0 {
		dy = -dy
		sy = -1
	}

	err := dx - dy
	stride := fbW * 4

	// COPYPEN fast path — no switch per pixel, no destination read
	if rop2 == 0x0D {
		for {
			if uint(x0) < uint(fbW) && uint(y0) < uint(fbH) {
				off := (fbH-1-y0)*stride + x0*4
				fb[off] = penR
				fb[off+1] = penG
				fb[off+2] = penB
				fb[off+3] = 0xFF
			}
			if x0 == x1 && y0 == y1 {
				break
			}
			e2 := 2 * err
			if e2 > -dy {
				err -= dy
				x0 += sx
			}
			if e2 < dx {
				err += dx
				y0 += sy
			}
		}
		return
	}

	// Generic ROP2 path
	for {
		if uint(x0) < uint(fbW) && uint(y0) < uint(fbH) {
			off := (fbH-1-y0)*stride + x0*4
			setPixelROP2(fb, off, penR, penG, penB, rop2)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// setPixelROP2 writes a pixel at fb[off] using ROP2 mix mode.
// Only called for non-COPYPEN modes (COPYPEN is inlined in callers).
func setPixelROP2(fb []byte, off int, r, g, b, rop2 uint8) {
	switch rop2 {
	case 0x01: // R2_BLACK
		fb[off] = 0
		fb[off+1] = 0
		fb[off+2] = 0
		fb[off+3] = 0xFF
	case 0x10: // R2_WHITE
		fb[off] = 0xFF
		fb[off+1] = 0xFF
		fb[off+2] = 0xFF
		fb[off+3] = 0xFF
	case 0x06: // R2_NOT — invert destination
		fb[off] ^= 0xFF
		fb[off+1] ^= 0xFF
		fb[off+2] ^= 0xFF
	case 0x07: // R2_XORPEN
		fb[off] ^= r
		fb[off+1] ^= g
		fb[off+2] ^= b
	case 0x0B: // R2_NOP
		// do nothing
	case 0x0F: // R2_MERGEPEN — pen OR dest
		fb[off] |= r
		fb[off+1] |= g
		fb[off+2] |= b
	case 0x09: // R2_MASKPEN — pen AND dest
		fb[off] &= r
		fb[off+1] &= g
		fb[off+2] &= b
	default: // includes 0x0D (COPYPEN) if reached from non-fast-path callers
		fb[off] = r
		fb[off+1] = g
		fb[off+2] = b
		fb[off+3] = 0xFF
	}
}

// hline draws a horizontal line from (x0,y) to (x1,y) inclusive on a bottom-up RGBA fb.
func hline(fb []byte, fbW, fbH, x0, x1, y int, r, g, b, rop2 uint8) {
	if uint(y) >= uint(fbH) {
		return
	}
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= fbW {
		x1 = fbW - 1
	}
	if x0 > x1 {
		return
	}
	row := (fbH - 1 - y) * fbW * 4
	if rop2 == 0x0D { // R2_COPYPEN fast path
		for x := x0; x <= x1; x++ {
			off := row + x*4
			fb[off] = r
			fb[off+1] = g
			fb[off+2] = b
			fb[off+3] = 0xFF
		}
	} else {
		for x := x0; x <= x1; x++ {
			setPixelROP2(fb, row+x*4, r, g, b, rop2)
		}
	}
}

// midpointEllipseFill draws a filled ellipse using the midpoint algorithm.
// cx2, cy2 are 2*centerX, 2*centerY; rx, ry are 2*radiusX, 2*radiusY.
func midpointEllipseFill(fb []byte, fbW, fbH, cx2, cy2, rx, ry int, r, g, b, rop2 uint8) {
	a := rx  // 2*a
	bb := ry // 2*b
	if a <= 0 || bb <= 0 {
		return
	}

	a2 := int64(a) * int64(a)
	b2 := int64(bb) * int64(bb)

	x := 0
	y := bb / 2
	if ry%2 != 0 {
		y = ry / 2
	}

	d1 := b2 - a2*int64(y) + a2/4
	lastY := y + 1

	for b2*int64(x) <= a2*int64(y) {
		if y != lastY {
			lx := (cx2 - 2*x + 1) / 2
			rx2 := (cx2 + 2*x) / 2
			ty := (cy2 - 2*y + 1) / 2
			by := (cy2 + 2*y) / 2
			hline(fb, fbW, fbH, lx, rx2, ty, r, g, b, rop2)
			if ty != by {
				hline(fb, fbW, fbH, lx, rx2, by, r, g, b, rop2)
			}
			lastY = y
		}
		x++
		if d1 < 0 {
			d1 += b2 * int64(2*x+1)
		} else {
			y--
			d1 += b2*int64(2*x+1) - 2*a2*int64(y) - a2
		}
	}

	d2 := b2*int64(x*x+x) + a2*int64(y*y-2*y+1) - a2*b2/4
	for y >= 0 {
		lx := (cx2 - 2*x + 1) / 2
		rx2 := (cx2 + 2*x) / 2
		ty := (cy2 - 2*y + 1) / 2
		by := (cy2 + 2*y) / 2
		hline(fb, fbW, fbH, lx, rx2, ty, r, g, b, rop2)
		if ty != by {
			hline(fb, fbW, fbH, lx, rx2, by, r, g, b, rop2)
		}
		y--
		if d2 > 0 {
			d2 -= a2 * int64(2*y+1)
		} else {
			x++
			d2 += b2*int64(2*x+1) - a2*int64(2*y+1)
		}
	}
}

// midpointEllipseOutline draws an ellipse outline using the midpoint algorithm.
func midpointEllipseOutline(fb []byte, fbW, fbH, cx2, cy2, rx, ry int, r, g, b, rop2 uint8) {
	a := rx
	bb := ry
	if a <= 0 || bb <= 0 {
		return
	}

	a2 := int64(a) * int64(a)
	b2 := int64(bb) * int64(bb)
	stride := fbW * 4

	x := 0
	y := bb / 2
	if ry%2 != 0 {
		y = ry / 2
	}

	d1 := b2 - a2*int64(y) + a2/4
	for b2*int64(x) <= a2*int64(y) {
		ellipsePlot4(fb, fbW, fbH, stride, cx2, cy2, x, y, r, g, b, rop2)
		x++
		if d1 < 0 {
			d1 += b2 * int64(2*x+1)
		} else {
			y--
			d1 += b2*int64(2*x+1) - 2*a2*int64(y) - a2
		}
	}

	d2 := b2*int64(x*x+x) + a2*int64(y*y-2*y+1) - a2*b2/4
	for y >= 0 {
		ellipsePlot4(fb, fbW, fbH, stride, cx2, cy2, x, y, r, g, b, rop2)
		y--
		if d2 > 0 {
			d2 -= a2 * int64(2*y+1)
		} else {
			x++
			d2 += b2*int64(2*x+1) - a2*int64(2*y+1)
		}
	}
}

// ellipsePlot4 plots 4 symmetrical points of an ellipse. Inlined by the compiler.
func ellipsePlot4(fb []byte, fbW, fbH, stride, cx2, cy2, x, y int, r, g, b, rop2 uint8) {
	px0 := (cx2 + 2*x) / 2
	px1 := (cx2 - 2*x + 1) / 2
	py0 := (cy2 + 2*y) / 2
	py1 := (cy2 - 2*y + 1) / 2

	if rop2 == 0x0D { // COPYPEN fast path
		if uint(px0) < uint(fbW) && uint(py0) < uint(fbH) {
			off := (fbH-1-py0)*stride + px0*4
			fb[off] = r
			fb[off+1] = g
			fb[off+2] = b
			fb[off+3] = 0xFF
		}
		if uint(px1) < uint(fbW) && uint(py0) < uint(fbH) {
			off := (fbH-1-py0)*stride + px1*4
			fb[off] = r
			fb[off+1] = g
			fb[off+2] = b
			fb[off+3] = 0xFF
		}
		if uint(px0) < uint(fbW) && uint(py1) < uint(fbH) {
			off := (fbH-1-py1)*stride + px0*4
			fb[off] = r
			fb[off+1] = g
			fb[off+2] = b
			fb[off+3] = 0xFF
		}
		if uint(px1) < uint(fbW) && uint(py1) < uint(fbH) {
			off := (fbH-1-py1)*stride + px1*4
			fb[off] = r
			fb[off+1] = g
			fb[off+2] = b
			fb[off+3] = 0xFF
		}
	} else {
		if uint(px0) < uint(fbW) && uint(py0) < uint(fbH) {
			setPixelROP2(fb, (fbH-1-py0)*stride+px0*4, r, g, b, rop2)
		}
		if uint(px1) < uint(fbW) && uint(py0) < uint(fbH) {
			setPixelROP2(fb, (fbH-1-py0)*stride+px1*4, r, g, b, rop2)
		}
		if uint(px0) < uint(fbW) && uint(py1) < uint(fbH) {
			setPixelROP2(fb, (fbH-1-py1)*stride+px0*4, r, g, b, rop2)
		}
		if uint(px1) < uint(fbW) && uint(py1) < uint(fbH) {
			setPixelROP2(fb, (fbH-1-py1)*stride+px1*4, r, g, b, rop2)
		}
	}
}

// decodeDeltaPoints decodes a Coded Delta List (MS-RDPEGDI 2.2.2.2.1.1.1.1)
// into dst[0:numEntries*2] as [dx0, dy0, dx1, dy1, ...].
// dst must have length >= numEntries*2. Returns false on decode error.
func decodeDeltaPoints(data []byte, numEntries int, dst []int) bool {
	if len(data) < 1 || numEntries <= 0 {
		return false
	}
	cbData := int(data[0])
	data = data[1:]
	if len(data) < cbData {
		return false
	}
	data = data[:cbData]

	numFlagBytes := (numEntries + 3) / 4
	if len(data) < numFlagBytes {
		return false
	}
	flagData := data[:numFlagBytes]
	data = data[numFlagBytes:]

	flagIdx := 0
	var flags byte

	for i := 0; i < numEntries; i++ {
		if i%4 == 0 {
			if flagIdx < len(flagData) {
				flags = flagData[flagIdx]
				flagIdx++
			}
		}

		if flags&0x80 != 0 {
			dst[i*2] = 0
		} else {
			var v int
			v, data = readDelta(data)
			dst[i*2] = v
		}
		flags <<= 1

		if flags&0x80 != 0 {
			dst[i*2+1] = 0
		} else {
			var v int
			v, data = readDelta(data)
			dst[i*2+1] = v
		}
		flags <<= 1
	}

	return true
}

// readDelta reads a single coded delta value (1 or 2 bytes).
// Bit 7: 1=two bytes, 0=one byte. Bit 6: sign extension.
func readDelta(data []byte) (int, []byte) {
	if len(data) == 0 {
		return 0, data
	}
	b := data[0]
	data = data[1:]

	if b&0x80 != 0 {
		if len(data) == 0 {
			return 0, data
		}
		hi := b & 0x3F
		if b&0x40 != 0 {
			hi |= 0xC0
		}
		lo := data[0]
		data = data[1:]
		return int(int16(uint16(hi)<<8 | uint16(lo))), data
	}

	v := b & 0x3F
	if b&0x40 != 0 {
		return int(int8(v | 0xC0)), data
	}
	return int(v), data
}

// ApplyROP3 computes the ternary raster operation for one byte (branchless).
// The rop byte encodes a truth table indexed by (P<<2 | S<<1 | D).
func ApplyROP3(rop, pat, src, dst byte) byte {
	np, ns, nd := ^pat, ^src, ^dst
	// For each truth table entry, compute a mask that selects the bits matching
	// that (P,S,D) combination, then AND with 0xFF or 0x00 based on whether
	// the rop bit is set. Branchless: 0 - (rop>>N & 1) = 0xFF if set, 0x00 if not.
	return (np&ns&nd)&(0-rop&1) |
		(np&ns&dst)&(0-(rop>>1)&1) |
		(np&src&nd)&(0-(rop>>2)&1) |
		(np&src&dst)&(0-(rop>>3)&1) |
		(pat&ns&nd)&(0-(rop>>4)&1) |
		(pat&ns&dst)&(0-(rop>>5)&1) |
		(pat&src&nd)&(0-(rop>>6)&1) |
		(pat&src&dst)&(0-(rop>>7)&1)
}
