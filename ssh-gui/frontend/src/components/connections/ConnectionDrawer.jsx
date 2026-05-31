import { useMemo, useState } from "react";

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

  const [search, setSearch] =
    useState("");

  const filteredConnections =
    useMemo(() => {

      const value =
        search
          .trim()
          .toLowerCase();

      if (!value) {
        return connections;
      }

      return connections.filter(c => {

        return (
          c.name
            ?.toLowerCase()
            .includes(value) ||

          c.host
            ?.toLowerCase()
            .includes(value) ||

          c.username
            ?.toLowerCase()
            .includes(value)
        );

      });

    }, [
      search,
      connections,
    ]);

  const ungroupedConnections =
    filteredConnections.filter(
      c =>
        !c.folderId ||
        c.folderId === ""
    );

  return (

    <div
      style={{
        height: "100%",

        display: "flex",
        flexDirection: "column",
      }}
    >

      <div
        style={{
          padding: "16px",
          borderBottom:
            "1px solid #e5e7eb",
          background: "#fff",
        }}
      >

        <input
          placeholder="Search connections..."
          value={search}
          onChange={(e) =>
            setSearch(
              e.target.value
            )
          }
          style={{
            width: "100%",

            padding: "12px",

            borderRadius: "10px",

            border:
              "1px solid #e5e7eb",

            fontSize: "14px",
          }}
        />

        <div
          style={{
            marginTop: "10px",

            fontSize: "12px",

            color: "#6b7280",
          }}
        >
          {filteredConnections.length}
          {" "}
          connection(s)
        </div>

      </div>

      <div
        style={{
          flex: 1,
          overflow: "auto",

          padding: "12px",
        }}
      >

        {ungroupedConnections.length > 0 && (

          <div
            style={{
              marginBottom: "20px",
            }}
          >

            <div
              style={{
                fontSize: "11px",

                fontWeight: "700",

                color: "#6b7280",

                letterSpacing: "1px",

                textTransform:
                  "uppercase",

                marginBottom: "10px",
              }}
            >
              Connections
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

        {folders.map(folder => {

          const folderConnections =
            filteredConnections.filter(
              c =>
                c.folderId ===
                folder.id
            );

          if (
            folderConnections.length === 0 &&
            search
          ) {
            return null;
          }

          return (

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

              {folderConnections.map(
                connection => (

                  <ConnectionNode
                    key={
                      connection.id
                    }
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

                )
              )}

            </FolderNode>

          );

        })}

      </div>

    </div>

  );

}