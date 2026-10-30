package sec

import (
	"context"
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"hash"
	"log/slog"
	"sync"
)

// Encryption method constants (from ServerSecurityData.EncryptionMethod).
const (
	Method40Bit  uint32 = 0x00000001
	Method56Bit  uint32 = 0x00000008
	Method128Bit uint32 = 0x00000002
	MethodFIPS   uint32 = 0x00000010
)

// keyUpdateThreshold is the number of encrypt/decrypt operations before
// a key update is triggered.
const keyUpdateThreshold = 4096

// RDPCrypto handles Standard RDP Security encryption and decryption.
type RDPCrypto struct {
	macKey     []byte
	encryptKey []byte
	decryptKey []byte

	// Initial keys saved for key update operations.
	initialEncKey []byte
	initialDecKey []byte

	encryptCipher *rc4.Cipher
	decryptCipher *rc4.Cipher

	// Reusable hash objects for generateMAC (avoids allocs per call).
	shaHash hash.Hash
	md5Hash hash.Hash

	encryptCount uint32
	decryptCount uint32

	method uint32

	log *slog.Logger

	encMu sync.Mutex
	decMu sync.Mutex
}

// NewRDPCrypto derives session keys from the client and server randoms and
// creates a new RDPCrypto ready for encrypt/decrypt operations.
func NewRDPCrypto(clientRandom, serverRandom []byte, method uint32, log *slog.Logger) (*RDPCrypto, error) {
	macKey, encKey, decKey := deriveSessionKeys(clientRandom, serverRandom, method)

	encCipher, err := rc4.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("creating encrypt cipher: %w", err)
	}
	decCipher, err := rc4.NewCipher(decKey)
	if err != nil {
		return nil, fmt.Errorf("creating decrypt cipher: %w", err)
	}

	initialEnc := make([]byte, len(encKey))
	copy(initialEnc, encKey)
	initialDec := make([]byte, len(decKey))
	copy(initialDec, decKey)

	return &RDPCrypto{
		macKey:        macKey,
		encryptKey:    encKey,
		decryptKey:    decKey,
		initialEncKey: initialEnc,
		initialDecKey: initialDec,
		encryptCipher: encCipher,
		decryptCipher: decCipher,
		shaHash:       sha1.New(),
		md5Hash:       md5.New(),
		log:           log,
		method:        method,
	}, nil
}

// Encrypt encrypts plaintext and prepends a security header (4 bytes) + MAC (8 bytes).
// flags are OR'd into the security header flags field.
// Single allocation: secHeader(4) + MAC(8) + ciphertext in one buffer.
func (rc *RDPCrypto) Encrypt(plaintext []byte, flags uint16) []byte {
	rc.encMu.Lock()
	defer rc.encMu.Unlock()

	if rc.encryptCount == keyUpdateThreshold {
		rc.updateEncryptKey()
	}

	// Single alloc: header + MAC + ciphertext
	out := make([]byte, 4+8+len(plaintext))
	binary.LittleEndian.PutUint16(out[0:2], flags)
	binary.LittleEndian.PutUint16(out[2:4], 0) // flagsHi
	rc.generateMAC(out[4:12], plaintext)
	rc.encryptCipher.XORKeyStream(out[12:], plaintext)
	rc.encryptCount++
	rc.log.LogAttrs(context.Background(), slog.LevelDebug, "sec encrypt", slog.Int("plainLen", len(plaintext)), slog.Int("count", int(rc.encryptCount)))

	return out
}

// EncryptInPlace encrypts plaintext in-place within buf and writes the security
// header + MAC before it. buf layout: [secHeader(4) + MAC(8) + plaintext...].
// The plaintext region (buf[12:]) is encrypted in-place. Returns buf unchanged.
// Caller must ensure buf has the correct layout and length.
func (rc *RDPCrypto) EncryptInPlace(buf []byte, flags uint16) {
	rc.encMu.Lock()
	defer rc.encMu.Unlock()

	if rc.encryptCount == keyUpdateThreshold {
		rc.updateEncryptKey()
	}

	plaintext := buf[12:]

	binary.LittleEndian.PutUint16(buf[0:2], flags)
	binary.LittleEndian.PutUint16(buf[2:4], 0) // flagsHi
	rc.generateMAC(buf[4:12], plaintext)

	rc.encryptCipher.XORKeyStream(plaintext, plaintext)
	rc.encryptCount++
	rc.log.LogAttrs(context.Background(), slog.LevelDebug, "sec encrypt", slog.Int("plainLen", len(plaintext)), slog.Int("count", int(rc.encryptCount)))
}

