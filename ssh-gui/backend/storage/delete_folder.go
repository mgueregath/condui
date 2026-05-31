package storage

func (d *Database) DeleteFolder(
	id string,
) error {

	_, err := d.DB.Exec(
		`
		DELETE FROM folders
		WHERE id = ?
		`,
		id,
	)

	return err
}
