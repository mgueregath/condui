package storage

import (
	"github.com/google/uuid"

	"ssh-gui/backend/models"
)

func (d *Database) CreateFolder(
	name string,
) (*models.Folder, error) {

	folder := &models.Folder{
		ID: uuid.NewString(),
		Name: name,
	}

	_, err := d.DB.Exec(
		`
		INSERT INTO folders(
			id,
			name
		)
		VALUES(
			?,
			?
		)
		`,
		folder.ID,
		folder.Name,
	)

	if err != nil {
		return nil, err
	}

	return folder, nil
}
