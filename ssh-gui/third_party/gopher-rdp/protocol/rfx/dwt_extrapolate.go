package rfx

// Inverse 2D DWT (extrapolate path) for RemoteFX Progressive codec.
// Uses non-power-of-2 subband sizes to handle boundary effects.
//
// The progressive codec uses non-power-of-2 subband sizes to handle boundary
// effects. Each DWT level has asymmetric low/high band dimensions:
//   Level 1: L=33, H=31 → 64x64 output
//   Level 2: L=17, H=16 → 33x33 output
//   Level 3: L=9,  H=8  → 17x17 output
//
// Subband layout in the 4096-element buffer:
//   [HL1(1023), LH1(1023), HH1(961), HL2(272), LH2(272), HH2(256),
//    HL3(72), LH3(72), HH3(64), LL3(81)]
//
// Lifting filter (different from standard RFX):
//   even = L - (H_left + H_right) / 2
//   odd  = (even_left + even_right) / 2 + 2 * H

// clampi16 saturates an int32 to the int16 range [-32768, 32767],
// matching FreeRDP's clampi16() in progressive.c. Go's int16() truncates
// (wraps), which produces wildly incorrect values when DWT intermediates
// overflow — corrupting high-contrast edges like text and UI borders.
func clampi16(v int32) int16 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return int16(v)
}

func extBandL(level int) int {
	return (64 >> uint(level)) + 1
}

func extBandH(level int) int {
	if level == 1 {
		return (64 >> 1) - 1
	}
	return (64 + (1 << uint(level-1))) >> uint(level)
}

// inverseDWTExtrapolate performs 3-level 2D inverse DWT using the progressive
// codec's extrapolate path (non-power-of-2 subband sizes).
// tmp is a scratch buffer of at least 4096 elements.
func inverseDWTExtrapolate(coeffs, tmp []int16) []int16 {
	if len(coeffs) < tileSize*tileSize {
		return coeffs
	}
	// Level 3: HL3 starts at offset 3807
	dwtLevelExt(coeffs[3807:], tmp, 3)
	// Level 2: HL2 starts at offset 3007
	dwtLevelExt(coeffs[3007:], tmp, 2)
	// Level 1: HL1 starts at offset 0
	dwtLevelExt(coeffs[0:], tmp, 1)
	return coeffs
}

// dwtLevelExt performs one level of 2D inverse DWT with extrapolation.
// buf starts at the level's HL band; output overwrites buf[0:nDst*nDst].
//
// Band layout at buf:
//   HL: nH cols × nL rows (nH*nL elements)
//   LH: nL cols × nH rows (nL*nH elements)
//   HH: nH cols × nH rows (nH*nH elements)
//   LL: nL cols × nL rows (nL*nL elements)
func dwtLevelExt(buf, tmp []int16, level int) {
	nL := extBandL(level)
	nH := extBandH(level)
	nDst := nL + nH

	off := 0
	hlOff := off
	off += nH * nL // HL
	lhOff := off
	off += nL * nH // LH
	hhOff := off
	off += nH * nH // HH
	llOff := off   // LL

	L := tmp[:nL*nDst]
	H := tmp[nL*nDst : (nL+nH)*nDst]

	// Horizontal IDWT: LL(low) + HL(high) → L (nL rows)
	idwtHoriz(buf[llOff:], nL, buf[hlOff:], nH, L, nDst, nL, nH, nL)
	// Horizontal IDWT: LH(low) + HH(high) → H (nH rows)
	idwtHoriz(buf[lhOff:], nL, buf[hhOff:], nH, H, nDst, nL, nH, nH)
	// Vertical IDWT: L(low rows) + H(high rows) → output at buf[0:]
	idwtVert(L, nDst, H, nDst, buf, nDst, nL, nH, nDst)
}

