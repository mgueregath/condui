package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/account"
)

// connCipherPrefix marks values produced by encryptConnectionPassword so that
// decryptConnectionPassword/isConnectionEncrypted can tell them apart from
// plaintext with certainty, instead of guessing from a ":" separator that a
// plaintext password may also happen to contain (see looksLikeEncryptedField).
const connCipherPrefix = "enc:"

// aesGCMNonceSize is the nonce length account.EncryptField always produces
// (AES-GCM standard), used to validate the legacy (unprefixed) format below.
const aesGCMNonceSize = 12

// looksLikeEncryptedField reports whether s matches the exact
// "nonce_b64:ciphertext_b64" shape produced by account.EncryptField, used to
// recognize connections encrypted before connCipherPrefix was introduced.
// Checking for the mere presence of a ":" is not enough: a user-chosen
// plaintext password containing a colon (e.g. "pass:word123") would falsely
// match and get stored unencrypted, later failing to decrypt as "vault
// error" that changing the password again could never clear.
func looksLikeEncryptedField(s string) bool {
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return false
	}
	nonce, err := base64.StdEncoding.DecodeString(s[:idx])
	if err != nil || len(nonce) != aesGCMNonceSize {
		return false
	}
	if _, err := base64.StdEncoding.DecodeString(s[idx+1:]); err != nil {
		return false
	}
	return true
}

// isConnectionEncrypted reports whether a connection secret is already in an
// encrypted form (either the current prefixed format or the legacy one).
func isConnectionEncrypted(s string) bool {
	return strings.HasPrefix(s, connCipherPrefix) || looksLikeEncryptedField(s)
}

const syncVaultSaltInput = "condui-sync-vault-v1"

// deriveSyncVaultKey returns a stable key derived only from the local vault
// password (no device-specific salt). It is kept as a legacy fallback: two
// devices only derive the same key from it if they happen to share the exact
// same LOCAL vault password, which is not guaranteed since each device's
// vault password is independent by design. New writes prefer the
// account-wide key (account.DeriveSyncKey, see getAccountSyncKey in app.go),
// which is reproducible on any device logged into the same account
// regardless of its local vault password; this one is only tried as a
// fallback when decrypting older sync-format secrets, or for the manual
// "unlock connection synced from another vault" flow.
func deriveSyncVaultKey(vaultPassword string) []byte {
	h := sha256.Sum256([]byte(syncVaultSaltInput))
	return account.DeriveKey(vaultPassword, h[:])
}

// encryptForSync encrypts a plaintext password with the given sync key and
// returns it in the format "sync:nonce_b64:ct_b64".
func encryptForSync(plaintext string, syncKey []byte) (string, error) {
	enc, err := account.EncryptField(syncKey, plaintext)
	if err != nil {
		return "", err
	}
	return "sync:" + enc, nil
}

// decryptFromSync decrypts a "sync:nonce_b64:ct_b64" value, trying each
// candidate key in order and returning the first successful decryption.
// Multiple keys exist to support both the current account-wide sync key and
// the legacy per-vault-password key for values written before it existed.
func decryptFromSync(encoded string, syncKeys ...[]byte) (string, error) {
	inner := strings.TrimPrefix(encoded, "sync:")
	var lastErr error
	for _, key := range syncKeys {
		if key == nil {
			continue
		}
		plain, err := account.DecryptField(key, inner)
		if err == nil {
			return plain, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no sync key available")
	}
	return "", lastErr
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
	enc, err := account.EncryptField(key, plaintext)
	if err != nil {
		return "", err
	}
	return connCipherPrefix + enc, nil
}

// decryptConnectionPassword decrypts an encrypted password using the vault master key.
// If the value doesn't look encrypted, returns it as-is for backward-compatibility
// with connections saved before encryption was added.
func decryptConnectionPassword(ciphertext string, key []byte) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if strings.HasPrefix(ciphertext, connCipherPrefix) {
		return account.DecryptField(key, strings.TrimPrefix(ciphertext, connCipherPrefix))
	}
	if looksLikeEncryptedField(ciphertext) {
		// Legacy encrypted value saved before connCipherPrefix was introduced.
		return account.DecryptField(key, ciphertext)
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
