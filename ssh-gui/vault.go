package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
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

// expandUserPath expands a leading "~" or "~/" to the current user's home
// directory, since Go's os package does not do this the way a shell would.
func expandUserPath(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}

// loadPrivateKey reads and parses an SSH private key from the given path.
// If the key is encrypted and passphrase is non-empty, it's used to decrypt it.
func loadPrivateKey(path, passphrase string) (ssh.Signer, error) {
	originalPath := path
	path = expandUserPath(path)

	// Log the path for debugging
	fmt.Printf("[DEBUG] Loading private key from: %s (original: %s)\n", path, originalPath)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read private key %s: %w", path, err)
	}

	fmt.Printf("[DEBUG] Read %d bytes from key file\n", len(data))

	// Try parsing without passphrase first
	signer, err := ssh.ParsePrivateKey(data)
	if err == nil {
		fmt.Printf("[DEBUG] Successfully parsed private key without passphrase\n")
		return signer, nil
	}

	fmt.Printf("[DEBUG] Failed to parse without passphrase: %v\n", err)

	// If we have a passphrase, try with it
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("cannot parse private key %s with passphrase: %w", path, err)
		}
		fmt.Printf("[DEBUG] Successfully parsed private key with passphrase\n")
		return signer, nil
	}

	// If the error suggests the key needs a passphrase, return a more helpful error
	errMsg := err.Error()
	if strings.Contains(errMsg, "encrypted") || strings.Contains(errMsg, "passphrase") {
		return nil, fmt.Errorf("private key %s is encrypted but no passphrase was provided", path)
	}

	return nil, fmt.Errorf("cannot parse private key %s: %w (this may be due to an unsupported key format or missing passphrase)", path, err)
}
