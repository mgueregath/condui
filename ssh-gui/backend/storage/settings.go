package storage

func (d *Database) GetSetting(key string) (string, error) {
	var value string
	err := d.DB.QueryRow(
		`SELECT value FROM settings WHERE key = ?`, key,
	).Scan(&value)
	if err != nil {
		return "", nil // not found = empty string
	}
	return value, nil
}

func (d *Database) SetSetting(key, value string) error {
	_, err := d.DB.Exec(
		`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
