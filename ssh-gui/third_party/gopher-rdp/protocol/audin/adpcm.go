package audin

import "encoding/binary"

// IMA ADPCM step size table (89 entries, per MS-IMA specification).
var imaStepTable = [89]int16{
	7, 8, 9, 10, 11, 12, 13, 14,
	16, 17, 19, 21, 23, 25, 28, 31,
	34, 37, 41, 45, 50, 55, 60, 66,
	73, 80, 88, 97, 106, 117, 128, 141,
	155, 170, 187, 206, 226, 249, 274, 301,
	331, 364, 400, 440, 484, 532, 585, 643,
	707, 777, 854, 938, 1031, 1132, 1245, 1368,
	1503, 1652, 1816, 1996, 2195, 2414, 2655, 2919,
	3210, 3530, 3882, 4266, 4691, 5158, 5674, 6244,
	6867, 7551, 8301, 9126, 10033, 11030, 12124, 13327,
	14654, 16109, 17710, 19467, 21399, 23525, 25867, 28437,
	31266,
}

// IMA ADPCM index adjustment table for each 4-bit nibble.
var imaIndexTable = [16]int8{
	-1, -1, -1, -1, 2, 4, 6, 8,
	-1, -1, -1, -1, 2, 4, 6, 8,
}

// MS-ADPCM coefficient pairs (7 standard pairs per MS specification).
var msAdpcmCoeffs = [7][2]int32{
	{256, 0},
	{512, -256},
	{0, 0},
	{192, 64},
	{240, 0},
	{460, -208},
	{392, -232},
}

// MS-ADPCM adaptation table.
var msAdaptTable = [16]int32{
	230, 230, 230, 230, 307, 409, 512, 614,
	768, 614, 512, 409, 307, 230, 230, 230,
}

// IMAEncState holds per-channel IMA ADPCM encoder state carried across blocks.
// Keeping stepIndex continuous prevents amplitude blowup when transitioning
// between silence and speech (step index 0 = step size 7, far too small for
// real audio).
type IMAEncState struct {
	stepIndex [2]int32
}

