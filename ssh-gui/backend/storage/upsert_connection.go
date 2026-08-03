package storage

import "ssh-gui/backend/models"

func (d *Database) UpsertConnection(connection *models.Connection) error {
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
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			folder_id=excluded.folder_id,
			name=excluded.name,
			host=excluded.host,
			port=excluded.port,
			username=excluded.username,
			auth_type=excluded.auth_type,
			password=excluded.password,
			private_key_path=excluded.private_key_path,
			passphrase=excluded.passphrase,
			color=excluded.color,
			jump_host_id=excluded.jump_host_id
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
