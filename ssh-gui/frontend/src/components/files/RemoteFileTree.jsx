import { useEffect, useState, forwardRef, useImperativeHandle } from "react";

import {
  ListDirectory,
  DeleteRemoteFile,
  RenameRemoteFile,
  CreateRemoteDirectory,
  DownloadFile,
  ReadRemoteFile,
  SaveRemoteFile,
} from "../../../wailsjs/go/main/App";

import RemoteFileNode from "./RemoteFileNode";
import FileContextMenu from "./FileContextMenu";
import RemoteFileEditorModal from "../editor/RemoteFileEditorModal";

const RemoteFileTree = forwardRef(function RemoteFileTree({ sessionId }, ref) {
  const [path, setPath] = useState("/");
  const [files, setFiles] = useState([]);

  const [editor, setEditor] = useState({
    open: false,
    path: "",
    content: "",
    modified: false,
  });

  const [contextMenu, setContextMenu] = useState({
    visible: false,
    x: 0,
    y: 0,
    item: null,
  });

  const load = async (targetPath) => {
    if (!sessionId) return;

    const result = await ListDirectory(sessionId, targetPath);

    setFiles(result || []);
    setPath(targetPath);
  };

  const refresh = () => load(path);
  const goParent = () => {
    if (path === "/") {
      return;
    }

    const parts = path.split("/").filter(Boolean);

    parts.pop();

    const parent = parts.length === 0 ? "/" : "/" + parts.join("/");

    load(parent);
  };

  useImperativeHandle(ref, () => ({
  refresh,
  goParent,
  currentPath: path,
}));

  const openContextMenu = (item, x, y) => {
    setContextMenu({
      visible: true,
      x,
      y,
      item,
    });
  };

  const openFile = async (file) => {
    const content = await ReadRemoteFile(sessionId, file.path);

    setEditor({
      open: true,
      path: file.path,
      content,
      modified: false,
    });
  };

  const saveEditor = async () => {
    await SaveRemoteFile(sessionId, editor.path, editor.content);

    setEditor((e) => ({
      ...e,
      modified: false,
    }));
  };

  const closeEditor = async () => {
    if (editor.modified) {
      if (window.confirm("Save changes?")) {
        await saveEditor();
      }
    }

    setEditor({
      open: false,
      path: "",
      content: "",
      modified: false,
    });
  };

  const deleteFile = async (item) => {
    if (!confirm(`Delete ${item.name}?`)) return;

    await DeleteRemoteFile(sessionId, item.path);

    refresh();
  };

  const renameFile = async (item) => {
    const name = prompt("New name", item.name);

    if (!name) return;

    const parent = item.path.substring(0, item.path.lastIndexOf("/"));

    await RenameRemoteFile(sessionId, item.path, `${parent}/${name}`);

    refresh();
  };

  const createFolder = async () => {
    const name = prompt("Folder name");

    if (!name) return;

    await CreateRemoteDirectory(
      sessionId,
      path === "/" ? `/${name}` : `${path}/${name}`,
    );

    refresh();
  };

  const downloadFile = async (item) => {
    try {
      await DownloadFile(sessionId, item.path, "");
      alert("Descarga completada con éxito");
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    load("/");
  }, [sessionId]);

  const parts = path.split("/").filter(Boolean);

  return (
    <div className="remote-tree">
      <div className="files-breadcrumb">
        <span className="breadcrumb-item root" onClick={() => load("/")}>
          /
        </span>

        {parts.map((p, i) => (
          <span key={i}>
            <span
              className="breadcrumb-item"
              onClick={() => load("/" + parts.slice(0, i + 1).join("/"))}
            >
              {i == 0 ? "" : "/"}
              {p}
            </span>
          </span>
        ))}
      </div>

      <div
        className="files-list"
        onContextMenu={(e) => {
          if (e.target !== e.currentTarget) return;

          e.preventDefault();

          openContextMenu(
            {
              isBackground: true,
              isDirectory: true,
              path,
            },
            e.clientX,
            e.clientY,
          );
        }}
      >
        {files.map((file) => (
          <RemoteFileNode
            key={file.path}
            item={file}
            onOpen={(node) => {
              if (node.isDirectory) load(node.path);
            }}
            onOpenFile={openFile}
            onContextMenu={openContextMenu}
          />
        ))}
      </div>

      <FileContextMenu
        {...contextMenu}
        onDownload={downloadFile}
        onDelete={deleteFile}
        onRename={renameFile}
        onNewFolder={createFolder}
        onOpenFile={openFile}
        onClose={() =>
          setContextMenu((c) => ({
            ...c,
            visible: false,
          }))
        }
      />

      <RemoteFileEditorModal
        open={editor.open}
        path={editor.path}
        content={editor.content}
        modified={editor.modified}
        onChange={(value) =>
          setEditor((e) => ({
            ...e,
            content: value,
            modified: true,
          }))
        }
        onSave={saveEditor}
        onClose={closeEditor}
      />
    </div>
  );
});

export default RemoteFileTree;