// idwtHoriz performs horizontal inverse DWT for nRows rows.
// Each row: nLow low-freq coefficients + nHigh high-freq → nLow+nHigh output.
// Lifting filter (extrapolate path):
//
//	even = L - (H_left + H_right) / 2
//	odd  = (even_left + even_right) / 2 + 2 * H
//
// clampi16() saturates intermediates to int16 range, matching FreeRDP progressive.c.
func idwtHoriz(low []int16, lowStep int, high []int16, highStep int,
	dst []int16, dstStep int, nLow, nHigh, nRows int) {

	nDst := nLow + nHigh
	for i := 0; i < nRows; i++ {
		pL := low[i*lowStep:][:nLow]
		pH := high[i*highStep:][:nHigh]
		pX := dst[i*dstStep:][:nDst]

		li, hi := 1, 1
		h0 := int32(pH[0])
		l0 := int32(pL[0])
		x0 := clampi16(l0 - h0)
		x2 := x0
		xi := 0

		for j := 0; j < nHigh-1; j++ {
			h1 := int32(pH[hi])
			hi++
			l0 = int32(pL[li])
			li++
			x2 = clampi16(l0 - (h0+h1)/2)
			x1 := clampi16((int32(x0)+int32(x2))/2 + 2*h0)
			pX[xi] = x0
			xi++
			pX[xi] = x1
			xi++
			x0 = x2
			h0 = h1
		}

		if nLow <= nHigh+1 {
			if nLow <= nHigh {
				pX[xi] = x2
				xi++
				pX[xi] = clampi16(int32(x2) + 2*h0)
			} else {
				l0 = int32(pL[li])
				x0 = clampi16(l0 - h0)
				pX[xi] = x2
				xi++
				pX[xi] = clampi16((int32(x0)+int32(x2))/2 + 2*h0)
				xi++
				pX[xi] = x0
			}
		} else {
			l0 = int32(pL[li])
			li++
			x0 = clampi16(l0 - h0/2)
			pX[xi] = x2
			xi++
			pX[xi] = clampi16((int32(x0)+int32(x2))/2 + 2*h0)
			xi++
			pX[xi] = x0
			xi++
			l0 = int32(pL[li])
			pX[xi] = clampi16((int32(x0) + l0) / 2)
		}
	}
}

// idwtVert performs vertical inverse DWT for nCols columns.
// Each column: nLow low-freq rows + nHigh high-freq rows → nLow+nHigh output rows.
//
// Processes 4 columns at a time so adjacent column loads share cache lines,
// then handles remaining columns one at a time.
func idwtVert(low []int16, lowStep int, high []int16, highStep int,
	dst []int16, dstStep int, nLow, nHigh, nCols int) {

	i := 0
	for ; i+4 <= nCols; i += 4 {
		idwtVertCol4(low, lowStep, high, highStep, dst, dstStep, nLow, nHigh, i)
	}
	for ; i < nCols; i++ {
		idwtVertCol1(low, lowStep, high, highStep, dst, dstStep, nLow, nHigh, i)
	}
}

