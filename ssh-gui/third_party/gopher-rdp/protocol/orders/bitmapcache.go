package orders

import (
	"encoding/binary"
	"github.com/bouncyball-git/gopher-rdp/protocol/rle"
)

// CachedBitmap stores a single bitmap cache entry, normalized to 32bpp BGRX.
type CachedBitmap struct {
	Width, Height int
	Data          []byte // bottom-up BGRX 32bpp
}

// NumBitmapCaches is the number of bitmap cache cells (MS-RDPBCGR Rev2 supports up to 5).
const NumBitmapCaches = 5

// WaitingListIndex is the special cache index used by the server when
// CBR2_DO_NOT_CACHE is set. The bitmap is parked here and only promoted
// to a real slot on second reference. Maps to the n+1 entry per cell.
const WaitingListIndex = 32767

// BitmapCache holds up to 5 bitmap caches (matching TS_BITMAPCACHE_CAPABILITYSET_REV2).
// Each cell has n+1 entries: n regular + 1 waiting list slot.
type BitmapCache struct {
	entries [NumBitmapCaches][]CachedBitmap
	valid   [NumBitmapCaches][]bool
	sizes   [NumBitmapCaches]int // regular entry count per cell (excludes waiting list slot)
}

// Init allocates the cache arrays with the given sizes per cache.
// Each cell gets size+1 entries to accommodate the waiting list slot.
func (bc *BitmapCache) Init(sizes [NumBitmapCaches]int) {
	bc.sizes = sizes
	for i, sz := range sizes {
		bc.entries[i] = make([]CachedBitmap, sz+1) // +1 for waiting list
		bc.valid[i] = make([]bool, sz+1)
	}
}

// Set stores a bitmap in the cache, reusing the backing slice when possible.
func (bc *BitmapCache) Set(cacheID, index, width, height int, data []byte) {
	if cacheID < 0 || cacheID >= NumBitmapCaches {
		return
	}
	// Map waiting list index to the extra slot at the end
	if index == WaitingListIndex {
		index = bc.sizes[cacheID]
	}
	if index < 0 || index >= len(bc.entries[cacheID]) {
		return
	}
	dst := &bc.entries[cacheID][index]
	dst.Width = width
	dst.Height = height
	need := width * height * 4
	if cap(dst.Data) >= need {
		dst.Data = dst.Data[:need]
	} else {
		dst.Data = make([]byte, need)
	}
	copy(dst.Data, data[:need])
	bc.valid[cacheID][index] = true
}

// Get returns a pointer to a cached bitmap, or nil if not valid.
func (bc *BitmapCache) Get(cacheID, index int) *CachedBitmap {
	if cacheID < 0 || cacheID >= NumBitmapCaches {
		return nil
	}
	// Map waiting list index to the extra slot at the end
	if index == WaitingListIndex {
		index = bc.sizes[cacheID]
	}
	if index < 0 || index >= len(bc.entries[cacheID]) {
		return nil
	}
	if !bc.valid[cacheID][index] {
		return nil
	}
	return &bc.entries[cacheID][index]
}

// Reset clears all valid flags (entries are lazily overwritten).
func (bc *BitmapCache) Reset() {
	for i := range bc.valid {
		clear(bc.valid[i])
	}
}

// readVar2 reads a 2-byte variable-length unsigned value (MS-RDPEGDI 2.2.2.2.1.2.1.2).
// If bit 7 of the first byte is set: value = ((first & 0x7F) << 8) | second.
// Otherwise: value = first byte alone.
func readVar2(data []byte, off int) (uint16, int) {
	if off >= len(data) {
		return 0, -1
	}
	b := data[off]
	if b&0x80 != 0 {
		if off+2 > len(data) {
			return 0, -1
		}
		return uint16(b&0x7F)<<8 | uint16(data[off+1]), off + 2
	}
	return uint16(b), off + 1
}

// readVar4 reads a 4-byte variable-length unsigned value (MS-RDPEGDI 2.2.2.2.1.2.1.3).
// Top 2 bits of first byte encode additional byte count (0-3).
// Value = (first & 0x3F) then big-endian shift-OR additional bytes.
func readVar4(data []byte, off int) (uint32, int) {
	if off >= len(data) {
		return 0, -1
	}
	b := data[off]
	extra := int((b & 0xC0) >> 6)
	v := uint32(b & 0x3F)
	off++
	if off+extra > len(data) {
		return 0, -1
	}
	for i := 0; i < extra; i++ {
		v = (v << 8) | uint32(data[off])
		off++
	}
	return v, off
}

