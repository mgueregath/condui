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
  onEditFolder,
  onDeleteFolder,
}) {

  const ungroupedConnections =
    connections.filter(
      c =>
        !c.folderId ||
        c.folderId === ""
    );

  return (
    <div>

      {ungroupedConnections.length > 0 && (

        <div
          style={{
            padding: "8px",
            borderBottom:
              "1px solid #333",
          }}
        >

          <div
            style={{
              color: "#888",
              fontSize: "12px",
              marginBottom: "8px",
              textTransform:
                "uppercase",
            }}
          >
            Sin carpeta
          </div>

          {ungroupedConnections.map(
            connection => (

              <ConnectionNode
                key={connection.id}
                connection={connection}
                onOpen={
                  onOpenConnection
                }
                onEdit={
                  onEditConnection
                }
                onDelete={
                  onDeleteConnection
                }
              />

            )
          )}

        </div>

      )}

      {folders.map(folder => (

        <FolderNode
          key={folder.id}
          folder={folder}
          expanded={
            expandedFolders.includes(
              folder.id
            )
          }
          onToggle={
            onToggleFolder
          }
          onEdit={
            onEditFolder
          }
          onDelete={
            onDeleteFolder
          }
        >

          {connections
            .filter(
              c =>
                c.folderId ===
                folder.id
            )
            .map(connection => (

              <ConnectionNode
                key={connection.id}
                connection={
                  connection
                }
                onOpen={
                  onOpenConnection
                }
                onEdit={
                  onEditConnection
                }
                onDelete={
                  onDeleteConnection
                }
              />

            ))}

        </FolderNode>

      ))}

    </div>
  );

}