// idwtVertCol4 processes 4 adjacent columns simultaneously.
func idwtVertCol4(low []int16, lowStep int, high []int16, highStep int,
	dst []int16, dstStep int, nLow, nHigh, col int) {

	lOff := col + lowStep
	hOff := col + highStep
	xOff := col

	h0a, h0b, h0c, h0d := int32(high[col]), int32(high[col+1]), int32(high[col+2]), int32(high[col+3])
	l0a, l0b, l0c, l0d := int32(low[col]), int32(low[col+1]), int32(low[col+2]), int32(low[col+3])
	x0a, x0b := clampi16(l0a-h0a), clampi16(l0b-h0b)
	x0c, x0d := clampi16(l0c-h0c), clampi16(l0d-h0d)
	x2a, x2b, x2c, x2d := x0a, x0b, x0c, x0d

	for j := 0; j < nHigh-1; j++ {
		h1a, h1b := int32(high[hOff]), int32(high[hOff+1])
		h1c, h1d := int32(high[hOff+2]), int32(high[hOff+3])
		hOff += highStep
		l0a, l0b = int32(low[lOff]), int32(low[lOff+1])
		l0c, l0d = int32(low[lOff+2]), int32(low[lOff+3])
		lOff += lowStep

		x2a = clampi16(l0a - (h0a+h1a)/2)
		x2b = clampi16(l0b - (h0b+h1b)/2)
		x2c = clampi16(l0c - (h0c+h1c)/2)
		x2d = clampi16(l0d - (h0d+h1d)/2)

		dst[xOff] = x0a
		dst[xOff+1] = x0b
		dst[xOff+2] = x0c
		dst[xOff+3] = x0d
		xOff += dstStep
		dst[xOff] = clampi16((int32(x0a)+int32(x2a))/2 + 2*h0a)
		dst[xOff+1] = clampi16((int32(x0b)+int32(x2b))/2 + 2*h0b)
		dst[xOff+2] = clampi16((int32(x0c)+int32(x2c))/2 + 2*h0c)
		dst[xOff+3] = clampi16((int32(x0d)+int32(x2d))/2 + 2*h0d)
		xOff += dstStep

		x0a, x0b, x0c, x0d = x2a, x2b, x2c, x2d
		h0a, h0b, h0c, h0d = h1a, h1b, h1c, h1d
	}

	if nLow <= nHigh+1 {
		if nLow <= nHigh {
			dst[xOff] = x2a
			dst[xOff+1] = x2b
			dst[xOff+2] = x2c
			dst[xOff+3] = x2d
			xOff += dstStep
			dst[xOff] = clampi16(int32(x2a) + 2*h0a)
			dst[xOff+1] = clampi16(int32(x2b) + 2*h0b)
			dst[xOff+2] = clampi16(int32(x2c) + 2*h0c)
			dst[xOff+3] = clampi16(int32(x2d) + 2*h0d)
		} else {
			l0a, l0b = int32(low[lOff]), int32(low[lOff+1])
			l0c, l0d = int32(low[lOff+2]), int32(low[lOff+3])
			x0a = clampi16(l0a - h0a)
			x0b = clampi16(l0b - h0b)
			x0c = clampi16(l0c - h0c)
			x0d = clampi16(l0d - h0d)
			dst[xOff] = x2a
			dst[xOff+1] = x2b
			dst[xOff+2] = x2c
			dst[xOff+3] = x2d
			xOff += dstStep
			dst[xOff] = clampi16((int32(x0a)+int32(x2a))/2 + 2*h0a)
			dst[xOff+1] = clampi16((int32(x0b)+int32(x2b))/2 + 2*h0b)
			dst[xOff+2] = clampi16((int32(x0c)+int32(x2c))/2 + 2*h0c)
			dst[xOff+3] = clampi16((int32(x0d)+int32(x2d))/2 + 2*h0d)
			xOff += dstStep
			dst[xOff] = x0a
			dst[xOff+1] = x0b
			dst[xOff+2] = x0c
			dst[xOff+3] = x0d
		}
	} else {
		l0a, l0b = int32(low[lOff]), int32(low[lOff+1])
		l0c, l0d = int32(low[lOff+2]), int32(low[lOff+3])
		lOff += lowStep
		x0a = clampi16(l0a - h0a/2)
		x0b = clampi16(l0b - h0b/2)
		x0c = clampi16(l0c - h0c/2)
		x0d = clampi16(l0d - h0d/2)
		dst[xOff] = x2a
		dst[xOff+1] = x2b
		dst[xOff+2] = x2c
		dst[xOff+3] = x2d
		xOff += dstStep
		dst[xOff] = clampi16((int32(x0a)+int32(x2a))/2 + 2*h0a)
		dst[xOff+1] = clampi16((int32(x0b)+int32(x2b))/2 + 2*h0b)
		dst[xOff+2] = clampi16((int32(x0c)+int32(x2c))/2 + 2*h0c)
		dst[xOff+3] = clampi16((int32(x0d)+int32(x2d))/2 + 2*h0d)
		xOff += dstStep
		dst[xOff] = x0a
		dst[xOff+1] = x0b
		dst[xOff+2] = x0c
		dst[xOff+3] = x0d
		xOff += dstStep
		l0a, l0b = int32(low[lOff]), int32(low[lOff+1])
		l0c, l0d = int32(low[lOff+2]), int32(low[lOff+3])
		dst[xOff] = clampi16((int32(x0a) + l0a) / 2)
		dst[xOff+1] = clampi16((int32(x0b) + l0b) / 2)
		dst[xOff+2] = clampi16((int32(x0c) + l0c) / 2)
		dst[xOff+3] = clampi16((int32(x0d) + l0d) / 2)
	}
}

