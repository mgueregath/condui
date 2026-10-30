package rdpsnd

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

// MS-ADPCM adaptation table.
var msAdaptTable = [16]int32{
	230, 230, 230, 230, 307, 409, 512, 614,
	768, 614, 512, 409, 307, 230, 230, 230,
}

// decodeIMAADPCM decodes IMA ADPCM data to 16-bit PCM samples.
//
// Block layout per channel: [predictor:i16 LE][stepIndex:u8][reserved:u8] (4 bytes header),
// then interleaved 4-bit nibbles. For stereo, nibbles alternate 8 samples per channel.
//
// dst is a reusable buffer; it will be grown if needed and the result slice returned.
func decodeIMAADPCM(src []byte, channels int, samplesPerBlock int, blockAlign int, dst []int16) []int16 {
	if channels < 1 || channels > 2 {
		return dst[:0]
	}

	headerSize := 4 * channels
	if blockAlign < headerSize {
		return dst[:0]
	}

	// Calculate total output samples across all blocks.
	totalBlocks := len(src) / blockAlign
	totalSamples := samplesPerBlock * channels * totalBlocks

	// Ensure dst has capacity.
	if cap(dst) < totalSamples {
		dst = make([]int16, totalSamples)
	}
	dst = dst[:totalSamples]

	outOff := 0
	for block := 0; block < totalBlocks; block++ {
		blockStart := block * blockAlign
		blockEnd := blockStart + blockAlign
		if blockEnd > len(src) {
			break
		}
		blk := src[blockStart:blockEnd]
		if channels == 1 {
			outOff = decodeIMAMonoBlock(blk, samplesPerBlock, dst, outOff)
		} else {
			outOff = decodeIMAStereoBlock(blk, samplesPerBlock, dst, outOff)
		}
	}

	return dst[:outOff]
}

// decodeIMAMonoBlock decodes a single mono IMA ADPCM block with inlined nibble logic.
func decodeIMAMonoBlock(blk []byte, samplesPerBlock int, dst []int16, outOff int) int {
	if len(blk) < 4 {
		return outOff
	}

	predictor := int32(int16(binary.LittleEndian.Uint16(blk[0:])))
	stepIndex := int32(blk[2])
	if stepIndex < 0 {
		stepIndex = 0
	} else if stepIndex > 88 {
		stepIndex = 88
	}

	dst[outOff] = int16(predictor)
	outOff++

	dataBytes := blk[4:]
	samplesDecoded := 1

	for i := 0; i < len(dataBytes) && samplesDecoded < samplesPerBlock; i++ {
		b := dataBytes[i]

		// Low nibble — inlined imaDecodeNibble.
		nibble := b & 0x0F
		step := int32(imaStepTable[stepIndex])
		diff := step >> 3
		if nibble&4 != 0 {
			diff += step
		}
		if nibble&2 != 0 {
			diff += step >> 1
		}
		if nibble&1 != 0 {
			diff += step >> 2
		}
		if nibble&8 != 0 {
			predictor -= diff
		} else {
			predictor += diff
		}
		if predictor > 32767 {
			predictor = 32767
		} else if predictor < -32768 {
			predictor = -32768
		}
		stepIndex += int32(imaIndexTable[nibble])
		if stepIndex < 0 {
			stepIndex = 0
		} else if stepIndex > 88 {
			stepIndex = 88
		}
		dst[outOff] = int16(predictor)
		outOff++
		samplesDecoded++
		if samplesDecoded >= samplesPerBlock {
			break
		}

		// High nibble — inlined imaDecodeNibble.
		nibble = b >> 4
		step = int32(imaStepTable[stepIndex])
		diff = step >> 3
		if nibble&4 != 0 {
			diff += step
		}
		if nibble&2 != 0 {
			diff += step >> 1
		}
		if nibble&1 != 0 {
			diff += step >> 2
		}
		if nibble&8 != 0 {
			predictor -= diff
		} else {
			predictor += diff
		}
		if predictor > 32767 {
			predictor = 32767
		} else if predictor < -32768 {
			predictor = -32768
		}
		stepIndex += int32(imaIndexTable[nibble])
		if stepIndex < 0 {
			stepIndex = 0
		} else if stepIndex > 88 {
			stepIndex = 88
		}
		dst[outOff] = int16(predictor)
		outOff++
		samplesDecoded++
	}

	return outOff
}