// bppFromID maps a bitsPerPixelId to actual BPP.
// Index 0-2 are unused/invalid, 3=8, 4=16, 5=24, 6=32.
var bppFromID = [7]int{0, 0, 0, 8, 16, 24, 32}

// DecodeCacheBitmapV1 parses a Cache Bitmap Rev1 secondary order (MS-RDPEGDI 2.2.2.2.1.2.2).
// Wire format: cacheID(1) + pad(1) + width(1) + height(1) + bpp(1) + bufsize(2) + cacheIdx(2) + data(bufsize).
// compressed=false for type 0 (raw), compressed=true for type 2 (compressed).
// The bitmap is normalized to 32bpp RGBA and stored in the cache.
func DecodeCacheBitmapV1(
	cache *BitmapCache,
	secData []byte, compressed bool,
	decompBuf []byte,
	pal *[256][3]byte,
) []byte {
	if len(secData) < 9 {
		return decompBuf
	}
	cacheID := int(secData[0])
	// secData[1] = pad
	width := int(secData[2])
	height := int(secData[3])
	bpp := int(secData[4])
	bufsize := int(binary.LittleEndian.Uint16(secData[5:7]))
	cacheIdx := int(binary.LittleEndian.Uint16(secData[7:9]))

	if width <= 0 || height <= 0 || bpp == 0 {
		return decompBuf
	}

	off := 9
	if off+bufsize > len(secData) {
		bufsize = len(secData) - off
	}
	bitmapData := secData[off : off+bufsize]

	bytesPP := (bpp + 7) / 8
	var pixels []byte
	if compressed {
		var err error
		if bpp == 32 {
			decompBuf, err = rle.DecompressPlanar(decompBuf, width, height, bitmapData)
		} else {
			decompBuf, err = rle.DecompressInto(decompBuf, width, height, bpp, bitmapData)
		}
		if err != nil {
			return decompBuf
		}
		pixels = decompBuf
	} else {
		// Uncompressed: flip rows to bottom-up order (MS-RDPEGDI 2.2.2.2.1.3.2)
		rowSize := width * bytesPP
		rawLen := height * rowSize
		if len(bitmapData) < rawLen {
			return decompBuf
		}
		if cap(decompBuf) >= rawLen {
			decompBuf = decompBuf[:rawLen]
		} else {
			decompBuf = make([]byte, rawLen)
		}
		for y := 0; y < height; y++ {
			srcOff := y * rowSize
			dstOff := (height - y - 1) * rowSize
			copy(decompBuf[dstOff:dstOff+rowSize], bitmapData[srcOff:srcOff+rowSize])
		}
		pixels = decompBuf
	}

	// Normalize to 32bpp RGBA
	need := width * height * 4
	if bpp == 32 && compressed {
		cache.Set(cacheID, cacheIdx, width, height, pixels)
	} else if bpp == 32 {
		var dst []byte
		if cap(decompBuf) >= need && len(pixels) > 0 && &pixels[0] != &decompBuf[0] {
			dst = decompBuf[:need]
		} else {
			dst = make([]byte, need)
		}
		for i := 0; i < need && i+3 < len(pixels); i += 4 {
			dst[i] = pixels[i+2]
			dst[i+1] = pixels[i+1]
			dst[i+2] = pixels[i]
			dst[i+3] = 0xFF
		}
		cache.Set(cacheID, cacheIdx, width, height, dst)
	} else {
		var dst []byte
		if cap(decompBuf) >= need && len(pixels) > 0 && &pixels[0] != &decompBuf[0] {
			dst = decompBuf[:need]
		} else {
			dst = make([]byte, need)
		}
		normalizeToRGBA32(dst, pixels, width, height, bpp, pal)
		cache.Set(cacheID, cacheIdx, width, height, dst)
	}

	return decompBuf
}

