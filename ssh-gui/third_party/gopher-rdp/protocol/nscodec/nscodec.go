// Package nscodec implements the NSCodec bitmap codec decoder (MS-RDPNSC).
//
// NSCodec compresses bitmaps as 4 separate color planes (luma, chroma-orange,
// chroma-green, alpha) using per-plane RLE, with YCoCg color space transform
// and optional chroma subsampling.
package nscodec

import (
	"context"
	"encoding/binary"
	"log/slog"
)

const levelTrace = slog.LevelDebug - 4

// CodecGUID is the NSCodec GUID used in TS_BITMAPCODEC capability sets.
var CodecGUID = [16]byte{
	0xB9, 0x1B, 0x8D, 0xCA, 0x0F, 0x00, 0x4F, 0x15,
	0x58, 0x9F, 0xAE, 0x2D, 0x1A, 0x87, 0xE2, 0xD6,
}

const headerSize = 20

// Decompress decodes an NSCodec-compressed bitmap into bottom-up RGBA pixels.
//
// dst and planesBuf are reusable buffers; they are grown as needed and returned
// for reuse by the caller (zero-alloc steady state).
func Decompress(dst, planesBuf []byte, width, height int, src []byte, log *slog.Logger) ([]byte, []byte, error) {
	if len(src) < headerSize {
		return dst, planesBuf, errShortHeader
	}

	lumaLen := int(binary.LittleEndian.Uint32(src[0:4]))
	coLen := int(binary.LittleEndian.Uint32(src[4:8]))
	cgLen := int(binary.LittleEndian.Uint32(src[8:12]))
	alphaLen := int(binary.LittleEndian.Uint32(src[12:16]))
	colorLossLevel := src[16]
	chromaSubLevel := src[17]
	// dynamicColorFidelity := src[18]
	// reserved := src[19]

	payload := src[headerSize:]
	totalPayload := lumaLen + coLen + cgLen + alphaLen
	if log != nil {
		log.Log(context.Background(), levelTrace, "decompress",
			"width", width, "height", height, "srcLen", len(src),
			"luma", lumaLen, "co", coLen, "cg", cgLen, "alpha", alphaLen,
			"colorLoss", colorLossLevel, "chromaSub", chromaSubLevel)
	}
	if totalPayload > len(payload) {
		return dst, planesBuf, errTruncatedPayload
	}

	lumaData := payload[:lumaLen]
	coData := payload[lumaLen : lumaLen+coLen]
	cgData := payload[lumaLen+coLen : lumaLen+coLen+cgLen]
	alphaData := payload[lumaLen+coLen+cgLen : lumaLen+coLen+cgLen+alphaLen]

	lumaSize := width * height
	var chromaSize int
	if chromaSubLevel == 1 {
		chromaSize = ((width + 1) / 2) * ((height + 1) / 2)
	} else {
		chromaSize = lumaSize
	}
	alphaSize := lumaSize

	// Allocate planes buffer: luma + co + cg + alpha contiguous
	planesTotal := lumaSize + chromaSize + chromaSize + alphaSize
	if cap(planesBuf) >= planesTotal {
		planesBuf = planesBuf[:planesTotal]
	} else {
		planesBuf = make([]byte, planesTotal)
	}

	lumaPlane := planesBuf[:lumaSize]
	coPlane := planesBuf[lumaSize : lumaSize+chromaSize]
	cgPlane := planesBuf[lumaSize+chromaSize : lumaSize+chromaSize+chromaSize]
	alphaPlane := planesBuf[lumaSize+chromaSize+chromaSize:]

	if err := decodePlaneRLE(lumaPlane, lumaData, lumaSize); err != nil {
		return dst, planesBuf, err
	}
	if err := decodePlaneRLE(coPlane, coData, chromaSize); err != nil {
		return dst, planesBuf, err
	}
	if err := decodePlaneRLE(cgPlane, cgData, chromaSize); err != nil {
		return dst, planesBuf, err
	}
	if alphaLen == 0 {
		// No alpha plane sent → fully opaque (0xFF), not transparent.
		for i := range alphaPlane {
			alphaPlane[i] = 0xFF
		}
	} else if err := decodePlaneRLE(alphaPlane, alphaData, alphaSize); err != nil {
		return dst, planesBuf, err
	}

	// Output BGRX
	dstSize := lumaSize * 4
	if cap(dst) >= dstSize {
		dst = dst[:dstSize]
	} else {
		dst = make([]byte, dstSize)
	}

	if chromaSubLevel == 1 {
		convertSubsampled(dst, lumaPlane, coPlane, cgPlane, alphaPlane, width, height, colorLossLevel)
	} else {
		convertDirect(dst, lumaPlane, coPlane, cgPlane, alphaPlane, lumaSize, colorLossLevel)
	}

	return dst, planesBuf, nil
}

