package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/account"
)

const syncVaultSaltInput = "condui-sync-vault-v1"

// deriveSyncVaultKey returns a stable key derived only from the vault password
// (no device-specific salt), making it reproducible on any device with the same password.
func deriveSyncVaultKey(vaultPassword string) []byte {
	h := sha256.Sum256([]byte(syncVaultSaltInput))
	return account.DeriveKey(vaultPassword, h[:])
}

// encryptForSync encrypts a plaintext password with the syncVaultKey and returns
// it in the format "sync:nonce_b64:ct_b64".
func encryptForSync(plaintext string, syncKey []byte) (string, error) {
	enc, err := account.EncryptField(syncKey, plaintext)
	if err != nil {
		return "", err
	}
	return "sync:" + enc, nil
}

// decryptFromSync decrypts a "sync:nonce_b64:ct_b64" value.
func decryptFromSync(encoded string, syncKey []byte) (string, error) {
	inner := strings.TrimPrefix(encoded, "sync:")
	return account.DecryptField(syncKey, inner)
}

// isSyncEncrypted reports whether the value was encrypted with the sync vault key.
func isSyncEncrypted(s string) bool {
	return strings.HasPrefix(s, "sync:")
}

// encryptConnectionPassword encrypts a plaintext password using the vault master key.
// Returns the encrypted string, or the original if it's empty/nil.
func encryptConnectionPassword(plaintext string, key []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return account.EncryptField(key, plaintext)
}

// decryptConnectionPassword decrypts an encrypted password using the vault master key.
// If the value doesn't look encrypted (no ":" separator), returns it as-is for
// backward-compatibility with connections saved before encryption was added.
func decryptConnectionPassword(ciphertext string, key []byte) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	// Detect if value is already encrypted (contains base64:base64 pattern)
	for _, c := range ciphertext {
		if c == ':' {
			return account.DecryptField(key, ciphertext)
		}
	}
	// Plaintext (legacy) — return as-is
	return ciphertext, nil
}

// loadPrivateKey reads and parses an SSH private key from the given path.
func loadPrivateKey(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read private key %s: %w", path, err)
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse private key %s: %w", path, err)
	}
	return signer, nil
}
