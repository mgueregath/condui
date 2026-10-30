package lic

import (
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
)

// LicenseCrypto holds derived licensing encryption keys and provides
// MAC computation and RC4 encryption per MS-RDPELE 5.1.3.
type LicenseCrypto struct {
	MACSaltKey          []byte // 16 bytes
	LicensingEncryptKey []byte // 16 bytes
}

// DeriveLicenseKeys derives the licensing MAC salt key and encryption key
// from preMasterSecret, clientRandom, and serverRandom per MS-RDPELE 5.1.3.
//
// MasterSecret = SaltedHash("A") + SaltedHash("BB") + SaltedHash("CCC")
//
//	where SaltedHash(I) = MD5(PM + SHA1(I + PM + CR + SR))
//
// SessionKeyBlob = SaltedHash("A") + SaltedHash("BB") + SaltedHash("CCC")
//
//	where SaltedHash(I) = MD5(MS + SHA1(I + MS + SR + CR))  ← note reversed order
//
// MACSaltKey = SessionKeyBlob[0:16]
// LicensingEncryptionKey = FinalHash(SessionKeyBlob[16:32])
//
//	where FinalHash(K) = MD5(K + CR + SR)
func DeriveLicenseKeys(preMasterSecret, clientRandom, serverRandom []byte) *LicenseCrypto {
	// MasterSecret: uses clientRandom, serverRandom order
	masterSecret := make([]byte, 0, 48)
	masterSecret = append(masterSecret, saltedHash(preMasterSecret, []byte("A"), clientRandom, serverRandom)...)
	masterSecret = append(masterSecret, saltedHash(preMasterSecret, []byte("BB"), clientRandom, serverRandom)...)
	masterSecret = append(masterSecret, saltedHash(preMasterSecret, []byte("CCC"), clientRandom, serverRandom)...)

	// SessionKeyBlob: uses serverRandom, clientRandom order (reversed!)
	sessionKeyBlob := make([]byte, 0, 48)
	sessionKeyBlob = append(sessionKeyBlob, saltedHash(masterSecret, []byte("A"), serverRandom, clientRandom)...)
	sessionKeyBlob = append(sessionKeyBlob, saltedHash(masterSecret, []byte("BB"), serverRandom, clientRandom)...)
	sessionKeyBlob = append(sessionKeyBlob, saltedHash(masterSecret, []byte("CCC"), serverRandom, clientRandom)...)

	macSaltKey := make([]byte, 16)
	copy(macSaltKey, sessionKeyBlob[0:16])

	encKey := finalHash(sessionKeyBlob[16:32], clientRandom, serverRandom)

	return &LicenseCrypto{
		MACSaltKey:          macSaltKey,
		LicensingEncryptKey: encKey,
	}
}

// saltedHash computes SaltedHash(I) = MD5(secret + SHA1(I + secret + rand1 + rand2))
func saltedHash(secret, iter, rand1, rand2 []byte) []byte {
	sha := sha1.New()
	sha.Write(iter)
	sha.Write(secret)
	sha.Write(rand1)
	sha.Write(rand2)
	shaHash := sha.Sum(nil)

	md := md5.New()
	md.Write(secret)
	md.Write(shaHash)
	return md.Sum(nil)
}

// finalHash computes FinalHash(K) = MD5(K + rand1 + rand2)
func finalHash(key, rand1, rand2 []byte) []byte {
	md := md5.New()
	md.Write(key)
	md.Write(rand1)
	md.Write(rand2)
	return md.Sum(nil)
}

// MAC computes the licensing MAC per MS-RDPELE 5.1.3:
//
//	SHAResult = SHA1(MACSaltKey + Pad1 + LE32(len(data)) + data)
//	MAC = MD5(MACSaltKey + Pad2 + SHAResult)
//
// Returns 16-byte MAC.
func (lc *LicenseCrypto) MAC(data []byte) []byte {
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(data)))

	sha := sha1.New()
	sha.Write(lc.MACSaltKey)
	sha.Write(pad1[:])
	sha.Write(lenBuf[:])
	sha.Write(data)
	shaResult := sha.Sum(nil)

	md := md5.New()
	md.Write(lc.MACSaltKey)
	md.Write(pad2[:])
	md.Write(shaResult)
	return md.Sum(nil)
}

// Encrypt performs RC4 encryption with a fresh cipher per operation.
func (lc *LicenseCrypto) Encrypt(plaintext []byte) []byte {
	cipher, _ := rc4.NewCipher(lc.LicensingEncryptKey)
	out := make([]byte, len(plaintext))
	cipher.XORKeyStream(out, plaintext)
	return out
}

// Decrypt performs RC4 decryption with a fresh cipher per operation.
// RC4 is symmetric, so this is the same as Encrypt.
func (lc *LicenseCrypto) Decrypt(ciphertext []byte) []byte {
	return lc.Encrypt(ciphertext)
}

// Pads for MAC computation (same as MS-RDPBCGR 5.3.6.1).
var (
	pad1 [40]byte
	pad2 [48]byte
)

func init() {
	for i := range pad1 {
		pad1[i] = 0x36
	}
	for i := range pad2 {
		pad2[i] = 0x5C
	}
}
