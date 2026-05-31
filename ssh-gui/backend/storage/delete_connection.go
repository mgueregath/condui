package storage

func (d *Database) DeleteConnection(
	id string,
) error {

	_, err := d.DB.Exec(
		`
		DELETE FROM connections
		WHERE id = ?
		`,
		id,
	)

	return err
}
