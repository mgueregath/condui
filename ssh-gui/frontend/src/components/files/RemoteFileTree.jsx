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
