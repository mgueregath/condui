package storage

import "ssh-gui/backend/models"

func (d *Database) GetConnectionByID(
	id string,
) (*models.Connection, error) {

	row := d.DB.QueryRow(
		`
		SELECT
			id,
			folder_id,
			name,
			host,
			port,
			username,
			auth_type,
			password,
			private_key_path,
			color
		FROM connections
		WHERE id = ?
		`,
		id,
	)

	var connection models.Connection

	err := row.Scan(
		&connection.ID,
		&connection.FolderID,
		&connection.Name,
		&connection.Host,
		&connection.Port,
		&connection.Username,
		&connection.AuthType,
		&connection.Password,
		&connection.PrivateKeyPath,
		&connection.Color,
	)

	if err != nil {
		return nil, err
	}

	return &connection, nil
}