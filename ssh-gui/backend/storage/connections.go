package storage

import (
	"github.com/google/uuid"

	"ssh-gui/backend/models"
)

func (d *Database) CreateConnection(
	connection *models.Connection,
) error {
	if err := connection.ValidateProtocol(); err != nil {
		return err
	}

	connection.ID =
		uuid.NewString()

	vncData, err := encodeVNC(connection.VNC)
	if err != nil {
		return err
	}
	_, err = d.DB.Exec(
		`
		INSERT INTO connections(
			ssh_enabled,
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
			?,
			?,
			?,
			?,
			?,
			?
		)
		`,
		connection.HasSSH(),
		vncData,
		connection.ConnectionProtocol(),
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
		connection.SourceShareID,
		connection.ShareRevoked,
	)
	if err != nil {
		return err
	}

	if connection.Tunnels != nil {
		connection.Tunnels, err = d.replaceTunnels(connection.ID, connection.Tunnels)
	}
	return err
}