// decodeIMAStereoBlock decodes a stereo IMA ADPCM block with proper interleaving.
// Nibble decode is inlined to avoid pointer indirection per sample.
func decodeIMAStereoBlock(blk []byte, samplesPerBlock int, dst []int16, outOff int) int {
	if len(blk) < 8 {
		return outOff
	}

	var predictor [2]int32
	var stepIndex [2]int32

	for ch := 0; ch < 2; ch++ {
		off := ch * 4
		predictor[ch] = int32(int16(binary.LittleEndian.Uint16(blk[off:])))
		stepIndex[ch] = int32(blk[off+2])
		if stepIndex[ch] > 88 {
			stepIndex[ch] = 88
		}
	}

	// Write initial samples (interleaved: L, R).
	if outOff+1 >= len(dst) {
		return outOff
	}
	dst[outOff] = int16(predictor[0])
	dst[outOff+1] = int16(predictor[1])
	outOff += 2

	dataBytes := blk[8:]
	samplesDecoded := 1

	// Stereo IMA: data comes in 8-byte chunks (4 bytes per channel).
	// Each 4 bytes = 8 nibbles = 8 samples for that channel.
	byteOff := 0
	for samplesDecoded < samplesPerBlock {
		var chSamples [2][8]int16
		chCount := [2]int{0, 0}

		for ch := 0; ch < 2; ch++ {
			pred := predictor[ch]
			si := stepIndex[ch]
			cnt := 0
			for j := 0; j < 4 && byteOff < len(dataBytes); j++ {
				b := dataBytes[byteOff]
				byteOff++

				// Low nibble — inlined.
				nibble := b & 0x0F
				step := int32(imaStepTable[si])
				diff := step >> 3
				if nibble&4 != 0 {
					diff += step
				}
				if nibble&2 != 0 {
					diff += step >> 1
				}
				if nibble&1 != 0 {
					diff += step >> 2
				}
				if nibble&8 != 0 {
					pred -= diff
				} else {
					pred += diff
				}
				if pred > 32767 {
					pred = 32767
				} else if pred < -32768 {
					pred = -32768
				}
				si += int32(imaIndexTable[nibble])
				if si < 0 {
					si = 0
				} else if si > 88 {
					si = 88
				}
				chSamples[ch][cnt] = int16(pred)
				cnt++

				// High nibble — inlined.
				nibble = b >> 4
				step = int32(imaStepTable[si])
				diff = step >> 3
				if nibble&4 != 0 {
					diff += step
				}
				if nibble&2 != 0 {
					diff += step >> 1
				}
				if nibble&1 != 0 {
					diff += step >> 2
				}
				if nibble&8 != 0 {
					pred -= diff
				} else {
					pred += diff
				}
				if pred > 32767 {
					pred = 32767
				} else if pred < -32768 {
					pred = -32768
				}
				si += int32(imaIndexTable[nibble])
				if si < 0 {
					si = 0
				} else if si > 88 {
					si = 88
				}
				chSamples[ch][cnt] = int16(pred)
				cnt++
			}
			predictor[ch] = pred
			stepIndex[ch] = si
			chCount[ch] = cnt
		}

		// Interleave: L0 R0 L1 R1 ...
		n := chCount[0]
		if chCount[1] < n {
			n = chCount[1]
		}
		// Cap to remaining samples to prevent writing past dst.
		if remaining := samplesPerBlock - samplesDecoded; n > remaining {
			n = remaining
		}
		for i := 0; i < n; i++ {
			dst[outOff] = chSamples[0][i]
			dst[outOff+1] = chSamples[1][i]
			outOff += 2
		}
		samplesDecoded += n
	}

	return outOff
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

// decodeMSADPCM decodes MS-ADPCM data to 16-bit PCM samples.
//
// Block layout: [bPredictor:u8 × nCh] [iDelta:i16 × nCh] [iSamp1:i16 × nCh]
// [iSamp2:i16 × nCh] then 4-bit nibble pairs (high nibble first).
//
// dst is a reusable buffer; it will be grown if needed and the result slice returned.
func decodeMSADPCM(src []byte, channels int, samplesPerBlock int, blockAlign int, dst []int16) []int16 {
	if channels < 1 || channels > 2 || blockAlign < 7*channels {
		return dst[:0]
	}

	totalBlocks := len(src) / blockAlign
	if totalBlocks == 0 && len(src) >= blockAlign {
		totalBlocks = 1
	}
	totalSamples := samplesPerBlock * channels * totalBlocks

	if cap(dst) < totalSamples {
		dst = make([]int16, totalSamples)
	}
	dst = dst[:totalSamples]

	outOff := 0
	for block := 0; block < totalBlocks; block++ {
		blockStart := block * blockAlign
		blockEnd := blockStart + blockAlign
		if blockEnd > len(src) {
			break
		}
		outOff = decodeMSBlock(src[blockStart:blockEnd], channels, samplesPerBlock, dst, outOff)
	}

	return dst[:outOff]
}

// decodeMSBlock decodes a single MS-ADPCM block.
func decodeMSBlock(blk []byte, channels int, samplesPerBlock int, dst []int16, outOff int) int {
	headerSize := 7 * channels // predictor(1) + delta(2) + samp1(2) + samp2(2) per channel
	if len(blk) < headerSize {
		return outOff
	}

	var coeff [2][2]int32
	var delta [2]int32
	var samp1 [2]int32
	var samp2 [2]int32

	off := 0
	// Read predictor indices.
	for ch := 0; ch < channels; ch++ {
		idx := int(blk[off])
		off++
		if idx > 6 {
			idx = 0
		}
		coeff[ch] = msAdpcmCoeffs[idx]
	}

	// Read deltas.
	for ch := 0; ch < channels; ch++ {
		delta[ch] = int32(int16(binary.LittleEndian.Uint16(blk[off:])))
		off += 2
	}

	// Read samp1 (most recent sample).
	for ch := 0; ch < channels; ch++ {
		samp1[ch] = int32(int16(binary.LittleEndian.Uint16(blk[off:])))
		off += 2
	}

	// Read samp2 (second most recent sample).
	for ch := 0; ch < channels; ch++ {
		samp2[ch] = int32(int16(binary.LittleEndian.Uint16(blk[off:])))
		off += 2
	}

	// Output first two samples per channel (samp2 first, then samp1).
	if channels == 1 {
		if outOff+1 >= len(dst) {
			return outOff
		}
		dst[outOff] = int16(samp2[0])
		dst[outOff+1] = int16(samp1[0])
		outOff += 2
	} else {
		if outOff+3 >= len(dst) {
			return outOff
		}
		dst[outOff] = int16(samp2[0])
		dst[outOff+1] = int16(samp2[1])
		dst[outOff+2] = int16(samp1[0])
		dst[outOff+3] = int16(samp1[1])
		outOff += 4
	}

	samplesDecoded := 2
	dataBytes := blk[off:]
	ch := 0

	for i := 0; i < len(dataBytes) && samplesDecoded < samplesPerBlock; i++ {
		// High nibble first — inlined msDecodeNibble.
		nibble := int32(dataBytes[i] >> 4)
		if nibble >= 8 {
			nibble -= 16
		}
		predictor := ((samp1[ch])*coeff[ch][0] + (samp2[ch])*coeff[ch][1]) >> 8
		predictor += nibble * delta[ch]
		if predictor > 32767 {
			predictor = 32767
		} else if predictor < -32768 {
			predictor = -32768
		}
		samp2[ch] = samp1[ch]
		samp1[ch] = predictor
		idx := nibble
		if idx < 0 {
			idx += 16
		}
		delta[ch] = (delta[ch] * msAdaptTable[idx]) >> 8
		if delta[ch] < 16 {
			delta[ch] = 16
		}
		dst[outOff] = int16(predictor)
		outOff++

		if channels == 2 {
			ch ^= 1
			if ch == 0 {
				samplesDecoded++
				if samplesDecoded >= samplesPerBlock {
					break
				}
			}
		} else {
			samplesDecoded++
			if samplesDecoded >= samplesPerBlock {
				break
			}
		}

		// Low nibble — inlined msDecodeNibble.
		nibble = int32(dataBytes[i] & 0x0F)
		if nibble >= 8 {
			nibble -= 16
		}
		predictor = ((samp1[ch])*coeff[ch][0] + (samp2[ch])*coeff[ch][1]) >> 8
		predictor += nibble * delta[ch]
		if predictor > 32767 {
			predictor = 32767
		} else if predictor < -32768 {
			predictor = -32768
		}
		samp2[ch] = samp1[ch]
		samp1[ch] = predictor
		idx = nibble
		if idx < 0 {
			idx += 16
		}
		delta[ch] = (delta[ch] * msAdaptTable[idx]) >> 8
		if delta[ch] < 16 {
			delta[ch] = 16
		}
		dst[outOff] = int16(predictor)
		outOff++

		if channels == 2 {
			ch ^= 1
			if ch == 0 {
				samplesDecoded++
			}
		} else {
			samplesDecoded++
		}
	}

	return outOff
}
