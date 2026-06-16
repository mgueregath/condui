package db

import "strings"

func (d *DB) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			tier TEXT NOT NULL DEFAULT 'free',
			public_key TEXT,
			identity_blob TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL,
			device_name TEXT,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS blobs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			blob_type TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			cipher_text TEXT NOT NULL,
			nonce TEXT NOT NULL,
			checksum TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS share_invites (
			id TEXT PRIMARY KEY,
			owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			recipient_email TEXT NOT NULL,
			blob_id TEXT NOT NULL REFERENCES blobs(id) ON DELETE CASCADE,
			encrypted_key TEXT,
			permissions TEXT NOT NULL DEFAULT 'read',
			status TEXT NOT NULL DEFAULT 'pending',
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Tier limits — configurable without code deploys
		`CREATE TABLE IF NOT EXISTS tier_limits (
			tier TEXT NOT NULL,
			resource TEXT NOT NULL,
			max_count INTEGER NOT NULL,
			PRIMARY KEY (tier, resource)
		)`,

		// Seed tier limits — INSERT OR REPLACE so values update on migrations
		`INSERT OR REPLACE INTO tier_limits VALUES ('free', 'blobs', 5)`,
		`INSERT OR REPLACE INTO tier_limits VALUES ('free', 'devices', 1)`,
		`INSERT OR REPLACE INTO tier_limits VALUES ('free', 'shares', 0)`,
		`INSERT OR REPLACE INTO tier_limits VALUES ('pro', 'blobs', -1)`,
		`INSERT OR REPLACE INTO tier_limits VALUES ('pro', 'devices', -1)`,
		`INSERT OR REPLACE INTO tier_limits VALUES ('pro', 'shares', -1)`,
	}

	for _, stmt := range statements {
		if _, err := d.Exec(stmt); err != nil {
			return err
		}
	}

	// ALTER TABLE is not idempotent in SQLite — ignore "duplicate column" errors.
	if _, err := d.Exec(`ALTER TABLE users ADD COLUMN tier_expires_at DATETIME`); err != nil {
		if !isDuplicateColumn(err) {
			return err
		}
	}

	return nil
}

func isDuplicateColumn(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists")
}