// EncodeIMAADPCM encodes 16-bit PCM samples into a single IMA ADPCM block.
//
// Block layout mirrors the decoder: per-channel [predictor:i16][stepIndex:u8][reserved:u8]
// header, then 4-bit nibbles. For stereo, nibbles alternate in 8-sample chunks per channel.
//
// state carries stepIndex across blocks; pass nil to start fresh.
// dst is a reusable buffer; it will be grown if needed and the result slice returned.
func EncodeIMAADPCM(samples []int16, channels int, samplesPerBlock int, dst []byte, state *IMAEncState) []byte {
	if channels < 1 || channels > 2 {
		return dst[:0]
	}

	headerSize := 4 * channels
	// Data bytes: (samplesPerBlock - 1) samples as nibbles per channel.
	nibbleSamples := samplesPerBlock - 1
	var dataBytes int
	if channels == 1 {
		dataBytes = (nibbleSamples + 1) / 2
	} else {
		// Stereo: 8-sample chunks per channel, each chunk = 4 bytes.
		chunks := (nibbleSamples + 7) / 8
		dataBytes = chunks * 4 * 2
	}
	blockSize := headerSize + dataBytes

	if cap(dst) < blockSize {
		dst = make([]byte, blockSize)
	}
	dst = dst[:blockSize]

	// Clear the buffer.
	for i := range dst {
		dst[i] = 0
	}

	nSamples := len(samples) / channels

	// Write per-channel headers with first sample as predictor.
	// stepIndex is carried from previous block via state.
	var predictor [2]int32
	var stepIndex [2]int32
	if state != nil {
		stepIndex = state.stepIndex
	}
	for ch := 0; ch < channels; ch++ {
		if nSamples > 0 {
			if channels == 1 {
				predictor[ch] = int32(samples[0])
			} else {
				predictor[ch] = int32(samples[ch])
			}
		}
		off := ch * 4
		binary.LittleEndian.PutUint16(dst[off:], uint16(int16(predictor[ch])))
		dst[off+2] = byte(stepIndex[ch])
		dst[off+3] = 0
	}

	if channels == 1 {
		pred := predictor[0]
		si := stepIndex[0]
		byteOff := headerSize
		for i := 1; i < samplesPerBlock; i += 2 {
			var s0 int32
			if i < nSamples {
				s0 = int32(samples[i])
			}

			// Inline imaEncodeNibble for first nibble.
			step := int32(imaStepTable[si])
			diff := s0 - pred
			var nib0 byte
			if diff < 0 {
				nib0 = 8
				diff = -diff
			}
			if diff >= step {
				nib0 |= 4
				diff -= step
			}
			if diff >= step>>1 {
				nib0 |= 2
				diff -= step >> 1
			}
			if diff >= step>>2 {
				nib0 |= 1
			}
			// Reconstruct.
			d := step >> 3
			if nib0&4 != 0 {
				d += step
			}
			if nib0&2 != 0 {
				d += step >> 1
			}
			if nib0&1 != 0 {
				d += step >> 2
			}
			if nib0&8 != 0 {
				pred -= d
			} else {
				pred += d
			}
			if pred > 32767 {
				pred = 32767
			} else if pred < -32768 {
				pred = -32768
			}
			si += int32(imaIndexTable[nib0])
			if si < 0 {
				si = 0
			} else if si > 88 {
				si = 88
			}

			if i+1 < samplesPerBlock {
				var s1 int32
				if i+1 < nSamples {
					s1 = int32(samples[i+1])
				}

				// Inline imaEncodeNibble for second nibble.
				step = int32(imaStepTable[si])
				diff = s1 - pred
				var nib1 byte
				if diff < 0 {
					nib1 = 8
					diff = -diff
				}
				if diff >= step {
					nib1 |= 4
					diff -= step
				}
				if diff >= step>>1 {
					nib1 |= 2
					diff -= step >> 1
				}
				if diff >= step>>2 {
					nib1 |= 1
				}
				d = step >> 3
				if nib1&4 != 0 {
					d += step
				}
				if nib1&2 != 0 {
					d += step >> 1
				}
				if nib1&1 != 0 {
					d += step >> 2
				}
				if nib1&8 != 0 {
					pred -= d
				} else {
					pred += d
				}
				if pred > 32767 {
					pred = 32767
				} else if pred < -32768 {
					pred = -32768
				}
				si += int32(imaIndexTable[nib1])
				if si < 0 {
					si = 0
				} else if si > 88 {
					si = 88
				}

				dst[byteOff] = nib0 | (nib1 << 4)
				byteOff++
			} else {
				dst[byteOff] = nib0
				byteOff++
			}
		}
		if state != nil {
			state.stepIndex[0] = si
		}
	} else {
		// Stereo: 8-sample chunks per channel, interleaved.
		pred0, pred1 := predictor[0], predictor[1]
		si0, si1 := stepIndex[0], stepIndex[1]
		byteOff := headerSize
		sampleIdx := 1 // first sample is in header
		for sampleIdx < samplesPerBlock {
			// Left channel: 8 samples = 4 bytes.
			for j := 0; j < 8; j += 2 {
				sidx := sampleIdx + j
				var s0, s1 int32
				if sidx < nSamples {
					s0 = int32(samples[sidx*2])
				}
				// Inline encode nibble.
				step := int32(imaStepTable[si0])
				diff := s0 - pred0
				var nib0 byte
				if diff < 0 {
					nib0 = 8
					diff = -diff
				}
				if diff >= step {
					nib0 |= 4
					diff -= step
				}
				if diff >= step>>1 {
					nib0 |= 2
					diff -= step >> 1
				}
				if diff >= step>>2 {
					nib0 |= 1
				}
				d := step >> 3
				if nib0&4 != 0 {
					d += step
				}
				if nib0&2 != 0 {
					d += step >> 1
				}
				if nib0&1 != 0 {
					d += step >> 2
				}
				if nib0&8 != 0 {
					pred0 -= d
				} else {
					pred0 += d
				}
				if pred0 > 32767 {
					pred0 = 32767
				} else if pred0 < -32768 {
					pred0 = -32768
				}
				si0 += int32(imaIndexTable[nib0])
				if si0 < 0 {
					si0 = 0
				} else if si0 > 88 {
					si0 = 88
				}

				if sidx+1 < nSamples {
					s1 = int32(samples[(sidx+1)*2])
				}
				step = int32(imaStepTable[si0])
				diff = s1 - pred0
				var nib1 byte
				if diff < 0 {
					nib1 = 8
					diff = -diff
				}
				if diff >= step {
					nib1 |= 4
					diff -= step
				}
				if diff >= step>>1 {
					nib1 |= 2
					diff -= step >> 1
				}
				if diff >= step>>2 {
					nib1 |= 1
				}
				d = step >> 3
				if nib1&4 != 0 {
					d += step
				}
				if nib1&2 != 0 {
					d += step >> 1
				}
				if nib1&1 != 0 {
					d += step >> 2
				}
				if nib1&8 != 0 {
					pred0 -= d
				} else {
					pred0 += d
				}
				if pred0 > 32767 {
					pred0 = 32767
				} else if pred0 < -32768 {
					pred0 = -32768
				}
				si0 += int32(imaIndexTable[nib1])
				if si0 < 0 {
					si0 = 0
				} else if si0 > 88 {
					si0 = 88
				}

				dst[byteOff] = nib0 | (nib1 << 4)
				byteOff++
			}

			// Right channel: 8 samples = 4 bytes.
			for j := 0; j < 8; j += 2 {
				sidx := sampleIdx + j
				var s0, s1 int32
				if sidx < nSamples {
					s0 = int32(samples[sidx*2+1])
				}
				step := int32(imaStepTable[si1])
				diff := s0 - pred1
				var nib0 byte
				if diff < 0 {
					nib0 = 8
					diff = -diff
				}
				if diff >= step {
					nib0 |= 4
					diff -= step
				}
				if diff >= step>>1 {
					nib0 |= 2
					diff -= step >> 1
				}
				if diff >= step>>2 {
					nib0 |= 1
				}
				d := step >> 3
				if nib0&4 != 0 {
					d += step
				}
				if nib0&2 != 0 {
					d += step >> 1
				}
				if nib0&1 != 0 {
					d += step >> 2
				}
				if nib0&8 != 0 {
					pred1 -= d
				} else {
					pred1 += d
				}
				if pred1 > 32767 {
					pred1 = 32767
				} else if pred1 < -32768 {
					pred1 = -32768
				}
				si1 += int32(imaIndexTable[nib0])
				if si1 < 0 {
					si1 = 0
				} else if si1 > 88 {
					si1 = 88
				}

				if sidx+1 < nSamples {
					s1 = int32(samples[(sidx+1)*2+1])
				}
				step = int32(imaStepTable[si1])
				diff = s1 - pred1
				var nib1 byte
				if diff < 0 {
					nib1 = 8
					diff = -diff
				}
				if diff >= step {
					nib1 |= 4
					diff -= step
				}
				if diff >= step>>1 {
					nib1 |= 2
					diff -= step >> 1
				}
				if diff >= step>>2 {
					nib1 |= 1
				}
				d = step >> 3
				if nib1&4 != 0 {
					d += step
				}
				if nib1&2 != 0 {
					d += step >> 1
				}
				if nib1&1 != 0 {
					d += step >> 2
				}
				if nib1&8 != 0 {
					pred1 -= d
				} else {
					pred1 += d
				}
				if pred1 > 32767 {
					pred1 = 32767
				} else if pred1 < -32768 {
					pred1 = -32768
				}
				si1 += int32(imaIndexTable[nib1])
				if si1 < 0 {
					si1 = 0
				} else if si1 > 88 {
					si1 = 88
				}

				dst[byteOff] = nib0 | (nib1 << 4)
				byteOff++
			}
			sampleIdx += 8
		}
		if state != nil {
			state.stepIndex[0] = si0
			state.stepIndex[1] = si1
		}
	}

	return dst
}

