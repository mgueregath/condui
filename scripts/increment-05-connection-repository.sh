#!/bin/bash

set -e

echo "========================================="
echo "Incremento 5 - Connection Repository"
echo "========================================="

cd ssh-gui

mkdir -p backend/storage
mkdir -p backend/models
mkdir -p frontend/src/types

cat > backend/models/connection.go <<'EOF'
package models

type Connection struct {
	ID string `json:"id"`

	FolderID *string `json:"folderId,omitempty"`

	Name string `json:"name"`

	Host string `json:"host"`

	Port int `json:"port"`

	Username string `json:"username"`

	AuthType string `json:"authType"`

	Password *string `json:"password,omitempty"`

	PrivateKeyPath *string `json:"privateKeyPath,omitempty"`

	Color *string `json:"color,omitempty"`
}
EOF

cat > backend/models/folder.go <<'EOF'
package models

type Folder struct {
	ID string `json:"id"`

	Name string `json:"name"`

	ParentID *string `json:"parentId,omitempty"`
}
EOF

cat > backend/storage/database.go <<'EOF'
package storage

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Database struct {
	DB *sql.DB
}

func NewDatabase(path string) (*Database, error) {

	db, err := sql.Open(
		"sqlite",
		path,
	)

	if err != nil {
		return nil, err
	}

	return &Database{
		DB: db,
	}, nil
}
EOF

cat > backend/storage/migrations.go <<'EOF'
package storage

func (d *Database) Migrate() error {

	queries := []string{

		`
		CREATE TABLE IF NOT EXISTS folders (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			parent_id TEXT
		);
		`,

		`
		CREATE TABLE IF NOT EXISTS connections (
			id TEXT PRIMARY KEY,

			folder_id TEXT,

			name TEXT NOT NULL,

			host TEXT NOT NULL,

			port INTEGER NOT NULL,

			username TEXT NOT NULL,

			auth_type TEXT NOT NULL,

			password TEXT,

			private_key_path TEXT,

			color TEXT
		);
		`,
	}

	for _, query := range queries {

		_, err := d.DB.Exec(query)

		if err != nil {
			return err
		}
	}

	return nil
}
EOF

cat > backend/storage/folders.go <<'EOF'
package storage

import (
	"github.com/google/uuid"

	"ssh-gui/backend/models"
)

func (d *Database) CreateFolder(
	name string,
) (*models.Folder, error) {

	folder := &models.Folder{
		ID: uuid.NewString(),
		Name: name,
	}

	_, err := d.DB.Exec(
		`
		INSERT INTO folders(
			id,
			name
		)
		VALUES(
			?,
			?
		)
		`,
		folder.ID,
		folder.Name,
	)

	if err != nil {
		return nil, err
	}

	return folder, nil
}
EOF

cat > backend/storage/connections.go <<'EOF'
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
			color
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
		connection.Color,
	)

	return err
}
EOF

cat > frontend/src/types/Connection.js <<'EOF'
export const AuthTypes = {
  PASSWORD: "password",
  PRIVATE_KEY: "privateKey",
};
EOF

cat > frontend/src/types/Folder.js <<'EOF'
export default {};
EOF

echo ""
echo "Incremento 5 preparado."
echo ""
echo "Archivos creados:"
echo "backend/models/connection.go"
echo "backend/models/folder.go"
echo "backend/storage/database.go"
echo "backend/storage/migrations.go"
echo "backend/storage/folders.go"
echo "backend/storage/connections.go"
echo "frontend/src/types/Connection.js"
echo "frontend/src/types/Folder.js"