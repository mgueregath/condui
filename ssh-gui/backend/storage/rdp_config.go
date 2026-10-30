package storage

import (
	"database/sql"
	"encoding/json"
	"ssh-gui/backend/models"
)

func encodeRDP(config *models.RDPConfig) (any, error) {
	if config == nil {
		return nil, nil
	}
	data, err := json.Marshal(config)
	return string(data), err
}
func decodeRDP(raw sql.NullString, c *models.Connection) error {
	if !raw.Valid || raw.String == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw.String), &c.RDP)
}
