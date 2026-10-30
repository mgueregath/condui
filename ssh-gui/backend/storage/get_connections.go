package storage

import (
	"database/sql"

	"ssh-gui/backend/models"
)

func (d *Database) GetConnections() (
	[]models.Connection,
	error,
) {

	rows, err :=
		d.DB.Query(
			`
			SELECT
			ssh_enabled,
			rdp_config,
			vnc_config,
			protocol,
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
				jump_host_id,
				source_share_id,
				share_revoked
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
		var vncData sql.NullString
		var rdpData sql.NullString

		err := rows.Scan(
			&connection.SSHEnabled,
			&rdpData,
			&vncData,
			&connection.Protocol,
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
			&connection.SourceShareID,
			&connection.ShareRevoked,
		)

		if err != nil {
			return nil, err
		}

		if err := decodeRDP(rdpData, &connection); err != nil {
			return nil, err
		}
		if err := decodeVNC(vncData, &connection); err != nil {
			return nil, err
		}
		connection.Tunnels, err = d.getTunnelsByConnectionID(connection.ID)
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
