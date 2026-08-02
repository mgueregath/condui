import { useEffect, useState, useMemo, forwardRef, useImperativeHandle } from "react";

import {
  ListDirectory,
  DeleteRemoteFile,
  RenameRemoteFile,
  CreateRemoteDirectory,
  DownloadFile,
  ReadRemoteFile,
  SaveRemoteFile,
  OpenSqliteExplorerWindow,
} from "../../../bindings/ssh-gui/app";

import RemoteFileNode from "./RemoteFileNode";
import FileContextMenu from "./FileContextMenu";
import RemoteFileEditorModal from "../editor/RemoteFileEditorModal";
import AlertModal from "../common/AlertModal";
import Modal from "../common/Modal";
import { useTranslation } from "react-i18next";
import { FaLongArrowAltDown, FaLongArrowAltUp } from "react-icons/fa";

const RemoteFileTree = forwardRef(function RemoteFileTree(
  { sessionId, initialPath = "/", onPathChange, dbManagerEnabled = false },
  ref,
) {
  const { t } = useTranslation();
  const sortOptions = [
    { key: "name", label: t("files.name") },
    { key: "size", label: t("files.size") },
    { key: "modTime", label: t("files.modified") },
    { key: "type", label: t("files.type") },
  ];
  const [path, setPath] = useState("/");
  const [files, setFiles] = useState([]);
  const [sortBy, setSortBy] = useState("name");
  const [sortDir, setSortDir] = useState("asc");

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

  const [alertModal, setAlertModal] = useState(null);
  const showAlert = (message, title = t("app.notice")) =>
    setAlertModal({ title, message });

  const [deleteTarget, setDeleteTarget] = useState(null);

  const load = async (targetPath) => {
    if (!sessionId) return;
    const result = await ListDirectory(sessionId, targetPath);
    setFiles(result || []);
    setPath(targetPath);
    onPathChange?.(sessionId, targetPath);
  };

  const refresh = () => load(path);
  const goParent = () => {
    if (path === "/") return;
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
    setContextMenu({ visible: true, x, y, item });
  };

  const openFile = async (file) => {
    if (dbManagerEnabled && file.name?.toLowerCase().endsWith(".db")) {
      await OpenSqliteExplorerWindow(sessionId, file.path);
      return;
    }
    const content = await ReadRemoteFile(sessionId, file.path);
    setEditor({ open: true, path: file.path, content, modified: false });
  };

  const saveEditor = async () => {
    await SaveRemoteFile(sessionId, editor.path, editor.content);
    setEditor((e) => ({ ...e, modified: false }));
  };

  const closeEditor = async () => {
    if (editor.modified) {
      if (window.confirm(t("files.saveChangesConfirm"))) await saveEditor();
    }
    setEditor({ open: false, path: "", content: "", modified: false });
  };

  const deleteFile = (item) => {
    setDeleteTarget(item);
  };

  const confirmDeleteFile = async () => {
    if (!deleteTarget) return;
    await DeleteRemoteFile(sessionId, deleteTarget.path);
    setDeleteTarget(null);
    refresh();
  };

  const renameFile = async (item) => {
    const name = prompt(t("files.newName"), item.name);
    if (!name) return;
    const parent = item.path.substring(0, item.path.lastIndexOf("/"));
    await RenameRemoteFile(sessionId, item.path, `${parent}/${name}`);
    refresh();
  };

  const createFolder = async () => {
    const name = prompt(t("files.folderName"));
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
      showAlert(t("files.downloadComplete"));
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    load(initialPath || "/");
  }, [sessionId]);

  const toggleSort = (field) => {
    if (sortBy === field) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortBy(field);
      setSortDir("asc");
    }
  };

  const sortedFiles = useMemo(() => {
    if (!files.length) return files;
    return [...files].sort((a, b) => {
      let cmp = 0;
      switch (sortBy) {
        case "name":
          if (a.isDirectory !== b.isDirectory) return a.isDirectory ? -1 : 1;
          cmp = a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
          break;
        case "size":
          if (a.isDirectory !== b.isDirectory) return a.isDirectory ? -1 : 1;
          cmp = a.size - b.size;
          break;
        case "modTime": {
          const ta = a.modTime ? new Date(a.modTime).getTime() : 0;
          const tb = b.modTime ? new Date(b.modTime).getTime() : 0;
          cmp = ta - tb;
          break;
        }
        case "type":
          if (a.isDirectory !== b.isDirectory) return a.isDirectory ? -1 : 1;
          const extA = a.name.includes(".") ? a.name.split(".").pop().toLowerCase() : "";
          const extB = b.name.includes(".") ? b.name.split(".").pop().toLowerCase() : "";
          cmp = extA.localeCompare(extB);
          if (cmp === 0) cmp = a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
          break;
      }
      return sortDir === "asc" ? cmp : -cmp;
    });
  }, [files, sortBy, sortDir]);

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
              {i === 0 ? "" : "/"}
              {p}
            </span>
          </span>
        ))}
      </div>

      <div className="files-sort-bar">
        {sortOptions.map((opt) => (
          <button
            key={opt.key}
            className={"files-sort-btn" + (sortBy === opt.key ? " active" : "")}
            onClick={() => toggleSort(opt.key)}
          >
            {opt.label}
            {sortBy === opt.key && (
              <span className="sort-arrow">
                {sortDir === "asc" ? <FaLongArrowAltUp /> : <FaLongArrowAltDown />}
              </span>
            )}
          </button>
        ))}
      </div>

      <div
        className="files-list"
        onContextMenu={(e) => {
          if (e.target !== e.currentTarget) return;
          e.preventDefault();
          openContextMenu(
            { isBackground: true, isDirectory: true, path },
            e.clientX,
            e.clientY,
          );
        }}
      >
        {sortedFiles.map((file) => (
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
        onClose={() => setContextMenu((c) => ({ ...c, visible: false }))}
      />

      <RemoteFileEditorModal
        open={editor.open}
        path={editor.path}
        content={editor.content}
        modified={editor.modified}
        onChange={(value) =>
          setEditor((e) => ({ ...e, content: value, modified: true }))
        }
        onSave={saveEditor}
        onClose={closeEditor}
      />

      <AlertModal
        open={!!alertModal}
        title={alertModal?.title}
        message={alertModal?.message}
        onClose={() => setAlertModal(null)}
      />

      <Modal open={!!deleteTarget} onClose={() => setDeleteTarget(null)}>
        <div>
          <div className="modal-header">
            <h2>{t("files.delete")}</h2>
          </div>
          <div className="modal-body">
            <div className="ssh-error-box" style={{ borderColor: "var(--red)" }}>
              <p style={{ margin: 0 }}>
                {t("files.deleteConfirm", { name: deleteTarget?.name })}
              </p>
            </div>
          </div>
          <div className="modal-footer">
            <button className="btn-secondary" onClick={() => setDeleteTarget(null)}>
              {t("common.cancel")}
            </button>
            <button className="btn-primary" onClick={confirmDeleteFile}>
              {t("common.delete")}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
});

export default RemoteFileTree;
