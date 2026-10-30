// Package rle implements RDP bitmap decompression (MS-RDPBCGR 3.1.9).
package rle

import (
	"encoding/binary"
	"errors"
)

var errDecompressFailed = errors.New("rle: bitmap decompression failed")

// Decompress decompresses an Interleaved RLE bitmap stream.
// bpp must be 8, 15, 16, or 24. The returned slice is in bottom-up scanline
// order (native RDP order) with length width*height*bytesPerPixel.
func Decompress(width, height, bpp int, src []byte) ([]byte, error) {
	return DecompressInto(nil, width, height, bpp, src)
}

// DecompressInto decompresses an Interleaved RLE bitmap stream into dst.
// If dst is too small (or nil), it is grown. The returned slice may differ
// from the input dst. bpp must be 8, 15, 16, or 24.
func DecompressInto(dst []byte, width, height, bpp int, src []byte) ([]byte, error) {
	bytesPP := bppToBytes(bpp)
	if bytesPP == 0 {
		return nil, errors.New("rle: unsupported bpp (must be 8, 15, 16, or 24)")
	}

	dstLen := width * height * bytesPP
	if cap(dst) < dstLen {
		dst = make([]byte, dstLen)
	} else {
		dst = dst[:dstLen]
		clear(dst)
	}

	ok := safeDecompress(dst, width, height, bytesPP, src)
	if !ok {
		return nil, errDecompressFailed
	}
	return dst, nil
}

// safeDecompress calls the appropriate bitmapDecompress function and
// recovers from panics caused by truncated/malformed input data.
func safeDecompress(dst []byte, width, height, bytesPP int, src []byte) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	switch bytesPP {
	case 1:
		return bitmapDecompress1(dst, width, height, src, len(src))
	case 2:
		return bitmapDecompress2(dst, width, height, src, len(src))
	case 3:
		return bitmapDecompress3(dst, width, height, src, len(src))
	}
	return false
}

func bppToBytes(bpp int) int {
	switch bpp {
	case 8:
		return 1
	case 15, 16:
		return 2
	case 24:
		return 3
	default:
		return 0
	}
}

// rleRepeat processes count pixels at position x, up to width.
func rleRepeat(count *int, x *int, width int, body func(xi int)) {
	for *count > 0 && *x < width {
		body(*x)
		*count--
		*x++
	}
}

// rleMaskUpdate advances the bitmask state for FillOrMix opcodes.
func rleMaskUpdate(input []byte, pos *int, mixmask *byte, mask *byte, fomMask int) {
	*mixmask <<= 1
	if *mixmask == 0 {
		if fomMask != 0 {
			*mask = byte(fomMask)
		} else {
			*mask = input[*pos]
			*pos++
		}
		*mixmask = 1
	}
}

// cval reads one byte from input at pos and advances pos.
func cval(input []byte, pos *int) byte {
	v := input[*pos]
	*pos++
	return v
}

// cval2 reads a uint16 (little-endian) from input at pos and advances pos by 2.
func cval2(input []byte, pos *int) uint16 {
	v := binary.LittleEndian.Uint16(input[*pos:])
	*pos += 2
	return v
}

// decodeOpcode decodes the RLE opcode, count, and offset from the input stream.
func decodeOpcode(input []byte, pos *int) (int, int, int) {
	code := cval(input, pos)
	opcode := int(code >> 4)
	var count, offset int

	switch {
	case opcode >= 0xc && opcode <= 0xe:
		opcode -= 6
		count = int(code & 0xf)
		offset = 16
	case opcode == 0xf:
		opcode = int(code & 0xf)
		if opcode < 9 {
			count = int(cval(input, pos))
			count |= int(cval(input, pos)) << 8
		} else {
			if opcode < 0xb {
				count = 8
			} else {
				count = 1
			}
		}
		offset = 0
	default:
		opcode >>= 1
		count = int(code & 0x1f)
		offset = 32
	}

	if offset != 0 {
		isfillormix := (opcode == 2) || (opcode == 7)
		if count == 0 {
			if isfillormix {
				count = int(cval(input, pos)) + 1
			} else {
				count = int(cval(input, pos)) + offset
			}
		} else if isfillormix {
			count <<= 3
		}
	}

	return opcode, count, offset
}

