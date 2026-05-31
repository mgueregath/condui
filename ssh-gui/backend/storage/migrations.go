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
	}

	for _, query := range queries {

		_, err := d.DB.Exec(query)

		if err != nil {
			return err
		}
	}

	return nil
}