// Bitmap cache V2 extraFlags field layout (MS-RDPEGDI 2.2.2.2.1.3.3).
const (
	bmpCache2IDMask     = 0x0007 // bits 0-2: cache ID
	bmpCache2ModeMask   = 0x0038 // bits 3-5: BPP mode (bppBytes = mode - 2)
	bmpCache2ModeShift  = 3
	bmpCache2Square     = 0x0080 // bit 7: height == width
	bmpCache2Persist    = 0x0100 // bit 8: persistent key present (8 bytes)
	bmpCache2LongFormat = 0x80   // bit 7 of cacheIndex byte: 2-byte index
	bmpCache2BufsizeMask = 0x3FFF
)

// DecodeCacheBitmapV2 parses a Cache Bitmap V2 secondary order
// and stores the result in the bitmap cache. The bitmap is normalized to 32bpp RGBA.
//
// Wire format (MS-RDPEGDI 2.2.2.2.1.3.3):
//
//	extraFlags: cacheID(3) | bppMode(3) | ?(1) | square(1) | persist(1) | ...
//	payload: [persistKey(8)] width(u8) [height(u8)] bufsize(u16BE) cacheIdx(1-2) data
//
// compressed indicates whether bitmapData is compressed.
// decompBuf is a reusable buffer (may be nil).
// Returns the (possibly reallocated) decompBuf.
func DecodeCacheBitmapV2(
	cache *BitmapCache,
	extraFlags uint16, secData []byte, compressed bool,
	decompBuf []byte,
	pal *[256][3]byte,
) []byte {
	cacheID := int(extraFlags & bmpCache2IDMask)
	bppBytes := int(((extraFlags & bmpCache2ModeMask) >> bmpCache2ModeShift) - 2)
	if bppBytes <= 0 || bppBytes > 4 {
		return decompBuf
	}
	bpp := bppBytes * 8

	off := 0

	// Optional persistent key (8 bytes)
	if extraFlags&bmpCache2Persist != 0 {
		off += 8
	}
	if off >= len(secData) {
		return decompBuf
	}

	// Width and height: single bytes
	width := int(secData[off])
	off++
	var height int
	if extraFlags&bmpCache2Square != 0 {
		height = width
	} else {
		if off >= len(secData) {
			return decompBuf
		}
		height = int(secData[off])
		off++
	}

	if width <= 0 || height <= 0 {
		return decompBuf
	}

	// Buffer size: 16-bit big-endian, masked to 14 bits
	if off+2 > len(secData) {
		return decompBuf
	}
	bufsize := int(uint16(secData[off])<<8 | uint16(secData[off+1]))
	bufsize &= bmpCache2BufsizeMask
	off += 2

	// Cache index: 1-2 bytes (high bit of first byte = long format)
	if off >= len(secData) {
		return decompBuf
	}
	cacheIdxRaw := secData[off]
	off++
	var cacheIdx int
	if cacheIdxRaw&bmpCache2LongFormat != 0 {
		if off >= len(secData) {
			return decompBuf
		}
		cacheIdx = int(cacheIdxRaw^bmpCache2LongFormat)<<8 | int(secData[off])
		off++
	} else {
		cacheIdx = int(cacheIdxRaw)
	}

	// Bitmap data
	if off+bufsize > len(secData) {
		bufsize = len(secData) - off
	}
	bitmapData := secData[off : off+bufsize]

	w := width
	h := height

	// Decompress if compressed
	var pixels []byte
	if compressed {
		var err error
		if bpp == 32 {
			decompBuf, err = rle.DecompressPlanar(decompBuf, w, h, bitmapData)
			if err != nil {
				return decompBuf
			}
			pixels = decompBuf
		} else {
			decompBuf, err = rle.DecompressInto(decompBuf, w, h, bpp, bitmapData)
			if err != nil {
				return decompBuf
			}
			pixels = decompBuf
		}
	} else {
		// Uncompressed: flip rows to bottom-up order (MS-RDPEGDI 2.2.2.2.1.3.3)
		rowSize := w * bppBytes
		rawLen := h * rowSize
		if len(bitmapData) < rawLen {
			return decompBuf
		}
		need := rawLen
		if cap(decompBuf) >= need {
			decompBuf = decompBuf[:need]
		} else {
			decompBuf = make([]byte, need)
		}
		for y := 0; y < h; y++ {
			srcOff := y * rowSize
			dstOff := (h - y - 1) * rowSize
			copy(decompBuf[dstOff:dstOff+rowSize], bitmapData[srcOff:srcOff+rowSize])
		}
		pixels = decompBuf
	}

	// Normalize to 32bpp RGBA for uniform cache access
	if bpp == 32 && compressed {
		// Compressed 32bpp → already RGBA from DecompressPlanar.
		cache.Set(cacheID, cacheIdx, w, h, pixels)
	} else if bpp == 32 {
		// Uncompressed 32bpp wire data is BGRX — convert to RGBA.
		need := w * h * 4
		var dst []byte
		if cap(decompBuf) >= need && len(pixels) > 0 && &pixels[0] != &decompBuf[0] {
			dst = decompBuf[:need]
		} else {
			dst = make([]byte, need)
		}
		for i := 0; i < need && i+3 < len(pixels); i += 4 {
			dst[i] = pixels[i+2]   // R
			dst[i+1] = pixels[i+1] // G
			dst[i+2] = pixels[i]   // B
			dst[i+3] = 0xFF        // A
		}
		cache.Set(cacheID, cacheIdx, w, h, dst)
	} else {
		// Convert to RGBA 32bpp
		need := w * h * 4
		// Reuse decompBuf if pixels don't overlap it, otherwise alloc temp
		var dst []byte
		if cap(decompBuf) >= need && len(pixels) > 0 && &pixels[0] != &decompBuf[0] {
			dst = decompBuf[:need]
		} else {
			dst = make([]byte, need)
		}
		normalizeToRGBA32(dst, pixels, w, h, bpp, pal)
		cache.Set(cacheID, cacheIdx, w, h, dst)
	}

	return decompBuf
}

