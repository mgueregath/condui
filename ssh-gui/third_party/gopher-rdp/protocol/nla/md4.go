package nla

import (
	"encoding/binary"
	"math/bits"
)

// md4Sum computes the MD4 hash of data per RFC 1320.
// MD4 is required for NTLM password hashing (NT hash = MD4(UTF16LE(password))).
// Not available in Go's standard library.
func md4Sum(data []byte) [16]byte {
	// Initial hash values
	var a, b, c, d uint32 = 0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476

	// Pre-processing: pad message to 64-byte boundary
	origLen := len(data)
	// Append 0x80
	data = append(data, 0x80)
	// Pad to 56 mod 64
	for len(data)%64 != 56 {
		data = append(data, 0)
	}
	// Append original length in bits as 64-bit little-endian
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(origLen)*8)
	data = append(data, lenBuf[:]...)

	// Process each 64-byte block
	for off := 0; off < len(data); off += 64 {
		block := data[off : off+64]
		var x [16]uint32
		for i := 0; i < 16; i++ {
			x[i] = binary.LittleEndian.Uint32(block[i*4:])
		}

		aa, bb, cc, dd := a, b, c, d

		// Round 1
		f := func(x, y, z uint32) uint32 { return (x & y) | (^x & z) }
		a = bits.RotateLeft32(a+f(b, c, d)+x[0], 3)
		d = bits.RotateLeft32(d+f(a, b, c)+x[1], 7)
		c = bits.RotateLeft32(c+f(d, a, b)+x[2], 11)
		b = bits.RotateLeft32(b+f(c, d, a)+x[3], 19)
		a = bits.RotateLeft32(a+f(b, c, d)+x[4], 3)
		d = bits.RotateLeft32(d+f(a, b, c)+x[5], 7)
		c = bits.RotateLeft32(c+f(d, a, b)+x[6], 11)
		b = bits.RotateLeft32(b+f(c, d, a)+x[7], 19)
		a = bits.RotateLeft32(a+f(b, c, d)+x[8], 3)
		d = bits.RotateLeft32(d+f(a, b, c)+x[9], 7)
		c = bits.RotateLeft32(c+f(d, a, b)+x[10], 11)
		b = bits.RotateLeft32(b+f(c, d, a)+x[11], 19)
		a = bits.RotateLeft32(a+f(b, c, d)+x[12], 3)
		d = bits.RotateLeft32(d+f(a, b, c)+x[13], 7)
		c = bits.RotateLeft32(c+f(d, a, b)+x[14], 11)
		b = bits.RotateLeft32(b+f(c, d, a)+x[15], 19)

		// Round 2
		g := func(x, y, z uint32) uint32 { return (x & y) | (x & z) | (y & z) }
		const k2 = 0x5A827999
		a = bits.RotateLeft32(a+g(b, c, d)+x[0]+k2, 3)
		d = bits.RotateLeft32(d+g(a, b, c)+x[4]+k2, 5)
		c = bits.RotateLeft32(c+g(d, a, b)+x[8]+k2, 9)
		b = bits.RotateLeft32(b+g(c, d, a)+x[12]+k2, 13)
		a = bits.RotateLeft32(a+g(b, c, d)+x[1]+k2, 3)
		d = bits.RotateLeft32(d+g(a, b, c)+x[5]+k2, 5)
		c = bits.RotateLeft32(c+g(d, a, b)+x[9]+k2, 9)
		b = bits.RotateLeft32(b+g(c, d, a)+x[13]+k2, 13)
		a = bits.RotateLeft32(a+g(b, c, d)+x[2]+k2, 3)
		d = bits.RotateLeft32(d+g(a, b, c)+x[6]+k2, 5)
		c = bits.RotateLeft32(c+g(d, a, b)+x[10]+k2, 9)
		b = bits.RotateLeft32(b+g(c, d, a)+x[14]+k2, 13)
		a = bits.RotateLeft32(a+g(b, c, d)+x[3]+k2, 3)
		d = bits.RotateLeft32(d+g(a, b, c)+x[7]+k2, 5)
		c = bits.RotateLeft32(c+g(d, a, b)+x[11]+k2, 9)
		b = bits.RotateLeft32(b+g(c, d, a)+x[15]+k2, 13)

		// Round 3
		h := func(x, y, z uint32) uint32 { return x ^ y ^ z }
		const k3 = 0x6ED9EBA1
		a = bits.RotateLeft32(a+h(b, c, d)+x[0]+k3, 3)
		d = bits.RotateLeft32(d+h(a, b, c)+x[8]+k3, 9)
		c = bits.RotateLeft32(c+h(d, a, b)+x[4]+k3, 11)
		b = bits.RotateLeft32(b+h(c, d, a)+x[12]+k3, 15)
		a = bits.RotateLeft32(a+h(b, c, d)+x[2]+k3, 3)
		d = bits.RotateLeft32(d+h(a, b, c)+x[10]+k3, 9)
		c = bits.RotateLeft32(c+h(d, a, b)+x[6]+k3, 11)
		b = bits.RotateLeft32(b+h(c, d, a)+x[14]+k3, 15)
		a = bits.RotateLeft32(a+h(b, c, d)+x[1]+k3, 3)
		d = bits.RotateLeft32(d+h(a, b, c)+x[9]+k3, 9)
		c = bits.RotateLeft32(c+h(d, a, b)+x[5]+k3, 11)
		b = bits.RotateLeft32(b+h(c, d, a)+x[13]+k3, 15)
		a = bits.RotateLeft32(a+h(b, c, d)+x[3]+k3, 3)
		d = bits.RotateLeft32(d+h(a, b, c)+x[11]+k3, 9)
		c = bits.RotateLeft32(c+h(d, a, b)+x[7]+k3, 11)
		b = bits.RotateLeft32(b+h(c, d, a)+x[15]+k3, 15)

		a += aa
		b += bb
		c += cc
		d += dd
	}

	var digest [16]byte
	binary.LittleEndian.PutUint32(digest[0:], a)
	binary.LittleEndian.PutUint32(digest[4:], b)
	binary.LittleEndian.PutUint32(digest[8:], c)
	binary.LittleEndian.PutUint32(digest[12:], d)
	return digest
}
