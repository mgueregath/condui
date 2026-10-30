// Package rfx implements the RemoteFX Progressive codec (MS-RDPRFX / MS-RDPEGFX).
package rfx

import "math/bits"

// RLGR (Run-Length Golomb-Rice) decoder per MS-RDPRFX 3.1.8.1.7.
//
// Key design points:
//   - Mode is determined by k value (k>0 = RL mode, k==0 = GR mode), NOT a mode bit
//   - GR mode uses interleaved sign encoding (code&1 = negative)
//   - RL mode: run of zeros then sign+GR magnitude (always non-zero)
//   - RLGR3: single GR code split into val1+val2 (not two separate GR values)
//   - Parameter adaptation uses LSGR=3 fractional scaling

const (
	lsgr  = 3
	kpmax = 80
	upGR  = 4 // RL mode: kp increment per leading zero
	dnGR  = 6 // RL mode: kp decrement after magnitude
	uqGR  = 3 // GR mode: kp increment when code==0
	dqGR  = 3 // GR mode: kp decrement when code!=0
)

// bitReader reads bits MSB-first from a byte slice using a 64-bit accumulator.
// Bits are left-aligned in acc: the next bit to read is always at bit 63.
// Invariant: the bottom (64-bits) bits of acc are always zero.
type bitReader struct {
	data []byte
	pos  int    // next byte position to load
	bits uint32 // number of valid bits in acc
	acc  uint64 // accumulator, MSB-aligned
}

func (r *bitReader) remaining() int {
	return (len(r.data)-r.pos)*8 + int(r.bits)
}

// refill loads bytes into the accumulator until it has at least 56 bits
// or the input is exhausted.
func (r *bitReader) refill() {
	for r.bits <= 56 && r.pos < len(r.data) {
		r.acc |= uint64(r.data[r.pos]) << (56 - r.bits)
		r.pos++
		r.bits += 8
	}
}

func (r *bitReader) readBit() uint32 {
	if r.bits == 0 {
		r.refill()
		if r.bits == 0 {
			return 0
		}
	}
	r.bits--
	bit := uint32(r.acc >> 63)
	r.acc <<= 1
	return bit
}

// readBits reads n bits as a single operation (no per-bit loop).
func (r *bitReader) readBits(n uint32) uint32 {
	if n == 0 {
		return 0
	}
	if r.bits < n {
		r.refill()
	}
	if r.bits < n {
		// Not enough bits remaining; read what's available
		if r.bits == 0 {
			return 0
		}
		val := uint32(r.acc >> (64 - n))
		r.acc = 0
		r.bits = 0
		return val
	}
	val := uint32(r.acc >> (64 - n))
	r.acc <<= n
	r.bits -= n
	return val
}

// countLeadingZeros counts consecutive 0 bits (terminated by 1 which is consumed).
// Uses hardware CLZ via math/bits instead of per-bit loop.
func (r *bitReader) countLeadingZeros() uint32 {
	var count uint32
	for {
		if r.bits == 0 {
			r.refill()
			if r.bits == 0 {
				return count
			}
		}
		lz := uint32(bits.LeadingZeros64(r.acc))
		if lz < r.bits {
			// Found terminating 1 within valid bits
			count += lz
			lz++
			r.acc <<= lz
			r.bits -= lz
			return count
		}
		// All valid bits are zeros
		count += r.bits
		r.bits = 0
		r.acc = 0
	}
}

// countLeadingOnes counts consecutive 1 bits (terminated by 0 which is consumed).
// Uses hardware CLZ via math/bits instead of per-bit loop.
func (r *bitReader) countLeadingOnes() uint32 {
	var count uint32
	for {
		if r.bits == 0 {
			r.refill()
			if r.bits == 0 {
				return count
			}
		}
		lo := uint32(bits.LeadingZeros64(^r.acc))
		if lo < r.bits {
			count += lo
			lo++
			r.acc <<= lo
			r.bits -= lo
			return count
		}
		count += r.bits
		r.bits = 0
		r.acc = 0
	}
}

