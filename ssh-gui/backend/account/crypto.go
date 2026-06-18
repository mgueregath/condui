package account

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Memory      = 64 * 1024
	argon2Iterations  = 3
	argon2Parallelism = 4
	argon2KeyLen      = 32
	saltLen           = 16

	vaultTestPlaintext = "condui-vault-ok"
)

// DeriveSyncKey derives a stable per-account key from the account password.
// Unlike the local vault key, this key is reproducible on every device.
func DeriveSyncKey(email, password string) []byte {
	salt := sha256.Sum256([]byte("condui-sync-v1:" + email))
	return argon2.IDKey(
		[]byte(password), salt[:],
		argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLen,
	)
}

// GenerateRandomKey returns a random 32-byte key for independent blob sharing.
func GenerateRandomKey() ([]byte, error) {
	key := make([]byte, argon2KeyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// GenerateSalt returns 16 cryptographically random bytes.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// DeriveKey derives a 32-byte AES key from password + salt using Argon2id.
func DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(password), salt,
		argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLen,
	)
}

// EncryptField encrypts plaintext with AES-256-GCM and returns a base64 string
// in the format nonce:ciphertext (both base64, separated by ":").
func EncryptField(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(nonce) + ":" + base64.StdEncoding.EncodeToString(ciphertext)
	return encoded, nil
}

// DecryptField decrypts a value produced by EncryptField.
func DecryptField(key []byte, encoded string) (string, error) {
	// Find separator
	for i, c := range encoded {
		if c == ':' {
			nonceB64 := encoded[:i]
			ctB64 := encoded[i+1:]
			nonce, err := base64.StdEncoding.DecodeString(nonceB64)
			if err != nil {
				return "", fmt.Errorf("invalid nonce encoding: %w", err)
			}
			ciphertext, err := base64.StdEncoding.DecodeString(ctB64)
			if err != nil {
				return "", fmt.Errorf("invalid ciphertext encoding: %w", err)
			}
			block, err := aes.NewCipher(key)
			if err != nil {
				return "", err
			}
			gcm, err := cipher.NewGCM(block)
			if err != nil {
				return "", err
			}
			plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
			if err != nil {
				return "", errors.New("decryption failed: wrong password or corrupted data")
			}
			return string(plaintext), nil
		}
	}
	return "", errors.New("invalid encrypted field format")
}

// MakeVaultTest encrypts the test string to verify password correctness.
func MakeVaultTest(key []byte) (string, error) {
	return EncryptField(key, vaultTestPlaintext)
}

// VerifyVaultTest decrypts the test cipher and checks it matches the expected value.
func VerifyVaultTest(key []byte, testCipher string) bool {
	plaintext, err := DecryptField(key, testCipher)
	if err != nil {
		return false
	}
	return plaintext == vaultTestPlaintext
}

// EncryptBlob encrypts arbitrary bytes for sync, returns (ciphertext, nonce, checksum) all base64.
func EncryptBlob(masterKey, plaintext []byte) (cipherB64, nonceB64, checksumB64 string, err error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	sum := sha256.Sum256(ciphertext)
	cipherB64 = base64.StdEncoding.EncodeToString(ciphertext)
	nonceB64 = base64.StdEncoding.EncodeToString(nonce)
	checksumB64 = base64.StdEncoding.EncodeToString(sum[:])
	return
}

// DecryptBlob decrypts a blob produced by EncryptBlob.
func DecryptBlob(masterKey []byte, cipherB64, nonceB64 string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// GenerateX25519Keypair generates a keypair for E2E sharing.
func GenerateX25519Keypair() (pubB64, privB64 string, err error) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return
	}
	pubB64 = base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes())
	privB64 = base64.StdEncoding.EncodeToString(priv.Bytes())
	return
}

// WrapKey encrypts a symmetric key using ECDH with recipient's X25519 public key.
// Returns base64-encoded encrypted key.
func WrapKey(recipientPubB64 string, symmetricKey []byte) (string, error) {
	recipientPubBytes, err := base64.StdEncoding.DecodeString(recipientPubB64)
	if err != nil {
		return "", err
	}
	curve := ecdh.X25519()
	recipientPub, err := curve.NewPublicKey(recipientPubBytes)
	if err != nil {
		return "", err
	}
	// Generate ephemeral keypair
	ephemeral, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	shared, err := ephemeral.ECDH(recipientPub)
	if err != nil {
		return "", err
	}
	// Derive AES key from shared secret
	hash := sha256.Sum256(shared)
	encryptedKey, err := EncryptField(hash[:], base64.StdEncoding.EncodeToString(symmetricKey))
	if err != nil {
		return "", err
	}
	// Prepend ephemeral public key so recipient can derive same shared secret
	ephPubB64 := base64.StdEncoding.EncodeToString(ephemeral.PublicKey().Bytes())
	return ephPubB64 + "|" + encryptedKey, nil
}

// UnwrapKey decrypts a wrapped key using the recipient's X25519 private key.
func UnwrapKey(myPrivB64, wrappedKey string) ([]byte, error) {
	// Split ephemeral pub key from encrypted payload
	for i, c := range wrappedKey {
		if c == '|' {
			ephPubB64 := wrappedKey[:i]
			encryptedKey := wrappedKey[i+1:]

			ephPubBytes, err := base64.StdEncoding.DecodeString(ephPubB64)
			if err != nil {
				return nil, err
			}
			myPrivBytes, err := base64.StdEncoding.DecodeString(myPrivB64)
			if err != nil {
				return nil, err
			}
			curve := ecdh.X25519()
			ephPub, err := curve.NewPublicKey(ephPubBytes)
			if err != nil {
				return nil, err
			}
			myPriv, err := curve.NewPrivateKey(myPrivBytes)
			if err != nil {
				return nil, err
			}
			shared, err := myPriv.ECDH(ephPub)
			if err != nil {
				return nil, err
			}
			hash := sha256.Sum256(shared)
			keyB64, err := DecryptField(hash[:], encryptedKey)
			if err != nil {
				return nil, err
			}
			return base64.StdEncoding.DecodeString(keyB64)
		}
	}
	return nil, errors.New("invalid wrapped key format")
}
