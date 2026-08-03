package storage

import (
	"github.com/google/uuid"

	"ssh-gui/backend/models"
)

func (d *Database) CreateConnection(
	connection *models.Connection,
) error {

	connection.ID =
		uuid.NewString()

	_, err := d.DB.Exec(
		`
		INSERT INTO connections(
			id,
			folder_id,
			name,
			host,
			port,
			username,
			auth_type,
			password,
			private_key_path,
			passphrase,
			color,
			jump_host_id
		)
		VALUES(
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?
		)
		`,
		connection.ID,
		connection.FolderID,
		connection.Name,
		connection.Host,
		connection.Port,
		connection.Username,
		connection.AuthType,
		connection.Password,
		connection.PrivateKeyPath,
		connection.Passphrase,
		connection.Color,
		connection.JumpHostID,
	)

	return err
}
