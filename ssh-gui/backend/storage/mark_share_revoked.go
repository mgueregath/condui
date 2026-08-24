package storage

// MarkConnectionShareRevoked flags a connection as no longer accessible
// because the owner revoked or deleted the share it was imported from, and
// wipes its stored credentials so it can't be used to connect anymore.
func (d *Database) MarkConnectionShareRevoked(connectionID string) error {
	_, err := d.DB.Exec(
		`
		UPDATE connections
		SET
			share_revoked=1,
			password=NULL,
			passphrase=NULL
		WHERE id=?
		`,
		connectionID,
	)
	return err
}

// GetActiveSharedConnections returns the local connections that were
// imported from a share and have not yet been marked revoked, i.e. the
// candidates to check against the current list of active incoming shares.
func (d *Database) GetActiveSharedConnections() ([]SharedConnectionRef, error) {
	rows, err := d.DB.Query(
		`
		SELECT id, source_share_id
		FROM connections
		WHERE source_share_id IS NOT NULL AND share_revoked = 0
		`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []SharedConnectionRef{}
	for rows.Next() {
		var ref SharedConnectionRef
		if err := rows.Scan(&ref.ConnectionID, &ref.ShareID); err != nil {
			return nil, err
		}
		result = append(result, ref)
	}
	return result, nil
}

type SharedConnectionRef struct {
	ConnectionID string
	ShareID      string
}