// rlgr1Decode decodes RLGR1 encoded data into dst[:n] coefficients.
// dst must have len >= n. It is zeroed before use. Returns dst[:n].
// Decodes RLGR1 encoded data per MS-RDPRFX 3.1.8.1.7.1.
func rlgr1Decode(dst []int16, data []byte) []int16 {
	n := len(dst)
	out := dst[:n]
	for i := range out {
		out[i] = 0
	}
	if len(data) == 0 {
		return out
	}

	r := &bitReader{data: data}
	var k, kp, kr, krp uint32
	k = 1
	kp = k << lsgr
	kr = 1
	krp = kr << lsgr
	idx := 0

	for r.remaining() > 0 && idx < n {
		if k > 0 {
			// RL Mode: run of zeros then one non-zero value

			// Count leading zeros → run quotient
			vk := r.countLeadingZeros()

			// Each leading zero adds (1 << k) to run, and updates k
			var run uint32
			for i := uint32(0); i < vk; i++ {
				run += 1 << k
				kp += upGR
				if kp > kpmax {
					kp = kpmax
				}
				k = kp >> lsgr
			}

			// Next k bits = run remainder
			if r.remaining() < int(k) {
				break
			}
			run += r.readBits(k)

			// Emit zeros
			end := idx + int(run)
			if end > n {
				end = n
			}
			idx = end

			if idx >= n {
				break
			}

			// Read sign bit
			if r.remaining() < 1 {
				break
			}
			sign := r.readBit()

			// Count leading ones → magnitude quotient
			vk = r.countLeadingOnes()

			// Next kr bits = code remainder
			if r.remaining() < int(kr) {
				break
			}
			var code uint32
			if kr > 0 {
				code = r.readBits(kr)
			}
			code |= vk << kr

			// Update kr/krp
			if vk == 0 {
				if krp > 2 {
					krp -= 2
				} else {
					krp = 0
				}
				kr = krp >> lsgr
			} else if vk != 1 {
				krp += vk
				if krp > kpmax {
					krp = kpmax
				}
				kr = krp >> lsgr
			}

			// Update k/kp (decrease after RL mode)
			if kp > dnGR {
				kp -= dnGR
			} else {
				kp = 0
			}
			k = kp >> lsgr

			// Emit magnitude
			if sign != 0 {
				out[idx] = -int16(code + 1)
			} else {
				out[idx] = int16(code + 1)
			}
			idx++

		} else {
			// GR Mode: single value with interleaved sign

			// Count leading ones → quotient
			vk := r.countLeadingOnes()

			// Next kr bits = code remainder
			if r.remaining() < int(kr) {
				break
			}
			var code uint32
			if kr > 0 {
				code = r.readBits(kr)
			}
			code |= vk << kr

			// Update kr/krp
			if vk == 0 {
				if krp > 2 {
					krp -= 2
				} else {
					krp = 0
				}
				kr = krp >> lsgr
			} else if vk != 1 {
				krp += vk
				if krp > kpmax {
					krp = kpmax
				}
				kr = krp >> lsgr
			}

			// RLGR1: interleaved sign encoding
			if code == 0 {
				// Zero value → increase kp (switch toward RL mode)
				kp += uqGR
				if kp > kpmax {
					kp = kpmax
				}
				k = kp >> lsgr
				out[idx] = 0
			} else {
				// Non-zero → decrease kp (stay in GR mode)
				if kp > dqGR {
					kp -= dqGR
				} else {
					kp = 0
				}
				k = kp >> lsgr

				// Interleaved sign: odd → negative, even → positive
				if code&1 != 0 {
					out[idx] = -int16((code + 1) >> 1)
				} else {
					out[idx] = int16(code >> 1)
				}
			}
			idx++
		}
	}

	return out
}

