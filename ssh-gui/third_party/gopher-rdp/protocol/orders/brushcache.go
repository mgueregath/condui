package orders

// HatchPatterns contains the 6 built-in hatch patterns (MS-RDPEGDI).
// Each pattern is 8 rows of 8 bits, MSB-first.
var HatchPatterns = [6][8]byte{
	{0x00, 0x00, 0x00, 0xFF, 0x00, 0x00, 0x00, 0x00}, // HS_HORIZONTAL
	{0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08}, // HS_VERTICAL
	{0x80, 0x40, 0x20, 0x10, 0x08, 0x04, 0x02, 0x01}, // HS_FDIAGONAL
	{0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80}, // HS_BDIAGONAL
	{0x08, 0x08, 0x08, 0xFF, 0x08, 0x08, 0x08, 0x08}, // HS_CROSS
	{0x81, 0x42, 0x24, 0x18, 0x18, 0x24, 0x42, 0x81}, // HS_DIAGCROSS
}

// CachedBrush stores a single cached brush entry.
type CachedBrush struct {
	Mono     bool       // true = 1bpp monochrome
	MonoData [8]byte    // 8 rows, bottom-up, MSB-first (only if Mono)
	Data     [256]byte  // 8x8 RGBA pixels (only if !Mono)
}

// BrushCache holds up to 64 cached brush entries.
type BrushCache struct {
	entries [64]CachedBrush
	valid   [64]bool
}

// Set stores a brush in the cache.
func (bc *BrushCache) Set(index uint8, brush *CachedBrush) {
	if index >= 64 {
		return
	}
	bc.entries[index] = *brush
	bc.valid[index] = true
}

// Get returns a pointer to a cached brush, or nil if not valid.
func (bc *BrushCache) Get(index uint8) *CachedBrush {
	if index >= 64 {
		return nil
	}
	if !bc.valid[index] {
		return nil
	}
	return &bc.entries[index]
}

// Reset clears all entries.
func (bc *BrushCache) Reset() {
	bc.valid = [64]bool{}
}

// Bitmap format constants for TS_CACHE_BRUSH (MS-RDPEGDI 2.2.2.2.1.2.7).
const (
	bmfBpp1  = 0x01
	bmfBpp8  = 0x03
	bmfBpp16 = 0x04
	bmfBpp24 = 0x05
	bmfBpp32 = 0x06
)

// DecodeCacheBrush parses a TS_CACHE_BRUSH secondary order (orderType=0x07)
// and stores the brush in the cache.
//
// Wire format: cacheEntry(1) + iBitmapFormat(1) + cx(1) + cy(1) +
//
//	style(1) + iBytes(1) + brushData(variable)
func DecodeCacheBrush(cache *BrushCache, data []byte) {
	if len(data) < 6 {
		return
	}
	cacheEntry := data[0]
	bitmapFormat := data[1]
	// cx := data[2] // always 8
	// cy := data[3] // always 8
	// style := data[4]
	iBytes := data[5]
	brushData := data[6:]

	if int(iBytes) > len(brushData) {
		return
	}
	brushData = brushData[:iBytes]

	var brush CachedBrush

	switch bitmapFormat {
	case bmfBpp1:
		// 1bpp monochrome: 8 bytes, one per row (reversed per MS-RDPEGDI 2.2.2.2.1.2.7)
		if iBytes < 8 {
			return
		}
		brush.Mono = true
		for i := range 8 {
			brush.MonoData[7-i] = brushData[i]
		}

	default:
		// Color brush: may be compressed or uncompressed
		bpp := formatToBpp(bitmapFormat)
		if bpp == 0 {
			return
		}
		rawBytesPerPixel := bpp / 8
		if bpp == 15 {
			rawBytesPerPixel = 2
		}
		uncompSize := 8 * 8 * rawBytesPerPixel

		if int(iBytes) == uncompSize {
			// Uncompressed: raw pixel data → convert to BGRX
			expandColorBrush(&brush, brushData, bitmapFormat)
		} else {
			// Compressed: 2-bit index table (16 bytes) + color palette (4 entries)
			paletteEntrySize := rawBytesPerPixel
			paletteSize := 4 * paletteEntrySize
			indexSize := 16 // 64 pixels * 2 bits / 8 bits = 16 bytes
			if int(iBytes) < indexSize+paletteSize {
				return
			}
			decompressColorBrush(&brush, brushData[:indexSize], brushData[indexSize:], bitmapFormat)
		}
	}

	cache.Set(cacheEntry, &brush)
}