// bitmapDecompress1 decompresses a 1 byte per pixel bitmap.
// Output is bottom-up: first decoded scanline at offset 0.
func bitmapDecompress1(output []byte, width, height int, input []byte, size int) bool {
	end := size
	pos := 0
	var prevline []byte
	var line []byte
	x := width
	lastopcode := -1
	insertmix := false
	bicolour := false
	var colour1, colour2 byte
	var mixmask, mask byte
	mix := byte(0xff)
	fomMask := 0
	rowNum := 0

	for pos < end {
		fomMask = 0
		opcode, count, _ := decodeOpcode(input, &pos)

		switch opcode {
		case 0: // Fill
			if lastopcode == opcode && !(x == width && prevline == nil) {
				insertmix = true
			}
		case 8: // Bicolour
			colour1 = cval(input, &pos)
			colour2 = cval(input, &pos)
		case 3: // Colour
			colour2 = cval(input, &pos)
		case 6: // SetMix/Mix
			mix = cval(input, &pos)
			opcode = 1
		case 7: // SetMix/FillOrMix
			mix = cval(input, &pos)
			opcode = 2
		case 9: // FillOrMix_1
			mask = 0x03
			opcode = 0x02
			fomMask = 3
		case 0x0a: // FillOrMix_2
			mask = 0x05
			opcode = 0x02
			fomMask = 5
		}
		lastopcode = opcode
		mixmask = 0

		for count > 0 {
			if x >= width {
				if rowNum >= height {
					return false
				}
				x = 0
				prevline = line
				line = output[rowNum*width:]
				rowNum++
			}
			switch opcode {
			case 0: // Fill
				if insertmix {
					if prevline == nil {
						line[x] = mix
					} else {
						line[x] = prevline[x] ^ mix
					}
					insertmix = false
					count--
					x++
				}
				if prevline == nil {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi] = 0
					})
				} else {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi] = prevline[xi]
					})
				}
			case 1: // Mix
				if prevline == nil {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi] = mix
					})
				} else {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi] = prevline[xi] ^ mix
					})
				}
			case 2: // FillOrMix
				if prevline == nil {
					rleRepeat(&count, &x, width, func(xi int) {
						rleMaskUpdate(input, &pos, &mixmask, &mask, fomMask)
						if mask&mixmask != 0 {
							line[xi] = mix
						} else {
							line[xi] = 0
						}
					})
				} else {
					rleRepeat(&count, &x, width, func(xi int) {
						rleMaskUpdate(input, &pos, &mixmask, &mask, fomMask)
						if mask&mixmask != 0 {
							line[xi] = prevline[xi] ^ mix
						} else {
							line[xi] = prevline[xi]
						}
					})
				}
			case 3: // Colour
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi] = colour2
				})
			case 4: // Copy
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi] = cval(input, &pos)
				})
			case 8: // Bicolour
				rleRepeat(&count, &x, width, func(xi int) {
					if bicolour {
						line[xi] = colour2
						bicolour = false
					} else {
						line[xi] = colour1
						bicolour = true
						count++
					}
				})
			case 0xd: // White
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi] = 0xff
				})
			case 0xe: // Black
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi] = 0
				})
			default:
				return false
			}
		}
	}
	return true
}

