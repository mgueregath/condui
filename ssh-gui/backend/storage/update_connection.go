package storage

import "ssh-gui/backend/models"

func (d *Database) UpdateConnection(
	connection *models.Connection,
) error {
	if err := connection.ValidateProtocol(); err != nil {
		return err
	}

	rdpData, err := encodeRDP(connection.RDP)
	if err != nil {
		return err
	}
	vncData, err := encodeVNC(connection.VNC)
	if err != nil {
		return err
	}
	_, err = d.DB.Exec(
		`
		UPDATE connections
		SET
			ssh_enabled=?,
			rdp_config=?,
			vnc_config=?,
			protocol=?,
			folder_id=?,
			name=?,
			host=?,
			port=?,
			username=?,
			auth_type=?,
			password=?,
			private_key_path=?,
			passphrase=?,
			color=?,
			jump_host_id=?
		WHERE id=?
		`,
		connection.HasSSH(),
		rdpData,
		vncData,
		connection.ConnectionProtocol(),
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
		connection.ID,
	)
	if err != nil {
		return err
	}

	if connection.Tunnels != nil {
		connection.Tunnels, err = d.replaceTunnels(connection.ID, connection.Tunnels)
	}
	return err
}