// formatToBpp returns the bits per pixel for a bitmap format code.
func formatToBpp(fmt byte) int {
	switch fmt {
	case bmfBpp8:
		return 8
	case bmfBpp16:
		return 16
	case bmfBpp24:
		return 24
	case bmfBpp32:
		return 32
	default:
		return 0
	}
}

// expandColorBrush converts uncompressed brush pixel data to RGBA.
func expandColorBrush(brush *CachedBrush, data []byte, bitmapFormat byte) {
	di := 0
	switch bitmapFormat {
	case bmfBpp32:
		// Wire format is BGRX — swap B↔R to RGBA.
		si := 0
		for i := 0; i < 64 && si+4 <= len(data); i++ {
			brush.Data[di] = data[si+2]   // R
			brush.Data[di+1] = data[si+1] // G
			brush.Data[di+2] = data[si]   // B
			brush.Data[di+3] = 0xFF
			si += 4
			di += 4
		}
		return
	case bmfBpp24:
		si := 0
		for i := 0; i < 64 && si+3 <= len(data); i++ {
			brush.Data[di] = data[si+2]   // R
			brush.Data[di+1] = data[si+1] // G
			brush.Data[di+2] = data[si]   // B
			brush.Data[di+3] = 0xFF
			si += 3
			di += 4
		}
	case bmfBpp16:
		si := 0
		for i := 0; i < 64 && si+2 <= len(data); i++ {
			v := uint16(data[si]) | uint16(data[si+1])<<8
			r := uint8((v >> 11) & 0x1F)
			g := uint8((v >> 5) & 0x3F)
			b := uint8(v & 0x1F)
			brush.Data[di] = (r << 3) | (r >> 2)
			brush.Data[di+1] = (g << 2) | (g >> 4)
			brush.Data[di+2] = (b << 3) | (b >> 2)
			brush.Data[di+3] = 0xFF
			si += 2
			di += 4
		}
	case bmfBpp8:
		for i := 0; i < 64 && i < len(data); i++ {
			v := data[i]
			brush.Data[di] = v
			brush.Data[di+1] = v
			brush.Data[di+2] = v
			brush.Data[di+3] = 0xFF
			di += 4
		}
	}
}

// decompressColorBrush expands a compressed color brush.
// The index table contains 64 2-bit indices packed into 16 bytes (4 pixels per byte).
// The palette follows with 4 color entries in the given bitmap format.
func decompressColorBrush(brush *CachedBrush, indices, palette []byte, bitmapFormat byte) {
	// First, decode the 4-entry palette to RGBA
	var pal [4][4]byte
	bpp := formatToBpp(bitmapFormat)
	rawBPP := bpp / 8
	if bpp == 15 {
		rawBPP = 2
	}

	for i := range 4 {
		off := i * rawBPP
		if off+rawBPP > len(palette) {
			break
		}
		switch bitmapFormat {
		case bmfBpp32:
			// Wire format is BGRX
			pal[i] = [4]byte{palette[off+2], palette[off+1], palette[off], 0xFF}
		case bmfBpp24:
			// Wire format is BGR
			pal[i] = [4]byte{palette[off+2], palette[off+1], palette[off], 0xFF}
		case bmfBpp16:
			v := uint16(palette[off]) | uint16(palette[off+1])<<8
			r := uint8((v >> 11) & 0x1F)
			g := uint8((v >> 5) & 0x3F)
			b := uint8(v & 0x1F)
			pal[i] = [4]byte{(r << 3) | (r >> 2), (g << 2) | (g >> 4), (b << 3) | (b >> 2), 0xFF}
		case bmfBpp8:
			v := palette[off]
			pal[i] = [4]byte{v, v, v, 0xFF}
		}
	}

	// Decode 2-bit indices: each byte holds 4 pixels, MSB first
	// Byte 0 bits 7-6 = pixel 0, bits 5-4 = pixel 1, etc.
	di := 0
	for byteIdx := 0; byteIdx < 16 && byteIdx < len(indices); byteIdx++ {
		b := indices[byteIdx]
		for shift := 6; shift >= 0; shift -= 2 {
			idx := (b >> uint(shift)) & 0x03
			c := pal[idx]
			brush.Data[di] = c[0]
			brush.Data[di+1] = c[1]
			brush.Data[di+2] = c[2]
			brush.Data[di+3] = c[3]
			di += 4
		}
	}
}
