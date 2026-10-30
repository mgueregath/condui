package storage

import "strings"

func (d *Database) Migrate() error {

	queries := []string{

		`
		CREATE TABLE IF NOT EXISTS folders (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			parent_id TEXT
		);
		`,

		`
		CREATE TABLE IF NOT EXISTS connections (
			id TEXT PRIMARY KEY,
			folder_id TEXT,
			name TEXT NOT NULL,
			host TEXT NOT NULL,
			port INTEGER NOT NULL,
			username TEXT NOT NULL,
			auth_type TEXT NOT NULL,
			password TEXT,
			private_key_path TEXT,
			color TEXT
		);
		`,

		`
		CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
		`,

		`
		CREATE TABLE IF NOT EXISTS known_hosts (
			hostname TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 22,
			fingerprint TEXT NOT NULL,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (hostname, port)
		);
		`,

		`
		CREATE TABLE IF NOT EXISTS tunnels (
			id TEXT PRIMARY KEY,
			connection_id TEXT NOT NULL,
			local_port INTEGER NOT NULL,
			remote_host TEXT NOT NULL,
			remote_port INTEGER NOT NULL
		);
		`,

		`
		CREATE INDEX IF NOT EXISTS idx_tunnels_connection_id ON tunnels(connection_id);
		`,

		`
		CREATE TABLE IF NOT EXISTS account (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			server_url TEXT,
			user_id TEXT,
			email TEXT,
			tier TEXT DEFAULT 'free',
			access_token TEXT,
			refresh_token TEXT,
			public_key TEXT,
			identity_blob TEXT,
			sync_key TEXT
		);
		`,
	}

	for _, query := range queries {
		if _, err := d.DB.Exec(query); err != nil {
			return err
		}
	}

	// ALTER TABLE migrations — idempotent via duplicate-column error suppression
	alterations := []string{
		`ALTER TABLE connections ADD COLUMN ssh_enabled INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE connections ADD COLUMN vnc_config TEXT`,
		`ALTER TABLE connections ADD COLUMN protocol TEXT NOT NULL DEFAULT 'ssh'`,
		`ALTER TABLE account ADD COLUMN tier_expires_at DATETIME`,
		`ALTER TABLE account ADD COLUMN sync_key TEXT`,
		`ALTER TABLE connections ADD COLUMN jump_host_id TEXT REFERENCES connections(id)`,
		`ALTER TABLE connections ADD COLUMN passphrase TEXT`,
		`ALTER TABLE connections ADD COLUMN source_share_id TEXT`,
		`ALTER TABLE connections ADD COLUMN share_revoked INTEGER NOT NULL DEFAULT 0`,
	}
	for _, alt := range alterations {
		if _, err := d.DB.Exec(alt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
	}

	// Move legacy VNC credentials without decrypting or duplicating them.
	_, err := d.DB.Exec(`UPDATE connections SET
  ssh_enabled=0,
  vnc_config=json_object('enabled',json('true'),'port',port,'username',username,'password',password,'reuse',json_object('user',json('false'),'pass',json('false'))),
  port=22, username='', auth_type='password', password=NULL, private_key_path=NULL, passphrase=NULL, jump_host_id=NULL
  WHERE protocol='vnc' AND vnc_config IS NULL`)
	if err != nil {
		return err
	}
	return nil
}
