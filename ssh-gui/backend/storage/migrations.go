package storage

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
		CREATE TABLE IF NOT EXISTS account (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			server_url TEXT,
			user_id TEXT,
			email TEXT,
			tier TEXT DEFAULT 'free',
			access_token TEXT,
			refresh_token TEXT,
			public_key TEXT,
			identity_blob TEXT
		);
		`,
	}

	for _, query := range queries {

		_, err := d.DB.Exec(query)

		if err != nil {
			return err
		}
	}

	return nil
}