// normalizeToRGBA32 converts lower-BPP pixel data to RGBA 32bpp.
// For 8bpp, pal provides the palette for index→RGB lookup.
func normalizeToRGBA32(dst, src []byte, width, height, bpp int, pal *[256][3]byte) {
	nPixels := width * height
	switch bpp {
	case 24:
		// src is BGR 3 bytes per pixel → dst is RGBA 4 bytes per pixel
		si := 0
		di := 0
		for i := 0; i < nPixels; i++ {
			if si+3 > len(src) {
				break
			}
			dst[di] = src[si+2]   // R
			dst[di+1] = src[si+1] // G
			dst[di+2] = src[si]   // B
			dst[di+3] = 0xFF
			si += 3
			di += 4
		}
	case 16:
		// RGB565: RRRRRGGG GGGBBBBB (little-endian)
		si := 0
		di := 0
		for i := 0; i < nPixels; i++ {
			if si+2 > len(src) {
				break
			}
			v := uint16(src[si]) | uint16(src[si+1])<<8
			r := uint8((v >> 11) & 0x1F)
			g := uint8((v >> 5) & 0x3F)
			b := uint8(v & 0x1F)
			dst[di] = (r << 3) | (r >> 2)     // R: 5-bit → 8-bit
			dst[di+1] = (g << 2) | (g >> 4)   // G: 6-bit → 8-bit
			dst[di+2] = (b << 3) | (b >> 2)   // B: 5-bit → 8-bit
			dst[di+3] = 0xFF
			si += 2
			di += 4
		}
	case 15:
		// RGB555: 0RRRRRGGGGGBBBBB (little-endian)
		si := 0
		di := 0
		for i := 0; i < nPixels; i++ {
			if si+2 > len(src) {
				break
			}
			v := uint16(src[si]) | uint16(src[si+1])<<8
			r := uint8((v >> 10) & 0x1F)
			g := uint8((v >> 5) & 0x1F)
			b := uint8(v & 0x1F)
			dst[di] = (r << 3) | (r >> 2)
			dst[di+1] = (g << 3) | (g >> 2)
			dst[di+2] = (b << 3) | (b >> 2)
			dst[di+3] = 0xFF
			si += 2
			di += 4
		}
	case 8:
		di := 0
		for i := 0; i < nPixels; i++ {
			if i >= len(src) {
				break
			}
			idx := src[i]
			if pal != nil {
				p := pal[idx]
				dst[di] = p[0]
				dst[di+1] = p[1]
				dst[di+2] = p[2]
			} else {
				dst[di] = idx
				dst[di+1] = idx
				dst[di+2] = idx
			}
			dst[di+3] = 0xFF
			di += 4
		}
	}
}