// inverseDWTExtAndConvert performs 3-level inverse DWT on Y, Cb, Cr coefficients
// using the extrapolate path, and converts to RGBA in a fused operation.
// For levels 3 and 2, components are processed independently.
// For level 1, the vertical IDWT and YCbCr→RGBA conversion are interleaved
// in 4-column groups to keep data in L1 cache, avoiding a separate color
// conversion pass over the full 64×64 tile.
// tmp must have length >= 3*4096.
func inverseDWTExtAndConvert(yCoeffs, cbCoeffs, crCoeffs, tmp []int16, out []byte) {
	if len(yCoeffs) < tileSize*tileSize || len(cbCoeffs) < tileSize*tileSize ||
		len(crCoeffs) < tileSize*tileSize {
		return
	}

	// Levels 3 and 2: process independently, sharing first 4096 of tmp.
	t := tmp[:4096]
	dwtLevelExt(yCoeffs[3807:], t, 3)
	dwtLevelExt(yCoeffs[3007:], t, 2)
	dwtLevelExt(cbCoeffs[3807:], t, 3)
	dwtLevelExt(cbCoeffs[3007:], t, 2)
	dwtLevelExt(crCoeffs[3807:], t, 3)
	dwtLevelExt(crCoeffs[3007:], t, 2)

	// Level 1: fused horizontal IDWT + vertical IDWT + color conversion.
	nL := extBandL(1) // 33
	nH := extBandH(1) // 31
	nDst := nL + nH   // 64

	// Per-component tmp for horizontal IDWT output (L and H bands).
	yTmp := tmp[0:4096]
	cbTmp := tmp[4096:8192]
	crTmp := tmp[8192:12288]

	// Band offsets within each coefficient buffer for level 1.
	hlOff := 0
	lhOff := nH * nL
	hhOff := lhOff + nL*nH
	llOff := hhOff + nH*nH

	// Horizontal IDWT for all 3 components: bands → L/H in per-component tmp.
	allCoeffs := [3][]int16{yCoeffs, cbCoeffs, crCoeffs}
	allTmp := [3][]int16{yTmp, cbTmp, crTmp}
	for ci := range allCoeffs {
		c := allCoeffs[ci]
		ct := allTmp[ci]
		L := ct[:nL*nDst]
		H := ct[nL*nDst : (nL+nH)*nDst]
		idwtHoriz(c[llOff:], nL, c[hlOff:], nH, L, nDst, nL, nH, nL)
		idwtHoriz(c[lhOff:], nL, c[hhOff:], nH, H, nDst, nL, nH, nH)
	}

	// Vertical IDWT for all 3 components, interleaved in 4-column groups
	// so that all components' data at a given column range stays L1-resident.
	yL, yH := yTmp[:nL*nDst], yTmp[nL*nDst:]
	cbL, cbH := cbTmp[:nL*nDst], cbTmp[nL*nDst:]
	crL, crH := crTmp[:nL*nDst], crTmp[nL*nDst:]

	for col := 0; col+4 <= nDst; col += 4 {
		idwtVertCol4(yL, nDst, yH, nDst, yCoeffs, nDst, nL, nH, col)
		idwtVertCol4(cbL, nDst, cbH, nDst, cbCoeffs, nDst, nL, nH, col)
		idwtVertCol4(crL, nDst, crH, nDst, crCoeffs, nDst, nL, nH, col)
	}
	// nDst=64 is divisible by 4; no tail columns needed.

	// Color convert: all 3 component pixel arrays (24KB total) are now warm
	// in L2 from the interleaved vertical IDWT above.
	ycbcrToRGBA(out, yCoeffs, cbCoeffs, crCoeffs)
}

// idwtVertCol1 processes a single column (tail after 4-wide blocks).
func idwtVertCol1(low []int16, lowStep int, high []int16, highStep int,
	dst []int16, dstStep int, nLow, nHigh, col int) {

	lOff := col + lowStep
	hOff := col + highStep
	xOff := col

	h0 := int32(high[col])
	l0 := int32(low[col])
	x0 := clampi16(l0 - h0)
	x2 := x0

	for j := 0; j < nHigh-1; j++ {
		h1 := int32(high[hOff])
		hOff += highStep
		l0 = int32(low[lOff])
		lOff += lowStep
		x2 = clampi16(l0 - (h0+h1)/2)
		x1 := clampi16((int32(x0)+int32(x2))/2 + 2*h0)
		dst[xOff] = x0
		xOff += dstStep
		dst[xOff] = x1
		xOff += dstStep
		x0 = x2
		h0 = h1
	}

	if nLow <= nHigh+1 {
		if nLow <= nHigh {
			dst[xOff] = x2
			xOff += dstStep
			dst[xOff] = clampi16(int32(x2) + 2*h0)
		} else {
			l0 = int32(low[lOff])
			x0 = clampi16(l0 - h0)
			dst[xOff] = x2
			xOff += dstStep
			dst[xOff] = clampi16((int32(x0)+int32(x2))/2 + 2*h0)
			xOff += dstStep
			dst[xOff] = x0
		}
	} else {
		l0 = int32(low[lOff])
		lOff += lowStep
		x0 = clampi16(l0 - h0/2)
		dst[xOff] = x2
		xOff += dstStep
		dst[xOff] = clampi16((int32(x0)+int32(x2))/2 + 2*h0)
		xOff += dstStep
		dst[xOff] = x0
		xOff += dstStep
		l0 = int32(low[lOff])
		dst[xOff] = clampi16((int32(x0) + l0) / 2)
	}
}
