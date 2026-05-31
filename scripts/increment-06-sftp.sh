#!/bin/bash

mkdir -p ssh-gui/backend/sftp

cat > ssh-gui/backend/sftp/file_item.go <<'EOF'
package sftp

type FileItem struct {
	Name string `json:"name"`
	Path string `json:"path"`

	IsDirectory bool `json:"isDirectory"`

	Size int64 `json:"size"`

	Mode string `json:"mode"`
}
EOF

cat > ssh-gui/backend/sftp/list_directory.go <<'EOF'
package sftp

import gosftp "github.com/pkg/sftp"

func ListDirectory(
	client *gosftp.Client,
	path string,
) (
	[]FileItem,
	error,
) {

	files, err :=
		client.ReadDir(path)

	if err != nil {
		return nil, err
	}

	result :=
		[]FileItem{}

	for _, file := range files {

		result =
			append(
				result,
				FileItem{
					Name: file.Name(),
					Path: path + "/" + file.Name(),

					IsDirectory:
						file.IsDir(),

					Size:
						file.Size(),

					Mode:
						file.Mode().String(),
				},
			)

	}

	return result, nil
}
EOF

mkdir -p ssh-gui/frontend/src/components/files

cat > ssh-gui/frontend/src/components/files/RemoteFileNode.jsx <<'EOF'
export default function RemoteFileNode({
  item,
  onOpen,
}) {

  return (

    <div
      onDoubleClick={() =>
        onOpen(item)
      }
      style={{
        padding: "6px 10px",
        cursor: "pointer",
      }}
    >

      {
        item.isDirectory
          ? "📁"
          : "📄"
      }

      {" "}

      {item.name}

    </div>

  );

}
EOF

cat > ssh-gui/frontend/src/components/files/RemoteFileTree.jsx <<'EOF'
import { useEffect, useState } from "react";

import {
  ListDirectory,
} from "../../../wailsjs/go/main/App";

import RemoteFileNode from "./RemoteFileNode";

export default function RemoteFileTree({
  sessionId,
}) {

  const [path, setPath] =
    useState("/");

  const [files, setFiles] =
    useState([]);

  const load =
    async (
      targetPath
    ) => {

      if (!sessionId) {
        return;
      }

      try {

        const result =
          await ListDirectory(
            sessionId,
            targetPath,
          );

        setFiles(
          result || [],
        );

        setPath(
          targetPath,
        );

      } catch (err) {

        console.error(err);

      }

    };

  useEffect(() => {

    load("/");

  }, [sessionId]);

  return (

    <div>

      <div
        style={{
          padding: "10px",
          borderBottom:
            "1px solid #333",
          fontSize: "12px",
        }}
      >
        {path}
      </div>

      {
        files.map(item => (

          <RemoteFileNode
            key={item.path}
            item={item}
            onOpen={(node) => {

              if (
                node.isDirectory
              ) {

                load(
                  node.path
                );

              }

            }}
          />

        ))
      }

    </div>

  );

}
EOF

echo "SFTP files created"