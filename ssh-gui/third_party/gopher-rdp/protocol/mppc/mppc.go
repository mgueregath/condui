// Package mppc implements MPPC bulk data decompression (MS-RDPBCGR 3.1.8.4.1/3.1.8.4.2).
//
// MPPC uses a sliding-window dictionary (8 KB for RDP4, 64 KB for RDP5) with
// Huffman-coded literals and LZ77 offset/length pairs. The server compresses
// slow-path Share Data PDU payloads and fast-path update data when the client
// advertises compression support in its Client Info PDU.
package mppc

import "errors"

// Compression type flags (MS-RDPBCGR 2.2.9.1.1.3.1.2.1).
const (
	TypeRDP4 = 0x00 // 8 KB dictionary (RDP 4.0)
	TypeRDP5 = 0x01 // 64 KB dictionary (RDP 5.0)
	TypeRDP6 = 0x02 // not supported
	TypeRDP61 = 0x03 // not supported
)

// Compression flags within the compressionFlags / compressedType byte.
const (
	FlagBig        = 0x01 // 64 KB dictionary (RDP5)
	FlagCompressed = 0x20 // payload is compressed
	FlagReset      = 0x40 // reset history offset
	FlagFlush      = 0x80 // flush (zero) entire history
)

const dictSize = 65536 // 64 KB history buffer

var errDecompress = errors.New("mppc: decompression error")

// Decompressor maintains the MPPC decompression state (history buffer).
type Decompressor struct {
	hist [dictSize]byte
	off  uint32 // current write position in history
}