// msEncodeNibble encodes a single sample as a 4-bit MS-ADPCM nibble,
// updating delta, samp1, samp2 in place. Returns the nibble (0..15).
func msEncodeNibble(sample int32, delta, samp1, samp2 *int32, coeff0, coeff1 int32) byte {
	predicted := ((*samp1)*coeff0 + (*samp2)*coeff1) >> 8
	error_ := sample - predicted

	// Quantise: nibble = error / delta, clamped to [-8, 7].
	var nibble int32
	if *delta != 0 {
		nibble = error_ / *delta
	}
	if nibble > 7 {
		nibble = 7
	} else if nibble < -8 {
		nibble = -8
	}

	// Reconstruct (same as decoder).
	reconstructed := predicted + nibble*(*delta)
	if reconstructed > 32767 {
		reconstructed = 32767
	} else if reconstructed < -32768 {
		reconstructed = -32768
	}

	*samp2 = *samp1
	*samp1 = reconstructed

	// Adapt delta.
	unsignedNib := nibble
	if unsignedNib < 0 {
		unsignedNib += 16
	}
	*delta = (*delta * msAdaptTable[unsignedNib]) >> 8
	if *delta < 16 {
		*delta = 16
	}

	if nibble < 0 {
		return byte(nibble + 16)
	}
	return byte(nibble)
}

