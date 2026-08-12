package storage

import (
	"github.com/google/uuid"

	"ssh-gui/backend/models"
)

// getTunnelsByConnectionID returns the tunnels configured for a connection,
// always as a non-nil slice (empty when there are none) so callers building
// a Connection for sync can tell "zero tunnels" apart from "not loaded".
func (d *Database) getTunnelsByConnectionID(connectionID string) ([]models.TunnelInfo, error) {
	rows, err := d.DB.Query(
		`
		SELECT id, connection_id, local_port, remote_host, remote_port
		FROM tunnels
		WHERE connection_id = ?
		ORDER BY local_port
		`,
		connectionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.TunnelInfo{}
	for rows.Next() {
		var tunnel models.TunnelInfo
		if err := rows.Scan(&tunnel.ID, &tunnel.ConnectionID, &tunnel.LocalPort, &tunnel.RemoteHost, &tunnel.RemotePort); err != nil {
			return nil, err
		}
		result = append(result, tunnel)
	}
	return result, nil
}

// replaceTunnels overwrites every tunnel stored for connectionID with the
// given set (assigning an ID to any that don't already have one) and returns
// the persisted result. Used whenever a connection is created/updated/synced
// so tunnel changes ride along with the rest of the connection's fields
// instead of needing their own separate CRUD/sync surface.
func (d *Database) replaceTunnels(connectionID string, tunnels []models.TunnelInfo) ([]models.TunnelInfo, error) {
	tx, err := d.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM tunnels WHERE connection_id = ?`, connectionID); err != nil {
		return nil, err
	}

	result := make([]models.TunnelInfo, len(tunnels))
	for i, tunnel := range tunnels {
		if tunnel.ID == "" {
			tunnel.ID = uuid.NewString()
		}
		tunnel.ConnectionID = connectionID
		tunnel.Active = false

		if _, err := tx.Exec(
			`INSERT INTO tunnels(id, connection_id, local_port, remote_host, remote_port) VALUES (?, ?, ?, ?, ?)`,
			tunnel.ID, tunnel.ConnectionID, tunnel.LocalPort, tunnel.RemoteHost, tunnel.RemotePort,
		); err != nil {
			return nil, err
		}
		result[i] = tunnel
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *Database) deleteTunnelsByConnectionID(connectionID string) error {
	_, err := d.DB.Exec(`DELETE FROM tunnels WHERE connection_id = ?`, connectionID)
	return err
}

// GetTunnelsByConnectionID returns the tunnels configured for a connection.
func (d *Database) GetTunnelsByConnectionID(connectionID string) ([]models.TunnelInfo, error) {
	return d.getTunnelsByConnectionID(connectionID)
}

// CreateTunnel adds a single tunnel to a connection.
func (d *Database) CreateTunnel(connectionID string, localPort int, remoteHost string, remotePort int) (models.TunnelInfo, error) {
	tunnel := models.TunnelInfo{
		ID:         uuid.NewString(),
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
	}
	_, err := d.DB.Exec(
		`INSERT INTO tunnels(id, connection_id, local_port, remote_host, remote_port) VALUES (?, ?, ?, ?, ?)`,
		tunnel.ID, connectionID, tunnel.LocalPort, tunnel.RemoteHost, tunnel.RemotePort,
	)
	if err != nil {
		return models.TunnelInfo{}, err
	}
	tunnel.ConnectionID = connectionID
	return tunnel, nil
}

// UpdateTunnel updates a single tunnel's parameters.
func (d *Database) UpdateTunnel(tunnelID string, localPort int, remoteHost string, remotePort int) (models.TunnelInfo, error) {
	_, err := d.DB.Exec(
		`UPDATE tunnels SET local_port = ?, remote_host = ?, remote_port = ? WHERE id = ?`,
		localPort, remoteHost, remotePort, tunnelID,
	)
	if err != nil {
		return models.TunnelInfo{}, err
	}
	var tunnel models.TunnelInfo
	row := d.DB.QueryRow(`SELECT id, connection_id, local_port, remote_host, remote_port FROM tunnels WHERE id = ?`, tunnelID)
	if err := row.Scan(&tunnel.ID, &tunnel.ConnectionID, &tunnel.LocalPort, &tunnel.RemoteHost, &tunnel.RemotePort); err != nil {
		return models.TunnelInfo{}, err
	}
	return tunnel, nil
}

// DeleteTunnel removes a single tunnel by ID.
func (d *Database) DeleteTunnel(tunnelID string) error {
	_, err := d.DB.Exec(`DELETE FROM tunnels WHERE id = ?`, tunnelID)
	return err
}