// rlgr3Decode decodes RLGR3 encoded data into dst[:n] coefficients.
// dst must have len >= n. It is zeroed before use. Returns dst[:n].
// Decodes RLGR3 encoded data per MS-RDPRFX 3.1.8.1.7.3.
// RLGR3 differs: GR mode decodes pairs by splitting the code into val1+val2.
func rlgr3Decode(dst []int16, data []byte) []int16 {
	n := len(dst)
	out := dst[:n]
	for i := range out {
		out[i] = 0
	}
	if len(data) == 0 {
		return out
	}

	r := &bitReader{data: data}
	var k, kp, kr, krp uint32
	k = 1
	kp = k << lsgr
	kr = 1
	krp = kr << lsgr
	idx := 0

	for r.remaining() > 0 && idx < n {
		if k > 0 {
			// RL Mode: identical to RLGR1

			vk := r.countLeadingZeros()

			var run uint32
			for i := uint32(0); i < vk; i++ {
				run += 1 << k
				kp += upGR
				if kp > kpmax {
					kp = kpmax
				}
				k = kp >> lsgr
			}

			if r.remaining() < int(k) {
				break
			}
			run += r.readBits(k)

			end := idx + int(run)
			if end > n {
				end = n
			}
			idx = end

			if idx >= n {
				break
			}

			if r.remaining() < 1 {
				break
			}
			sign := r.readBit()

			vk = r.countLeadingOnes()

			if r.remaining() < int(kr) {
				break
			}
			var code uint32
			if kr > 0 {
				code = r.readBits(kr)
			}
			code |= vk << kr

			if vk == 0 {
				if krp > 2 {
					krp -= 2
				} else {
					krp = 0
				}
				kr = krp >> lsgr
			} else if vk != 1 {
				krp += vk
				if krp > kpmax {
					krp = kpmax
				}
				kr = krp >> lsgr
			}

			if kp > dnGR {
				kp -= dnGR
			} else {
				kp = 0
			}
			k = kp >> lsgr

			if sign != 0 {
				out[idx] = -int16(code + 1)
			} else {
				out[idx] = int16(code + 1)
			}
			idx++

		} else {
			// GR Mode: decode pair (RLGR3 specific)

			vk := r.countLeadingOnes()

			if r.remaining() < int(kr) {
				break
			}
			var code uint32
			if kr > 0 {
				code = r.readBits(kr)
			}
			code |= vk << kr

			if vk == 0 {
				if krp > 2 {
					krp -= 2
				} else {
					krp = 0
				}
				kr = krp >> lsgr
			} else if vk != 1 {
				krp += vk
				if krp > kpmax {
					krp = kpmax
				}
				kr = krp >> lsgr
			}

			// Split code into val1 and val2
			var nIdx, val1, val2 uint32
			if code != 0 {
				nIdx = uint32(bits.Len32(code))
			}

			if r.remaining() < int(nIdx) {
				break
			}
			if nIdx > 0 {
				val1 = r.readBits(nIdx)
			}
			val2 = code - val1

			// Update k/kp
			if val1 != 0 && val2 != 0 {
				if kp > 2*dqGR {
					kp -= 2 * dqGR
				} else {
					kp = 0
				}
				k = kp >> lsgr
			} else if val1 == 0 && val2 == 0 {
				kp += 2 * uqGR
				if kp > kpmax {
					kp = kpmax
				}
				k = kp >> lsgr
			}

			// Emit val1 (interleaved sign)
			if val1&1 != 0 {
				out[idx] = -int16((val1 + 1) >> 1)
			} else {
				out[idx] = int16(val1 >> 1)
			}
			idx++

			// Emit val2 (interleaved sign)
			if idx < n {
				if val2&1 != 0 {
					out[idx] = -int16((val2 + 1) >> 1)
				} else {
					out[idx] = int16(val2 >> 1)
				}
				idx++
			}
		}
	}

	return out
}
