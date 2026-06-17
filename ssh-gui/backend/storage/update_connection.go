package storage

import "ssh-gui/backend/models"

func (d *Database) UpdateConnection(
	connection *models.Connection,
) error {

	_, err := d.DB.Exec(
		`
		UPDATE connections
		SET
			folder_id=?,
			name=?,
			host=?,
			port=?,
			username=?,
			auth_type=?,
			password=?,
			private_key_path=?,
			color=?,
			jump_host_id=?
		WHERE id=?
		`,
		connection.FolderID,
		connection.Name,
		connection.Host,
		connection.Port,
		connection.Username,
		connection.AuthType,
		connection.Password,
		connection.PrivateKeyPath,
		connection.Color,
		connection.JumpHostID,
		connection.ID,
	)

	return err
}
