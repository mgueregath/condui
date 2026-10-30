package sec

import "math/big"

// RSAEncrypt performs textbook RSA encryption (no padding): c = m^e mod n.
// Both plaintext and the returned ciphertext are in little-endian byte order,
// matching the RDP wire format. The result is sized to key.BitLen/8 bytes
// plus 8 zero bytes per MS-RDPBCGR spec.
func RSAEncrypt(plaintext []byte, key *RSAPublicKey) []byte {
	// Strip the 8-byte zero padding from modulus to get the actual modulus.
	modLen := len(key.Modulus)
	if modLen > 8 {
		modLen -= 8
	}

	// Reverse plaintext and modulus from little-endian to big-endian for math/big.
	m := new(big.Int).SetBytes(reverseBytes(plaintext))
	n := new(big.Int).SetBytes(reverseBytes(key.Modulus[:modLen]))
	e := big.NewInt(int64(key.PubExp))

	// c = m^e mod n
	c := new(big.Int).Exp(m, e, n)

	// Convert result back to little-endian, padded to modLen bytes.
	cBytes := c.Bytes() // big-endian
	result := reverseBytes(cBytes)

	// Pad or trim to modLen
	outLen := int(key.BitLen / 8)
	if len(result) < outLen {
		padded := make([]byte, outLen)
		copy(padded, result)
		result = padded
	} else if len(result) > outLen {
		result = result[:outLen]
	}

	// Append 8 zero bytes per spec.
	result = append(result, make([]byte, 8)...)
	return result
}

// reverseBytes returns a new slice with bytes in reverse order.
func reverseBytes(b []byte) []byte {
	n := len(b)
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = b[n-1-i]
	}
	return out
}
