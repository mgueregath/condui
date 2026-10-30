package orders

import "encoding/binary"

// CachedGlyph stores a single cached glyph entry.
type CachedGlyph struct {
	X, Y   int16  // origin offset within glyph bitmap
	CX, CY uint16 // glyph bitmap dimensions
	Data   []byte // 1bpp bitmap, byte-aligned rows, MSB-first
}

// GlyphCache holds 10 glyph caches with up to 254 entries each,
// plus a 256-entry fragment cache for glyph fragment replay (MS-RDPEGDI 2.2.2.2.1.2.5).
type GlyphCache struct {
	caches   [10][254]CachedGlyph
	valid    [10][254]bool
	fragData [256][256]byte // fragment cache: 256 entries, up to 256 bytes each
	fragLen  [256]uint8     // length of each cached fragment
	fragSet  [256]bool      // whether fragment slot has been populated
}

// Set stores a glyph in the cache. It copies the data to avoid aliasing.
func (gc *GlyphCache) Set(cacheID, index uint8, g *CachedGlyph) {
	if cacheID >= 10 || index >= 254 {
		return
	}
	dst := &gc.caches[cacheID][index]
	dst.X = g.X
	dst.Y = g.Y
	dst.CX = g.CX
	dst.CY = g.CY
	// Reuse existing backing slice if large enough
	rowBytes := (int(g.CX) + 7) / 8
	need := rowBytes * int(g.CY)
	if cap(dst.Data) >= need {
		dst.Data = dst.Data[:need]
	} else {
		dst.Data = make([]byte, need)
	}
	copy(dst.Data, g.Data[:need])
	gc.valid[cacheID][index] = true
}

// Get returns a pointer to a cached glyph, or nil if not valid.
func (gc *GlyphCache) Get(cacheID, index uint8) *CachedGlyph {
	if cacheID >= 10 || index >= 254 {
		return nil
	}
	if !gc.valid[cacheID][index] {
		return nil
	}
	return &gc.caches[cacheID][index]
}

// SetFragment stores a glyph fragment in the fragment cache.
func (gc *GlyphCache) SetFragment(index uint8, data []byte) {
	n := len(data)
	if n > 256 {
		n = 256
	}
	copy(gc.fragData[index][:n], data[:n])
	gc.fragLen[index] = uint8(n)
	gc.fragSet[index] = true
}

// GetFragment returns the cached fragment data, or nil if not set.
func (gc *GlyphCache) GetFragment(index uint8) []byte {
	if !gc.fragSet[index] {
		return nil
	}
	return gc.fragData[index][:gc.fragLen[index]]
}

// DecodeCacheGlyph parses a Cache Glyph Rev 1 secondary order (orderType=0x03)
// and stores glyphs in the cache.
//
// Format: cacheId(u8), cGlyphs(u8), then per glyph:
//
//	cacheIndex(u16 LE), x(i16 LE), y(i16 LE), cx(u16 LE), cy(u16 LE), aj[](1bpp)
func DecodeCacheGlyph(cache *GlyphCache, data []byte) {
	if len(data) < 2 {
		return
	}
	cacheID := data[0]
	cGlyphs := data[1]
	off := 2

	var g CachedGlyph
	for i := uint8(0); i < cGlyphs; i++ {
		if off+10 > len(data) {
			return
		}
		cacheIndex := binary.LittleEndian.Uint16(data[off:])
		g.X = int16(binary.LittleEndian.Uint16(data[off+2:]))
		g.Y = int16(binary.LittleEndian.Uint16(data[off+4:]))
		g.CX = binary.LittleEndian.Uint16(data[off+6:])
		g.CY = binary.LittleEndian.Uint16(data[off+8:])
		off += 10

		rowBytes := (int(g.CX) + 7) / 8
		dataLen := rowBytes * int(g.CY)
		// Wire format pads glyph data to 4-byte alignment
		// (MS-RDPEGDI 2.2.2.2.1.3.1 glyph data alignment).
		wireLen := (dataLen + 3) & ^3
		if off+wireLen > len(data) {
			return
		}
		g.Data = data[off : off+dataLen]
		off += wireLen

		cache.Set(cacheID, uint8(cacheIndex), &g)
	}
}
