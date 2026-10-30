// Planar codec decompression for 32 bpp bitmaps (MS-RDPEGDI 2.2.2.5).
//
// 32 bpp compressed bitmaps use the RDP 6.0 Bitmap Compression format
// instead of Interleaved RLE. Pixel data is split into separate A/R/G/B
// planes, each independently compressed. Each control byte encodes a
// run length (low nibble) and raw byte count (high nibble). The first
// scanline contains absolute values; subsequent scanlines use zigzag-
// encoded deltas from the row above.
package rle

import "errors"

// Planar format header flags (MS-RDPEGDI 2.2.2.5.1).
const (
	planarCLLMask = 0x07 // bits 0-2: compression level (lossy)
	planarRLE     = 0x10 // bit 4: per-plane RLE
	planarNA      = 0x20 // bit 5: no alpha plane
)

var (
	errPlanarShort   = errors.New("planar: data too short")
	errPlanarOverrun = errors.New("planar: scanline overrun")
)

// DecompressPlanar decompresses RDP 6.0 planar-coded bitmap data into
// bottom-up RGBA (4 bytes per pixel) output. R/G/B planes are decoded
// directly into the output buffer at stride-4 intervals, eliminating
// the intermediate planes buffer. dst is a caller-provided reusable
// buffer (may be nil; will be allocated/grown as needed).
func DecompressPlanar(dst []byte, width, height int, src []byte) ([]byte, error) {
	if len(src) < 1 {
		return nil, errPlanarShort
	}

	hdr := src[0]
	cll := int(hdr & planarCLLMask)
	rle := hdr&planarRLE != 0
	noAlpha := hdr&planarNA != 0
	off := 1

	planeSize := width * height
	dstLen := planeSize * 4
	if cap(dst) < dstLen {
		dst = make([]byte, dstLen)
	} else {
		dst = dst[:dstLen]
	}

	// Pre-fill alpha to 0xFF. Desktop bitmaps are always opaque;
	// servers commonly send alpha=0 which would make pixels transparent.
	for i := 3; i < dstLen; i += 4 {
		dst[i] = 0xFF
	}

	if rle {
		// Plane order: Alpha (if present), Red, Green, Blue.
		if !noAlpha {
			// Skip alpha plane — we force 0xFF above.
			consumed, err := skipPlaneRLE(src[off:], width, height)
			if err != nil {
				return nil, err
			}
			off += consumed
		}
		// Decode R, G, B directly into dst at byte offsets 0, 1, 2 with stride 4.
		for _, planeOff := range [3]int{0, 1, 2} {
			consumed, err := decodePlaneRLEStrided(dst[planeOff:], src[off:], width, height, 4)
			if err != nil {
				return nil, err
			}
			off += consumed
		}
	} else {
		// Uncompressed planes.
		if !noAlpha {
			// Skip alpha plane raw bytes.
			if off+planeSize > len(src) {
				return nil, errPlanarShort
			}
			off += planeSize
		}
		// Scatter each plane into dst at stride 4.
		for _, planeOff := range [3]int{0, 1, 2} {
			if off+planeSize > len(src) {
				return nil, errPlanarShort
			}
			plane := src[off : off+planeSize]
			for i, v := range plane {
				dst[i*4+planeOff] = v
			}
			off += planeSize
		}
	}

	// Apply CLL (compression level lossy): left-shift R, G, B channels.
	if cll > 0 {
		for i := 0; i < dstLen; i += 4 {
			dst[i] <<= cll
			dst[i+1] <<= cll
			dst[i+2] <<= cll
		}
	}

	return dst, nil
}