// Decrypt strips the MAC (8 bytes) and decrypts the ciphertext in-place.
// data should start at the MAC (security header already stripped).
// Returns a sub-slice of data containing the decrypted plaintext.
// Safe because all callers process data synchronously before the next read.
func (rc *RDPCrypto) Decrypt(data []byte) ([]byte, error) {
	rc.decMu.Lock()
	defer rc.decMu.Unlock()

	if len(data) < 8 {
		return nil, fmt.Errorf("encrypted data too short: %d bytes", len(data))
	}

	if rc.decryptCount == keyUpdateThreshold {
		rc.updateDecryptKey()
	}

	// mac := data[:8]
	ciphertext := data[8:]

	rc.decryptCipher.XORKeyStream(ciphertext, ciphertext)
	rc.decryptCount++
	rc.log.LogAttrs(context.Background(), slog.LevelDebug, "sec decrypt", slog.Int("cipherLen", len(ciphertext)), slog.Int("count", int(rc.decryptCount)))

	// MAC validation is intentionally skipped — many RDP server implementations
	// produce non-conforming MACs and existing clients also skip verification.

	return ciphertext, nil
}

// DecryptFastPath decrypts a fast-path PDU. data = MAC(8) + ciphertext.
// Returns the decrypted payload.
func (rc *RDPCrypto) DecryptFastPath(data []byte) ([]byte, error) {
	return rc.Decrypt(data)
}

// generateMAC computes the non-FIPS MAC per MS-RDPBCGR 5.3.6.1 and writes
// the 8-byte result into dst. dst must be at least 8 bytes.
//
//	Pad1 = 0x36 repeated 40 times
//	Pad2 = 0x5C repeated 48 times
//	SHA = SHA1(MACKey + Pad1 + LE_u32(len) + data)
//	MAC = MD5(MACKey + Pad2 + SHA)[:8]
//
// Uses reusable hash objects and stack-allocated scratch buffers (zero allocs).
func (rc *RDPCrypto) generateMAC(dst []byte, data []byte) {
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(data)))

	// SHA1 pass (reuse hash, reset before use)
	rc.shaHash.Reset()
	rc.shaHash.Write(rc.macKey)
	rc.shaHash.Write(pad1[:])
	rc.shaHash.Write(lenBuf[:])
	rc.shaHash.Write(data)
	var shaBuf [20]byte
	shaResult := rc.shaHash.Sum(shaBuf[:0])

	// MD5 pass (reuse hash, reset before use)
	rc.md5Hash.Reset()
	rc.md5Hash.Write(rc.macKey)
	rc.md5Hash.Write(pad2[:])
	rc.md5Hash.Write(shaResult)
	var mdBuf [16]byte
	mdResult := rc.md5Hash.Sum(mdBuf[:0])
	copy(dst, mdResult[:8])
}

// Pads for MAC computation.
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

// deriveSessionKeys implements the full MS-RDPBCGR 5.3.5 key derivation chain.
func deriveSessionKeys(clientRandom, serverRandom []byte, method uint32) (macKey, encKey, decKey []byte) {
	// 1. PreMasterSecret = ClientRandom[:24] + ServerRandom[:24]
	preMaster := make([]byte, 48)
	copy(preMaster[:24], clientRandom[:24])
	copy(preMaster[24:], serverRandom[:24])

	// 2. MasterSecret = SaltedHash(PM, "A") + SaltedHash(PM, "BB") + SaltedHash(PM, "CCC")
	masterSecret := make([]byte, 0, 48)
	masterSecret = append(masterSecret, saltedHash(preMaster, []byte("A"), clientRandom, serverRandom)...)
	masterSecret = append(masterSecret, saltedHash(preMaster, []byte("BB"), clientRandom, serverRandom)...)
	masterSecret = append(masterSecret, saltedHash(preMaster, []byte("CCC"), clientRandom, serverRandom)...)

	// 3. SessionKeyBlob = SaltedHash(MS, "X") + SaltedHash(MS, "YY") + SaltedHash(MS, "ZZZ")
	sessionKeyBlob := make([]byte, 0, 48)
	sessionKeyBlob = append(sessionKeyBlob, saltedHash(masterSecret, []byte("X"), clientRandom, serverRandom)...)
	sessionKeyBlob = append(sessionKeyBlob, saltedHash(masterSecret, []byte("YY"), clientRandom, serverRandom)...)
	sessionKeyBlob = append(sessionKeyBlob, saltedHash(masterSecret, []byte("ZZZ"), clientRandom, serverRandom)...)

	// 4. Extract keys from session key blob
	macKey = sessionKeyBlob[0:16]

	// Client encrypt key = FinalHash(SKB[32:48])
	// Client decrypt key = FinalHash(SKB[16:32])
	encKey = finalHash(sessionKeyBlob[32:48], clientRandom, serverRandom)
	decKey = finalHash(sessionKeyBlob[16:32], clientRandom, serverRandom)

	// 5. Apply salt reduction for 40/56-bit methods
	switch method {
	case Method40Bit:
		macKey = reduce40(macKey)
		encKey = reduce40(encKey)
		decKey = reduce40(decKey)
	case Method56Bit:
		macKey = reduce56(macKey)
		encKey = reduce56(encKey)
		decKey = reduce56(decKey)
	}

	return macKey, encKey, decKey
}

