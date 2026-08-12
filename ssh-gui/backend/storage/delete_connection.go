package storage

func (d *Database) DeleteConnection(
	id string,
) error {

	if err := d.deleteTunnelsByConnectionID(id); err != nil {
		return err
	}

	_, err := d.DB.Exec(
		`
		DELETE FROM connections
		WHERE id = ?
		`,
		id,
	)

	return err
}