// decodePlaneRLEStrided decodes a single RLE-compressed plane directly into
// an RGBA output buffer. Decoded bytes are written at dst[0], dst[stride],
// dst[2*stride], etc. This fuses plane decoding with interleaving, avoiding
// an intermediate buffer and a separate scatter pass.
// Returns the number of source bytes consumed.
func decodePlaneRLEStrided(dst []byte, src []byte, width, height, stride int) (int, error) {
	srcOff := 0
	dstOff := 0
	rowStride := width * stride

	// First scanline: absolute values (no delta, no prevRow lookup).
	if height > 0 {
		var pixel byte
		for x := 0; x < width; {
			if srcOff >= len(src) {
				return 0, errPlanarShort
			}
			ctrl := src[srcOff]
			srcOff++

			nRun := int(ctrl & 0x0F)
			cRaw := int(ctrl >> 4)
			if nRun == 1 {
				nRun = cRaw + 16
				cRaw = 0
			} else if nRun == 2 {
				nRun = cRaw + 32
				cRaw = 0
			}

			total := nRun + cRaw
			if x+total > width {
				return 0, errPlanarOverrun
			}
			if srcOff+cRaw > len(src) {
				return 0, errPlanarShort
			}

			// BCE: prove dst write range is in-bounds.
			_ = dst[dstOff+(total-1)*stride]

			for range cRaw {
				pixel = src[srcOff]
				srcOff++
				dst[dstOff] = pixel
				dstOff += stride
			}
			for range nRun {
				dst[dstOff] = pixel
				dstOff += stride
			}
			x += total
		}
	}

	// Subsequent scanlines: zigzag-encoded deltas from the row above.
	for y := 1; y < height; y++ {
		var delta byte

		for x := 0; x < width; {
			if srcOff >= len(src) {
				return 0, errPlanarShort
			}
			ctrl := src[srcOff]
			srcOff++

			nRun := int(ctrl & 0x0F)
			cRaw := int(ctrl >> 4)
			if nRun == 1 {
				nRun = cRaw + 16
				cRaw = 0
			} else if nRun == 2 {
				nRun = cRaw + 32
				cRaw = 0
			}

			total := nRun + cRaw
			if x+total > width {
				return 0, errPlanarOverrun
			}
			if srcOff+cRaw > len(src) {
				return 0, errPlanarShort
			}

			// BCE: prove dst and prevRow ranges are in-bounds.
			_ = dst[dstOff+(total-1)*stride]
			_ = dst[dstOff+(total-1)*stride-rowStride]

			for range cRaw {
				dv := src[srcOff]
				srcOff++
				delta = dv >> 1
				if dv&1 != 0 {
					delta = ^delta
				}
				dst[dstOff] = dst[dstOff-rowStride] + delta
				dstOff += stride
			}
			for range nRun {
				dst[dstOff] = dst[dstOff-rowStride] + delta
				dstOff += stride
			}
			x += total
		}
	}

	return srcOff, nil
}

// skipPlaneRLE parses a single RLE-compressed plane without writing output,
// returning the number of source bytes consumed. Used to skip the alpha plane.
func skipPlaneRLE(src []byte, width, height int) (int, error) {
	srcOff := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; {
			if srcOff >= len(src) {
				return 0, errPlanarShort
			}
			ctrl := src[srcOff]
			srcOff++

			nRun := int(ctrl & 0x0F)
			cRaw := int(ctrl >> 4)
			if nRun == 1 {
				nRun = cRaw + 16
				cRaw = 0
			} else if nRun == 2 {
				nRun = cRaw + 32
				cRaw = 0
			}

			total := nRun + cRaw
			if x+total > width {
				return 0, errPlanarOverrun
			}
			if srcOff+cRaw > len(src) {
				return 0, errPlanarShort
			}
			srcOff += cRaw
			x += total
		}
	}
	return srcOff, nil
}

// decodePlaneRLE decodes a single RLE-compressed plane using the RDP 6.0
// planar codec format. Each control byte packs a run length (low nibble)
// and raw byte count (high nibble). The first scanline has absolute values;
// subsequent scanlines use zigzag-encoded deltas from the row above.
// Returns the number of source bytes consumed.
func decodePlaneRLE(dst []byte, src []byte, width, height int) (int, error) {
	return decodePlaneRLEStrided(dst, src, width, height, 1)
}
