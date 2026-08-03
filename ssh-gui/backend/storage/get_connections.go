package storage

import "ssh-gui/backend/models"

func (d *Database) GetConnections() (
	[]models.Connection,
	error,
) {

	rows, err :=
		d.DB.Query(
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
				passphrase,
				color,
				jump_host_id
			FROM connections
			ORDER BY name
			`,
		)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result :=
		[]models.Connection{}

	for rows.Next() {

		var connection models.Connection

		err := rows.Scan(
			&connection.ID,
			&connection.FolderID,
			&connection.Name,
			&connection.Host,
			&connection.Port,
			&connection.Username,
			&connection.AuthType,
			&connection.Password,
			&connection.PrivateKeyPath,
			&connection.Passphrase,
			&connection.Color,
			&connection.JumpHostID,
		)

		if err != nil {
			return nil, err
		}

		result =
			append(
				result,
				connection,
			)
	}

	return result, nil
}
