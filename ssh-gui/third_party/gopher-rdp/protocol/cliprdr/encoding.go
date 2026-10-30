package cliprdr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"strings"
	"unicode/utf16"
)

// encodeUTF16LE encodes a Go string as UTF-16LE bytes (without null terminator).
func encodeUTF16LE(s string) []byte {
	if s == "" {
		return nil
	}
	u16 := utf16.Encode([]rune(s))
	buf := make([]byte, len(u16)*2)
	for i, c := range u16 {
		binary.LittleEndian.PutUint16(buf[i*2:], c)
	}
	return buf
}

// decodeUTF16LE decodes UTF-16LE bytes into a Go string, stripping trailing nulls.
func decodeUTF16LE(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	// Ensure even length
	n := len(data) &^ 1
	u16 := make([]uint16, n/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	// Strip trailing null characters
	for len(u16) > 0 && u16[len(u16)-1] == 0 {
		u16 = u16[:len(u16)-1]
	}
	return string(utf16.Decode(u16))
}

// textToWire converts a Go string to RDP clipboard wire format:
// LF → CR-LF normalization, UTF-16LE encoding, null terminator appended.
func textToWire(text string) []byte {
	// Normalize: first collapse any existing CR-LF to LF, then LF to CR-LF
	s := strings.ReplaceAll(text, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", "\r\n")
	u16 := utf16.Encode([]rune(s))
	// +1 for null terminator
	buf := make([]byte, (len(u16)+1)*2)
	for i, c := range u16 {
		binary.LittleEndian.PutUint16(buf[i*2:], c)
	}
	// Last 2 bytes already zero (null terminator)
	return buf
}

// textFromWire converts RDP clipboard wire format to a Go string:
// UTF-16LE decode, strip null terminator, CR-LF → LF normalization.
func textFromWire(data []byte) string {
	s := decodeUTF16LE(data)
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// BITMAPINFOHEADER constants (MS-RDPECLIP, referencing MS-WMF 2.2.2.3).
const (
	bihSize   = 40 // sizeof(BITMAPINFOHEADER)
	biRGB     = 0  // BI_RGB (uncompressed)
	biBitfields = 3 // BI_BITFIELDS
)

// dibToPNG converts a CF_DIB payload (BITMAPINFOHEADER + pixel data) to PNG.
// Supports BI_RGB with 24-bit (BGR) and 32-bit (BGRA), and BI_BITFIELDS
// with standard 32-bit BGRA masks.
func dibToPNG(dib []byte) ([]byte, error) {
	if len(dib) < bihSize {
		return nil, fmt.Errorf("cliprdr: DIB too short (%d bytes)", len(dib))
	}

	biWidth := int(int32(binary.LittleEndian.Uint32(dib[4:8])))
	biHeight := int(int32(binary.LittleEndian.Uint32(dib[8:12])))
	biBitCount := binary.LittleEndian.Uint16(dib[14:16])
	biCompression := binary.LittleEndian.Uint32(dib[16:20])

	if biWidth <= 0 {
		return nil, fmt.Errorf("cliprdr: invalid DIB width %d", biWidth)
	}

	// Negative height = top-down; positive = bottom-up.
	topDown := biHeight < 0
	if topDown {
		biHeight = -biHeight
	}
	if biHeight <= 0 {
		return nil, fmt.Errorf("cliprdr: invalid DIB height %d", biHeight)
	}

	// Determine bytes per pixel and validate compression.
	var bpp int
	switch {
	case biCompression == biRGB && biBitCount == 24:
		bpp = 3
	case biCompression == biRGB && biBitCount == 32:
		bpp = 4
	case biCompression == biBitfields && biBitCount == 32:
		bpp = 4
	default:
		return nil, fmt.Errorf("cliprdr: unsupported DIB format (compression=%d, bitCount=%d)", biCompression, biBitCount)
	}

	// Skip color masks for BI_BITFIELDS (3 × uint32 after header).
	pixelOffset := bihSize
	if biCompression == biBitfields {
		pixelOffset += 12
	}

	// Each row is padded to a 4-byte boundary.
	rowBytes := (biWidth*bpp + 3) &^ 3
	needed := pixelOffset + rowBytes*biHeight
	if len(dib) < needed {
		return nil, fmt.Errorf("cliprdr: DIB data truncated (have %d, need %d)", len(dib), needed)
	}

	img := image.NewNRGBA(image.Rect(0, 0, biWidth, biHeight))
	for y := 0; y < biHeight; y++ {
		// Source row index: bottom-up rows start from the bottom.
		srcY := y
		if !topDown {
			srcY = biHeight - 1 - y
		}
		srcRow := dib[pixelOffset+srcY*rowBytes:]
		dstOff := y * img.Stride
		for x := 0; x < biWidth; x++ {
			off := x * bpp
			b := srcRow[off]
			g := srcRow[off+1]
			r := srcRow[off+2]
			a := uint8(255)
			if bpp == 4 {
				a = srcRow[off+3]
			}
			img.Pix[dstOff] = r
			img.Pix[dstOff+1] = g
			img.Pix[dstOff+2] = b
			img.Pix[dstOff+3] = a
			dstOff += 4
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("cliprdr: PNG encode failed: %w", err)
	}
	return buf.Bytes(), nil
}

// pngToDIB converts a PNG image to a CF_DIB payload (BITMAPINFOHEADER +
// 32-bit BGRA pixel data, bottom-up rows).
func pngToDIB(pngData []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("cliprdr: PNG decode failed: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("cliprdr: invalid image dimensions %dx%d", w, h)
	}

	bpp := 4
	rowBytes := w * bpp // 32-bit already 4-byte aligned
	pixelSize := rowBytes * h
	dib := make([]byte, bihSize+pixelSize)

	// BITMAPINFOHEADER
	binary.LittleEndian.PutUint32(dib[0:4], bihSize)         // biSize
	binary.LittleEndian.PutUint32(dib[4:8], uint32(w))       // biWidth
	binary.LittleEndian.PutUint32(dib[8:12], uint32(h))      // biHeight (positive = bottom-up)
	binary.LittleEndian.PutUint16(dib[12:14], 1)             // biPlanes
	binary.LittleEndian.PutUint16(dib[14:16], 32)            // biBitCount
	binary.LittleEndian.PutUint32(dib[16:20], biRGB)         // biCompression
	binary.LittleEndian.PutUint32(dib[20:24], uint32(pixelSize)) // biSizeImage

	// Write pixels bottom-up as BGRA.
	for y := 0; y < h; y++ {
		dstRow := dib[bihSize+(h-1-y)*rowBytes:]
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			off := x * bpp
			dstRow[off] = uint8(b >> 8)
			dstRow[off+1] = uint8(g >> 8)
			dstRow[off+2] = uint8(r >> 8)
			dstRow[off+3] = uint8(a >> 8)
		}
	}

	return dib, nil
}

