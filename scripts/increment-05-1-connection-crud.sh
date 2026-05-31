#!/bin/bash

set -e

echo "====================================="
echo "Incremento 5.1 - Connection CRUD"
echo "====================================="

cd ssh-gui

mkdir -p frontend/src/hooks
mkdir -p frontend/src/components/connections

cat > backend/storage/get_folders.go <<'EOF'
package storage

import "ssh-gui/backend/models"

func (d *Database) GetFolders() (
	[]models.Folder,
	error,
) {

	rows, err :=
		d.DB.Query(
			`
			SELECT
				id,
				name,
				parent_id
			FROM folders
			ORDER BY name
			`,
		)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result :=
		[]models.Folder{}

	for rows.Next() {

		var folder models.Folder

		err := rows.Scan(
			&folder.ID,
			&folder.Name,
			&folder.ParentID,
		)

		if err != nil {
			return nil, err
		}

		result =
			append(
				result,
				folder,
			)
	}

	return result, nil
}
EOF

cat > backend/storage/get_connections.go <<'EOF'
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
				color
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
			&connection.Color,
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
EOF

cat > frontend/src/hooks/useConnections.js <<'EOF'
import {
  useEffect,
  useState,
} from "react";

import {
  GetFolders,
  GetConnections,
} from "../../wailsjs/go/main/App";

export function useConnections() {

  const [folders,
    setFolders] =
    useState([]);

  const [connections,
    setConnections] =
    useState([]);

  const load =
    async () => {

      const f =
        await GetFolders();

      const c =
        await GetConnections();

      setFolders(f || []);
      setConnections(c || []);

    };

  useEffect(() => {
    load();
  }, []);

  return {
    folders,
    connections,
    reload: load,
  };
}
EOF

cat > frontend/src/components/connections/FolderModal.jsx <<'EOF'
export default function FolderModal() {
  return null;
}
EOF

cat > frontend/src/components/connections/ConnectionModal.jsx <<'EOF'
export default function ConnectionModal() {
  return null;
}
EOF

cat > frontend/src/components/connections/FolderNode.jsx <<'EOF'
export default function FolderNode({
  folder,
}) {

  return (
    <div>
      {folder.name}
    </div>
  );

}
EOF

cat > frontend/src/components/connections/ConnectionNode.jsx <<'EOF'
export default function ConnectionNode({
  connection,
}) {

  return (
    <div>
      {connection.name}
    </div>
  );

}
EOF

echo ""
echo "Incremento 5.1 preparado"
echo ""
echo "Archivos nuevos:"
echo ""
echo "backend/storage/get_folders.go"
echo "backend/storage/get_connections.go"
echo ""
echo "frontend/src/hooks/useConnections.js"
echo ""
echo "frontend/src/components/connections/*"