// decodePlaneRLE decodes a single NSCodec RLE-encoded plane.
func decodePlaneRLE(dst []byte, src []byte, dstSize int) error {
	if len(src) == 0 {
		// Empty plane: zero-fill output
		for i := range dst[:dstSize] {
			dst[i] = 0
		}
		return nil
	}
	if len(src) == dstSize {
		copy(dst, src)
		return nil
	}
	if len(src) < 4 {
		// Less than 4 bytes but not empty — treat as raw data + zero fill
		copy(dst, src)
		for i := len(src); i < dstSize; i++ {
			dst[i] = 0
		}
		return nil
	}

	// Last 4 bytes are EndData — placed at end of output.
	endData := src[len(src)-4:]
	rleData := src[:len(src)-4]

	di := 0
	si := 0
	endPos := dstSize - 4 // where EndData goes

	for di < endPos && si < len(rleData) {
		val := rleData[si]
		si++

		if si < len(rleData) && val == rleData[si] {
			// RUN
			si++ // consume duplicate
			if si >= len(rleData) {
				return errTruncatedRLE
			}
			factor := rleData[si]
			si++
			var count int
			if factor < 0xFF {
				count = int(factor) + 2
			} else {
				// Long run: 4-byte LE count follows
				if si+4 > len(rleData) {
					return errTruncatedRLE
				}
				count = int(binary.LittleEndian.Uint32(rleData[si : si+4]))
				si += 4
			}

			end := di + count
			if end > endPos {
				end = endPos
			}
			// Use copy-doubling for memset: write first byte, then
			// repeatedly double via copy()'s assembly memcpy.
			if n := end - di; n > 0 {
				dst[di] = val
				for filled := 1; filled < n; filled *= 2 {
					copy(dst[di+filled:end], dst[di:di+filled])
				}
			}
			di = end
		} else {
			// LITERAL
			dst[di] = val
			di++
		}
	}

	// Zero-fill gap (shouldn't happen in well-formed data)
	for di < endPos {
		dst[di] = 0
		di++
	}

	// Place EndData
	if endPos >= 0 && endPos+4 <= dstSize {
		_ = dst[endPos+3] // BCE
		dst[endPos] = endData[0]
		dst[endPos+1] = endData[1]
		dst[endPos+2] = endData[2]
		dst[endPos+3] = endData[3]
	}

	return nil
}

// convertDirect converts YCoCg planes to RGBA without chroma subsampling.
func convertDirect(dst, luma, co, cg, alpha []byte, pixelCount int, colorLossLevel byte) {
	shift := colorLossLevel - 1
	// BCE: prove all plane indices and dst[i*4+3] are in-bounds.
	_ = luma[pixelCount-1]
	_ = co[pixelCount-1]
	_ = cg[pixelCount-1]
	_ = alpha[pixelCount-1]
	_ = dst[pixelCount*4-1]
	for i := 0; i < pixelCount; i++ {
		y := int16(luma[i])
		coVal := int16(int8(co[i] << shift))
		cgVal := int16(int8(cg[i] << shift))

		r := y + coVal - cgVal
		g := y + cgVal
		b := y - coVal - cgVal

		d := dst[i*4 : i*4+4 : i*4+4] // 4-element window BCE
		d[0] = clampByte(r)
		d[1] = clampByte(g)
		d[2] = clampByte(b)
		d[3] = alpha[i]
	}
}

// convertSubsampled converts YCoCg planes to RGBA with 2x2 chroma subsampling.
func convertSubsampled(dst, luma, co, cg, alpha []byte, width, height int, colorLossLevel byte) {
	shift := colorLossLevel - 1
	chromaW := (width + 1) / 2
	// BCE: prove all plane and dst accesses are in-bounds.
	_ = luma[width*height-1]
	_ = alpha[width*height-1]
	_ = co[((height-1)/2)*chromaW+(width-1)/2]
	_ = cg[((height-1)/2)*chromaW+(width-1)/2]
	_ = dst[width*height*4-1]
	di := 0
	lumaOff := 0
	for row := 0; row < height; row++ {
		chromaBase := (row / 2) * chromaW
		for col := 0; col < width; col++ {
			chromaIdx := chromaBase + col/2
			y := int16(luma[lumaOff])
			coVal := int16(int8(co[chromaIdx] << shift))
			cgVal := int16(int8(cg[chromaIdx] << shift))

			d := dst[di : di+4 : di+4]
			d[0] = clampByte(y + coVal - cgVal)
			d[1] = clampByte(y + cgVal)
			d[2] = clampByte(y - coVal - cgVal)
			d[3] = alpha[lumaOff]
			di += 4
			lumaOff++
		}
	}
}

// clampByte clamps an int16 to [0, 255].
func clampByte(v int16) byte {
	if uint16(v)&0xFF00 == 0 {
		return byte(v)
	}
	// v < 0 or v > 255
	// If negative, high bit is set → shift gives 0xFF → ^0xFF = 0
	// If positive overflow, high bit clear → shift gives 0 → ^0 = 0xFF
	return byte(^(v >> 15))
}

// Error sentinels
var (
	errShortHeader      = nscodecError("nscodec: header too short")
	errTruncatedPayload = nscodecError("nscodec: payload truncated")
	errTruncatedRLE     = nscodecError("nscodec: truncated RLE data")
)

type nscodecError string

func (e nscodecError) Error() string { return string(e) }
