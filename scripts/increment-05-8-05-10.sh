#!/bin/bash

set -e

cd ssh-gui

echo "================================="
echo "Incremento 5.8 -> 5.10"
echo "================================="

#################################################
# UPDATE FOLDER
#################################################

cat > backend/storage/update_folder.go <<'EOF'
package storage

func (d *Database) UpdateFolder(
	id string,
	name string,
) error {

	_, err := d.DB.Exec(
		`
		UPDATE folders
		SET name = ?
		WHERE id = ?
		`,
		name,
		id,
	)

	return err
}
EOF

#################################################
# DELETE FOLDER
#################################################

cat > backend/storage/delete_folder.go <<'EOF'
package storage

func (d *Database) DeleteFolder(
	id string,
) error {

	_, err := d.DB.Exec(
		`
		DELETE FROM folders
		WHERE id = ?
		`,
		id,
	)

	return err
}
EOF

#################################################
# UPDATE CONNECTION
#################################################

cat > backend/storage/update_connection.go <<'EOF'
package storage

import "ssh-gui/backend/models"

func (d *Database) UpdateConnection(
	connection *models.Connection,
) error {

	_, err := d.DB.Exec(
		`
		UPDATE connections
		SET
			folder_id=?,
			name=?,
			host=?,
			port=?,
			username=?,
			auth_type=?,
			password=?,
			private_key_path=?,
			color=?
		WHERE id=?
		`,
		connection.FolderID,
		connection.Name,
		connection.Host,
		connection.Port,
		connection.Username,
		connection.AuthType,
		connection.Password,
		connection.PrivateKeyPath,
		connection.Color,
		connection.ID,
	)

	return err
}
EOF

#################################################
# DELETE CONNECTION
#################################################

cat > backend/storage/delete_connection.go <<'EOF'
package storage

func (d *Database) DeleteConnection(
	id string,
) error {

	_, err := d.DB.Exec(
		`
		DELETE FROM connections
		WHERE id = ?
		`,
		id,
	)

	return err
}
EOF

#################################################
# DRAWER
#################################################

cat > frontend/src/components/connections/ConnectionDrawer.jsx <<'EOF'
import FolderNode from "./FolderNode";
import ConnectionNode from "./ConnectionNode";

export default function ConnectionDrawer({
  folders,
  connections,
  expandedFolders,
  onToggleFolder,
  onOpenConnection,
  onEditConnection,
  onDeleteConnection,
}) {

  return (
    <div>

      {folders.map(folder => (

        <FolderNode
          key={folder.id}
          folder={folder}
          expanded={expandedFolders.includes(folder.id)}
          onToggle={onToggleFolder}
        >

          {connections
            .filter(
              c => c.folderId === folder.id
            )
            .map(connection => (

              <ConnectionNode
                key={connection.id}
                connection={connection}
                onOpen={onOpenConnection}
                onEdit={onEditConnection}
                onDelete={onDeleteConnection}
              />

            ))}

        </FolderNode>

      ))}

    </div>
  );

}
EOF

echo ""
echo "Incremento 5.8 -> 5.10 generado"
echo ""