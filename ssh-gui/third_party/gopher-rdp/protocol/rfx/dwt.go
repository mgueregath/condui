package rfx

// Inverse 2D Discrete Wavelet Transform for RemoteFX tiles (64x64 pixels, 3 levels).
//
// Coefficient buffer layout (MS-RDPRFX convention):
//   [HL1(1024), LH1(1024), HH1(1024), HL2(256), LH2(256), HH2(256), HL3(64), LH3(64), HH3(64), LL3(64)]
//   Total = 3*1024 + 3*256 + 4*64 = 3072 + 768 + 256 = 4096
//
// Lifting steps (MS-RDPRFX 3.1.8.1.5):
//   even[n] = low[n] - ((high[n-1] + high[n] + 1) >> 1)
//   odd[n]  = (high[n] << 1) + ((even[n] + even[n+1]) >> 1)

const tileSize = 64

// Subband offsets within the 4096-element coefficient buffer.
const (
	offHL1 = 0
	offLH1 = 1024
	offHH1 = 2048
	offHL2 = 3072
	offLH2 = 3328
	offHH2 = 3584
	offHL3 = 3840
	offLH3 = 3904
	offHH3 = 3968
	offLL3 = 4032
)

// inverseDWT performs 3-level 2D inverse DWT on 4096 coefficients.
// tmp is a scratch buffer of at least 4096 elements.
// colL, colH must have len >= 32; colOut must have len >= 64.
// Modifies the buffer in-place; the 64x64 result is at buffer[0:4096].
func inverseDWT(coeffs, tmp []int16) []int16 {
	if len(coeffs) < tileSize*tileSize {
		return coeffs
	}

	// Level 3: 4 × 8x8 subbands → 16x16 output at offset 3840
	dwtLevel(coeffs, tmp, 3840, 8)
	// Level 2: 4 × 16x16 subbands → 32x32 output at offset 3072
	dwtLevel(coeffs, tmp, 3072, 16)
	// Level 1: 4 × 32x32 subbands → 64x64 output at offset 0
	dwtLevel(coeffs, tmp, 0, 32)

	return coeffs
}

// dwtLevel performs one level of 2D inverse DWT.
// Layout at offset: HL(n*n) + LH(n*n) + HH(n*n) + LL(n*n)
// Output: 2n*2n values written at offset.
func dwtLevel(buf, tmp []int16, offset, n int) {
	n2 := n * 2
	nn := n * n

	hl := buf[offset : offset+nn]
	lh := buf[offset+nn : offset+2*nn]
	hh := buf[offset+2*nn : offset+3*nn]
	ll := buf[offset+3*nn : offset+4*nn]

	// Step 1: Inverse horizontal DWT
	// Low rows: combine LL (low) + HL (high) → tmp rows 0..n-1 (width 2n)
	// High rows: combine LH (low) + HH (high) → tmp rows n..2n-1 (width 2n)
	for y := 0; y < n; y++ {
		inverseDWT1D(tmp[y*n2:(y+1)*n2], ll[y*n:(y+1)*n], hl[y*n:(y+1)*n], n)
		inverseDWT1D(tmp[(n+y)*n2:(n+y+1)*n2], lh[y*n:(y+1)*n], hh[y*n:(y+1)*n], n)
	}

	// Step 2: Inverse vertical DWT for each column.
	// Stack arrays avoid heap-allocated colL/colH/colOut slices (max n=32).
	var colL, colH [32]int16
	var colOut [64]int16
	for x := 0; x < n2; x++ {
		for y := 0; y < n; y++ {
			colL[y] = tmp[y*n2+x]
			colH[y] = tmp[(n+y)*n2+x]
		}
		inverseDWT1D(colOut[:], colL[:], colH[:], n)
		for y := 0; y < n2; y++ {
			buf[offset+y*n2+x] = colOut[y]
		}
	}
}

// inverseDWT1D performs 1D inverse DWT (MS-RDPRFX 3.1.8.1.5):
//   even[n] = low[n] - ((high[n-1] + high[n] + 1) >> 1)
//   odd[n]  = (high[n] << 1) + ((even[n] + even[n+1]) >> 1)
func inverseDWT1D(out []int16, low, high []int16, n int) {
	if n == 0 {
		return
	}

	// BCE hints: prove all indices are in-bounds.
	_ = low[n-1]
	_ = high[n-1]
	_ = out[2*n-1]

	// Reconstruct even samples (update step inverse).
	// Peel first iteration: hLeft = high[0] (symmetric extension).
	out[0] = low[0] - ((high[0] + high[0] + 1) >> 1)
	// Middle iterations: no boundary checks needed.
	for i := 1; i < n; i++ {
		out[2*i] = low[i] - ((high[i-1] + high[i] + 1) >> 1)
	}

	// Reconstruct odd samples (predict step inverse).
	// Middle iterations: eRight = out[2*i+2].
	for i := 0; i < n-1; i++ {
		out[2*i+1] = (high[i] << 1) + ((out[2*i] + out[2*i+2]) >> 1)
	}
	// Peel last iteration: eRight = eLeft (symmetric extension).
	out[2*n-1] = (high[n-1] << 1) + out[2*(n-1)]
}