// bitmapDecompress2 decompresses a 2 byte per pixel bitmap (15/16 bpp).
// Output is bottom-up: first decoded scanline at offset 0.
func bitmapDecompress2(output []byte, width, height int, input []byte, size int) bool {
	end := size
	pos := 0
	var prevline []uint16
	var line []uint16
	out16 := make([]uint16, width*height)
	x := width
	lastopcode := -1
	insertmix := false
	bicolour := false
	var colour1, colour2 uint16
	var mixmask, mask byte
	mix := uint16(0xffff)
	fomMask := 0
	rowNum := 0

	for pos < end {
		fomMask = 0
		opcode, count, _ := decodeOpcode(input, &pos)

		switch opcode {
		case 0:
			if lastopcode == opcode && !(x == width && prevline == nil) {
				insertmix = true
			}
		case 8:
			colour1 = cval2(input, &pos)
			colour2 = cval2(input, &pos)
		case 3:
			colour2 = cval2(input, &pos)
		case 6:
			mix = cval2(input, &pos)
			opcode = 1
		case 7:
			mix = cval2(input, &pos)
			opcode = 2
		case 9:
			mask = 0x03
			opcode = 0x02
			fomMask = 3
		case 0x0a:
			mask = 0x05
			opcode = 0x02
			fomMask = 5
		}
		lastopcode = opcode
		mixmask = 0

		for count > 0 {
			if x >= width {
				if rowNum >= height {
					break
				}
				x = 0
				prevline = line
				line = out16[rowNum*width:]
				rowNum++
			}
			switch opcode {
			case 0: // Fill
				if insertmix {
					if prevline == nil {
						line[x] = mix
					} else {
						line[x] = prevline[x] ^ mix
					}
					insertmix = false
					count--
					x++
				}
				if prevline == nil {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi] = 0
					})
				} else {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi] = prevline[xi]
					})
				}
			case 1: // Mix
				if prevline == nil {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi] = mix
					})
				} else {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi] = prevline[xi] ^ mix
					})
				}
			case 2: // FillOrMix
				if prevline == nil {
					rleRepeat(&count, &x, width, func(xi int) {
						rleMaskUpdate(input, &pos, &mixmask, &mask, fomMask)
						if mask&mixmask != 0 {
							line[xi] = mix
						} else {
							line[xi] = 0
						}
					})
				} else {
					rleRepeat(&count, &x, width, func(xi int) {
						rleMaskUpdate(input, &pos, &mixmask, &mask, fomMask)
						if mask&mixmask != 0 {
							line[xi] = prevline[xi] ^ mix
						} else {
							line[xi] = prevline[xi]
						}
					})
				}
			case 3: // Colour
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi] = colour2
				})
			case 4: // Copy
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi] = cval2(input, &pos)
				})
			case 8: // Bicolour
				rleRepeat(&count, &x, width, func(xi int) {
					if bicolour {
						line[xi] = colour2
						bicolour = false
					} else {
						line[xi] = colour1
						bicolour = true
						count++
					}
				})
			case 0xd: // White
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi] = 0xffff
				})
			case 0xe: // Black
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi] = 0
				})
			default:
				return false
			}
		}
	}

	// Convert uint16 slice to output bytes (little-endian)
	for i, v := range out16 {
		binary.LittleEndian.PutUint16(output[i*2:], v)
	}
	return true
}

