package storage

func (d *Database) GetKnownHost(hostname string, port int) (string, error) {
	var fingerprint string
	err := d.DB.QueryRow(
		`SELECT fingerprint FROM known_hosts WHERE hostname = ? AND port = ?`,
		hostname, port,
	).Scan(&fingerprint)
	if err != nil {
		return "", nil // not found = empty string, no error
	}
	return fingerprint, nil
}

func (d *Database) UpsertKnownHost(hostname string, port int, fingerprint string) error {
	_, err := d.DB.Exec(
		`INSERT INTO known_hosts(hostname, port, fingerprint)
		 VALUES(?, ?, ?)
		 ON CONFLICT(hostname, port) DO UPDATE SET fingerprint = excluded.fingerprint`,
		hostname, port, fingerprint,
	)
	return err
}
