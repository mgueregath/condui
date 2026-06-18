package storage

import "ssh-gui/backend/models"

func (d *Database) UpsertFolder(folder *models.Folder) error {
	_, err := d.DB.Exec(
		`
		INSERT INTO folders(
			id,
			name,
			parent_id
		)
		VALUES(?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			parent_id=excluded.parent_id
		`,
		folder.ID,
		folder.Name,
		folder.ParentID,
	)
	return err
}