// Decompress expands MPPC-compressed data. ctype is the compressedType byte
// containing both the compression type (bits 0-3) and flags (bits 4-7).
// Returns the decompressed data or an error.
func (d *Decompressor) Decompress(data []byte, ctype byte) ([]byte, error) {
	if ctype&FlagCompressed == 0 {
		// Not compressed — return data as-is but still handle reset/flush.
		if ctype&FlagReset != 0 {
			d.off = 0
		}
		if ctype&FlagFlush != 0 {
			clear(d.hist[:])
			d.off = 0
		}
		return data, nil
	}

	big := ctype&FlagBig != 0

	if ctype&FlagReset != 0 {
		d.off = 0
	}
	if ctype&FlagFlush != 0 {
		clear(d.hist[:])
		d.off = 0
	}

	nextOffset := int32(d.off)
	oldOffset := nextOffset

	clen := uint32(len(data))
	if clen == 0 {
		d.off = uint32(nextOffset)
		return nil, nil
	}

	var (
		walkerLen int32
		walker    int32
		i         uint32
	)

	for {
		if walkerLen == 0 {
			if i >= clen {
				break
			}
			walker = int32(data[i]) << 24
			i++
			walkerLen = 8
		}
		if walker >= 0 {
			// Literal byte (high bit 0): value is next 8 bits with high bit = 0.
			if walkerLen < 8 {
				if i >= clen {
					if walker != 0 {
						return nil, errDecompress
					}
					break
				}
				walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
				i++
				walkerLen += 8
			}
			if nextOffset >= dictSize {
				return nil, errDecompress
			}
			d.hist[nextOffset] = byte(uint32(walker) >> 24)
			nextOffset++
			walker <<= 8
			walkerLen -= 8
			continue
		}
		walker <<= 1
		walkerLen--
		if walkerLen == 0 {
			if i >= clen {
				return nil, errDecompress
			}
			walker = int32(data[i]) << 24
			i++
			walkerLen = 8
		}
		// Literal byte (high bit 1): value is next 8 bits OR'd with 0x80.
		if walker >= 0 {
			if walkerLen < 8 {
				if i >= clen {
					return nil, errDecompress
				}
				walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
				i++
				walkerLen += 8
			}
			if nextOffset >= dictSize {
				return nil, errDecompress
			}
			d.hist[nextOffset] = byte(walker>>24) | 0x80
			nextOffset++
			walker <<= 8
			walkerLen -= 8
			continue
		}

		// Decode offset/length pair.
		walker <<= 1
		walkerLen--
		minBits := int32(2)
		if big {
			minBits = 3
		}
		if walkerLen < minBits {
			if i >= clen {
				return nil, errDecompress
			}
			walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
			i++
			walkerLen += 8
		}

		var matchOff int32
		if big {
			switch uint32(walker) >> 29 {
			case 7: // 0-63
				for walkerLen < 9 {
					if i >= clen {
						return nil, errDecompress
					}
					walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
					i++
					walkerLen += 8
				}
				walker <<= 3
				matchOff = int32(uint32(walker) >> 26)
				walker <<= 6
				walkerLen -= 9

			case 6: // 64-319
				for walkerLen < 11 {
					if i >= clen {
						return nil, errDecompress
					}
					walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
					i++
					walkerLen += 8
				}
				walker <<= 3
				matchOff = int32(uint32(walker)>>24) + 64
				walker <<= 8
				walkerLen -= 11

			case 5, 4: // 320-2367
				for walkerLen < 13 {
					if i >= clen {
						return nil, errDecompress
					}
					walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
					i++
					walkerLen += 8
				}
				walker <<= 2
				matchOff = int32(uint32(walker)>>21) + 320
				walker <<= 11
				walkerLen -= 13

			default: // 2368-65535
				for walkerLen < 17 {
					if i >= clen {
						return nil, errDecompress
					}
					walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
					i++
					walkerLen += 8
				}
				walker <<= 1
				matchOff = int32(uint32(walker)>>16) + 2368
				walker <<= 16
				walkerLen -= 17
			}
		} else {
			switch uint32(walker) >> 30 {
			case 3: // 0-63
				if walkerLen < 8 {
					if i >= clen {
						return nil, errDecompress
					}
					walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
					i++
					walkerLen += 8
				}
				walker <<= 2
				matchOff = int32(uint32(walker) >> 26)
				walker <<= 6
				walkerLen -= 8

			case 2: // 64-319
				for walkerLen < 10 {
					if i >= clen {
						return nil, errDecompress
					}
					walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
					i++
					walkerLen += 8
				}
				walker <<= 2
				matchOff = int32(uint32(walker)>>24) + 64
				walker <<= 8
				walkerLen -= 10

			default: // 320-8191
				for walkerLen < 14 {
					if i >= clen {
						return nil, errDecompress
					}
					walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
					i++
					walkerLen += 8
				}
				matchOff = int32(walker>>18) + 320
				walker <<= 14
				walkerLen -= 14
			}
		}

		if walkerLen == 0 {
			if i >= clen {
				return nil, errDecompress
			}
			walker = int32(data[i]) << 24
			i++
			walkerLen = 8
		}

		// Decode length of match.
		var matchLen int32
		if walker >= 0 {
			// Special case — length of 3 is in bit 0.
			matchLen = 3
			walker <<= 1
			walkerLen--
		} else {
			matchBits := int32(11)
			if big {
				matchBits = 14
			}
			for {
				walker <<= 1
				walkerLen--
				if walkerLen == 0 {
					if i >= clen {
						return nil, errDecompress
					}
					walker = int32(data[i]) << 24
					i++
					walkerLen = 8
				}
				if walker >= 0 {
					break
				}
				matchBits--
				if matchBits == 0 {
					return nil, errDecompress
				}
			}
			if big {
				matchLen = 16 - matchBits
			} else {
				matchLen = 13 - matchBits
			}
			walker <<= 1
			walkerLen--
			for walkerLen < matchLen {
				if i >= clen {
					return nil, errDecompress
				}
				walker |= int32(data[i]&0xff) << (24 - uint(walkerLen))
				i++
				walkerLen += 8
			}

			bits := matchLen
			matchLen = walker >> (32 - uint(bits))
			matchLen &= (1 << uint(bits)) - 1
			matchLen |= 1 << uint(bits)
			walker <<= uint(bits)
			walkerLen -= bits
		}

		if nextOffset+matchLen >= dictSize {
			return nil, errDecompress
		}

		// Copy match — areas can overlap, so copy byte by byte.
		mask := int32(65535)
		if !big {
			mask = 8191
		}
		k := (nextOffset - matchOff) & mask
		for matchLen > 0 {
			d.hist[nextOffset] = d.hist[k]
			nextOffset++
			k++
			matchLen--
		}
	}

	d.off = uint32(nextOffset)

	roff := uint32(oldOffset)
	rlen := uint32(nextOffset - oldOffset)

	// Return a copy so the caller owns the slice.
	out := make([]byte, rlen)
	copy(out, d.hist[roff:roff+rlen])
	return out, nil
}