// saltedHash computes SaltedHash(S, I) = MD5(S + SHA1(I + S + CR + SR))
func saltedHash(secret, iter, clientRandom, serverRandom []byte) []byte {
	sha := sha1.New()
	sha.Write(iter)
	sha.Write(secret)
	sha.Write(clientRandom)
	sha.Write(serverRandom)
	shaHash := sha.Sum(nil)

	md := md5.New()
	md.Write(secret)
	md.Write(shaHash)
	return md.Sum(nil)
}

// finalHash computes FinalHash(K) = MD5(K + CR + SR)
func finalHash(key, clientRandom, serverRandom []byte) []byte {
	md := md5.New()
	md.Write(key)
	md.Write(clientRandom)
	md.Write(serverRandom)
	return md.Sum(nil)
}

// reduce40 applies 40-bit salt reduction: set first 3 bytes to 0xD1, 0x26, 0x9E
// and truncate to 8 bytes.
func reduce40(key []byte) []byte {
	out := make([]byte, 8)
	copy(out, key[:8])
	out[0] = 0xD1
	out[1] = 0x26
	out[2] = 0x9E
	return out
}

// reduce56 applies 56-bit salt reduction: set first byte to 0xD1
// and truncate to 8 bytes.
func reduce56(key []byte) []byte {
	out := make([]byte, 8)
	copy(out, key[:8])
	out[0] = 0xD1
	return out
}

// updateEncryptKey performs key update per MS-RDPBCGR 5.3.6.2.
func (rc *RDPCrypto) updateEncryptKey() {
	rc.encryptKey = updateKey(rc.initialEncKey, rc.encryptKey, rc.method)
	rc.encryptCipher, _ = rc4.NewCipher(rc.encryptKey)
	rc.encryptCount = 0
	rc.log.LogAttrs(context.Background(), slog.LevelDebug, "sec encrypt key update")
}

// updateDecryptKey performs key update per MS-RDPBCGR 5.3.6.2.
func (rc *RDPCrypto) updateDecryptKey() {
	rc.decryptKey = updateKey(rc.initialDecKey, rc.decryptKey, rc.method)
	rc.decryptCipher, _ = rc4.NewCipher(rc.decryptKey)
	rc.decryptCount = 0
	rc.log.LogAttrs(context.Background(), slog.LevelDebug, "sec decrypt key update")
}

// updateKey implements the key update algorithm:
//
//	SHA = SHA1(InitKey + Pad1 + CurKey)
//	TempKey = MD5(InitKey + Pad2 + SHA)
//	NewKey = RC4(TempKey[:keyLen], TempKey[:keyLen])
//	Apply salt reduction if needed.
func updateKey(initialKey, currentKey []byte, method uint32) []byte {
	sha := sha1.New()
	sha.Write(initialKey)
	sha.Write(pad1[:])
	sha.Write(currentKey)
	shaHash := sha.Sum(nil)

	md := md5.New()
	md.Write(initialKey)
	md.Write(pad2[:])
	md.Write(shaHash)
	tempKey := md.Sum(nil)

	keyLen := len(currentKey)
	cipher, _ := rc4.NewCipher(tempKey[:keyLen])
	newKey := make([]byte, keyLen)
	cipher.XORKeyStream(newKey, tempKey[:keyLen])

	switch method {
	case Method40Bit:
		newKey = reduce40(newKey)
	case Method56Bit:
		newKey = reduce56(newKey)
	}

	return newKey
}
