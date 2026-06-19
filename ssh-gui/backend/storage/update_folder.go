package storage

func (d *Database) UpdateFolder(
	id string,
	name string,
) error {

	_, err := d.DB.Exec(
		`
		UPDATE folders
		SET name = ?
		WHERE id = ?
		`,
		name,
		id,
	)

	return err
}