// chooseMSCoefficient picks the best coefficient pair index (0..6) for
// a block of interleaved int16 samples on one channel.
func chooseMSCoefficient(samples []int16, offset int, stride int, count int) int {
	if count < 3 {
		return 0
	}

	bestIdx := 0
	bestErr := int64(1<<62 - 1)

	for ci := 0; ci < 7; ci++ {
		c0, c1 := msAdpcmCoeffs[ci][0], msAdpcmCoeffs[ci][1]
		var totalErr int64
		for i := 2; i < count && i < 10; i++ {
			predicted := (int64(samples[offset+(i-1)*stride])*int64(c0) + int64(samples[offset+(i-2)*stride])*int64(c1)) >> 8
			diff := int64(samples[offset+i*stride]) - predicted
			totalErr += diff * diff
			if totalErr >= bestErr {
				break
			}
		}
		if totalErr < bestErr {
			bestErr = totalErr
			bestIdx = ci
		}
	}
	return bestIdx
}

// computeInitialDelta returns a reasonable starting delta for MS-ADPCM encoding
// by looking at sample magnitudes.
func computeInitialDelta(samples []int16, offset int, stride int, count int) int32 {
	if count < 2 {
		return 16
	}
	var maxDiff int32
	for i := 1; i < count && i < 32; i++ {
		d := int32(samples[offset+i*stride]) - int32(samples[offset+(i-1)*stride])
		if d < 0 {
			d = -d
		}
		if d > maxDiff {
			maxDiff = d
		}
	}
	// delta ≈ maxDiff / 4, but at least 16.
	delta := maxDiff / 4
	if delta < 16 {
		delta = 16
	}
	return delta
}

// encodeMSADPCM encodes 16-bit PCM samples into a single MS-ADPCM block.
//
// Block layout: [bPredictor:u8 × nCh][iDelta:i16 × nCh][iSamp1:i16 × nCh]
// [iSamp2:i16 × nCh] then nibble pairs (high nibble first).
//
// dst is a reusable buffer; it will be grown if needed and the result slice returned.
func encodeMSADPCM(samples []int16, channels int, samplesPerBlock int, blockAlign int, dst []byte) []byte {
	if channels < 1 || channels > 2 {
		return dst[:0]
	}

	headerSize := 7 * channels
	if blockAlign < headerSize {
		return dst[:0]
	}

	if cap(dst) < blockAlign {
		dst = make([]byte, blockAlign)
	}
	dst = dst[:blockAlign]

	for i := range dst {
		dst[i] = 0
	}

	nSamples := len(samples) / channels

	var coeff [2][2]int32
	var delta [2]int32
	var samp1 [2]int32
	var samp2 [2]int32

	off := 0
	// Write predictor indices — work directly on strided int16 data.
	for ch := 0; ch < channels; ch++ {
		ci := chooseMSCoefficient(samples, ch, channels, nSamples)
		coeff[ch] = msAdpcmCoeffs[ci]
		dst[off] = byte(ci)
		off++
	}

	// Write deltas.
	for ch := 0; ch < channels; ch++ {
		delta[ch] = computeInitialDelta(samples, ch, channels, nSamples)
		binary.LittleEndian.PutUint16(dst[off:], uint16(int16(delta[ch])))
		off += 2
	}

	// Write samp1 (most recent = samples[1] if available).
	for ch := 0; ch < channels; ch++ {
		if nSamples > 1 {
			samp1[ch] = int32(samples[1*channels+ch])
		} else if nSamples > 0 {
			samp1[ch] = int32(samples[ch])
		}
		binary.LittleEndian.PutUint16(dst[off:], uint16(int16(samp1[ch])))
		off += 2
	}

	// Write samp2 (second most recent = samples[0]).
	for ch := 0; ch < channels; ch++ {
		if nSamples > 0 {
			samp2[ch] = int32(samples[ch])
		}
		binary.LittleEndian.PutUint16(dst[off:], uint16(int16(samp2[ch])))
		off += 2
	}

	// Encode remaining samples as nibble pairs (high nibble first).
	// The first two samples per channel are in the header (samp2, samp1).
	ch := 0
	sampleIdx := 2
	nibbleCount := 0
	var currentByte byte

	for sampleIdx < samplesPerBlock && off < blockAlign {
		var s int32
		if sampleIdx < nSamples {
			s = int32(samples[sampleIdx*channels+ch])
		}
		nib := msEncodeNibble(s, &delta[ch], &samp1[ch], &samp2[ch], coeff[ch][0], coeff[ch][1])

		if nibbleCount%2 == 0 {
			currentByte = nib << 4
		} else {
			currentByte |= nib
			dst[off] = currentByte
			off++
		}
		nibbleCount++

		if channels == 2 {
			ch ^= 1
			if ch == 0 {
				sampleIdx++
			}
		} else {
			sampleIdx++
		}
	}

	// Flush last nibble if odd.
	if nibbleCount%2 != 0 && off < blockAlign {
		dst[off] = currentByte
		off++
	}

	return dst
}
