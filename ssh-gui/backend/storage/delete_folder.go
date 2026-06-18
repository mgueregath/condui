package storage

func (d *Database) DeleteFolder(
	id string,
) error {

	if _, err := d.DB.Exec(
		`
		UPDATE connections
		SET folder_id = NULL
		WHERE folder_id = ?
		`,
		id,
	); err != nil {
		return err
	}

	_, err := d.DB.Exec(
		`
		DELETE FROM folders
		WHERE id = ?
		`,
		id,
	)

	return err
}