// bitmapDecompress3 decompresses a 3 byte per pixel bitmap (24 bpp).
// Output is bottom-up: first decoded scanline at offset 0.
func bitmapDecompress3(output []byte, width, height int, input []byte, size int) bool {
	end := size
	pos := 0
	var prevline []byte
	var line []byte
	x := width
	lastopcode := -1
	insertmix := false
	bicolour := false
	colour1 := [3]byte{}
	colour2 := [3]byte{}
	var mixmask, mask byte
	mix := [3]byte{0xff, 0xff, 0xff}
	fomMask := 0
	rowNum := 0

	for pos < end {
		fomMask = 0
		opcode, count, _ := decodeOpcode(input, &pos)

		switch opcode {
		case 0:
			if lastopcode == opcode && !(x == width && prevline == nil) {
				insertmix = true
			}
		case 8:
			colour1[0] = cval(input, &pos)
			colour1[1] = cval(input, &pos)
			colour1[2] = cval(input, &pos)
			colour2[0] = cval(input, &pos)
			colour2[1] = cval(input, &pos)
			colour2[2] = cval(input, &pos)
		case 3:
			colour2[0] = cval(input, &pos)
			colour2[1] = cval(input, &pos)
			colour2[2] = cval(input, &pos)
		case 6:
			mix[0] = cval(input, &pos)
			mix[1] = cval(input, &pos)
			mix[2] = cval(input, &pos)
			opcode = 1
		case 7:
			mix[0] = cval(input, &pos)
			mix[1] = cval(input, &pos)
			mix[2] = cval(input, &pos)
			opcode = 2
		case 9:
			mask = 0x03
			opcode = 0x02
			fomMask = 3
		case 0x0a:
			mask = 0x05
			opcode = 0x02
			fomMask = 5
		}
		lastopcode = opcode
		mixmask = 0

		for count > 0 {
			if x >= width {
				if rowNum >= height {
					return false
				}
				x = 0
				prevline = line
				line = output[rowNum*width*3:]
				rowNum++
			}
			switch opcode {
			case 0: // Fill
				if insertmix {
					if prevline == nil {
						line[x*3] = mix[0]
						line[x*3+1] = mix[1]
						line[x*3+2] = mix[2]
					} else {
						line[x*3] = prevline[x*3] ^ mix[0]
						line[x*3+1] = prevline[x*3+1] ^ mix[1]
						line[x*3+2] = prevline[x*3+2] ^ mix[2]
					}
					insertmix = false
					count--
					x++
				}
				if prevline == nil {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi*3] = 0
						line[xi*3+1] = 0
						line[xi*3+2] = 0
					})
				} else {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi*3] = prevline[xi*3]
						line[xi*3+1] = prevline[xi*3+1]
						line[xi*3+2] = prevline[xi*3+2]
					})
				}
			case 1: // Mix
				if prevline == nil {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi*3] = mix[0]
						line[xi*3+1] = mix[1]
						line[xi*3+2] = mix[2]
					})
				} else {
					rleRepeat(&count, &x, width, func(xi int) {
						line[xi*3] = prevline[xi*3] ^ mix[0]
						line[xi*3+1] = prevline[xi*3+1] ^ mix[1]
						line[xi*3+2] = prevline[xi*3+2] ^ mix[2]
					})
				}
			case 2: // FillOrMix
				if prevline == nil {
					rleRepeat(&count, &x, width, func(xi int) {
						rleMaskUpdate(input, &pos, &mixmask, &mask, fomMask)
						if mask&mixmask != 0 {
							line[xi*3] = mix[0]
							line[xi*3+1] = mix[1]
							line[xi*3+2] = mix[2]
						} else {
							line[xi*3] = 0
							line[xi*3+1] = 0
							line[xi*3+2] = 0
						}
					})
				} else {
					rleRepeat(&count, &x, width, func(xi int) {
						rleMaskUpdate(input, &pos, &mixmask, &mask, fomMask)
						if mask&mixmask != 0 {
							line[xi*3] = prevline[xi*3] ^ mix[0]
							line[xi*3+1] = prevline[xi*3+1] ^ mix[1]
							line[xi*3+2] = prevline[xi*3+2] ^ mix[2]
						} else {
							line[xi*3] = prevline[xi*3]
							line[xi*3+1] = prevline[xi*3+1]
							line[xi*3+2] = prevline[xi*3+2]
						}
					})
				}
			case 3: // Colour
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi*3] = colour2[0]
					line[xi*3+1] = colour2[1]
					line[xi*3+2] = colour2[2]
				})
			case 4: // Copy
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi*3] = cval(input, &pos)
					line[xi*3+1] = cval(input, &pos)
					line[xi*3+2] = cval(input, &pos)
				})
			case 8: // Bicolour
				rleRepeat(&count, &x, width, func(xi int) {
					if bicolour {
						line[xi*3] = colour2[0]
						line[xi*3+1] = colour2[1]
						line[xi*3+2] = colour2[2]
						bicolour = false
					} else {
						line[xi*3] = colour1[0]
						line[xi*3+1] = colour1[1]
						line[xi*3+2] = colour1[2]
						bicolour = true
						count++
					}
				})
			case 0xd: // White
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi*3] = 0xff
					line[xi*3+1] = 0xff
					line[xi*3+2] = 0xff
				})
			case 0xe: // Black
				rleRepeat(&count, &x, width, func(xi int) {
					line[xi*3] = 0
					line[xi*3+1] = 0
					line[xi*3+2] = 0
				})
			default:
				return false
			}
		}
	}
	return true
}
