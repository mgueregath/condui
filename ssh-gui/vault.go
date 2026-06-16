package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/account"
)

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
