import { useCallback, useEffect, useRef, useState } from "react";
import Editor from "@monaco-editor/react";
import RemoteMarkdownPreview from "./RemoteMarkdownPreview";
import "./RemoteFileEditor.css";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";

const getExtension = (path = "") => {
  return path.split(".").pop()?.toLowerCase();
};

const getLanguage = (path) => {
  const ext = getExtension(path);
  const map = {
    js: "javascript",
    jsx: "javascript",
    ts: "typescript",
    tsx: "typescript",
    json: "json",
    html: "html",
    css: "css",
    go: "go",
    py: "python",
    java: "java",
    sh: "shell",
    yaml: "yaml",
    yml: "yaml",
    md: "markdown",
    markdown: "markdown",
    sql: "sql",
    xml: "xml",
    env: "plaintext",
    log: "plaintext",
    txt: "plaintext",
  };
  return map[ext] || "plaintext";
};

const isImage = (path) => {
  const ext = getExtension(path);
  return ["png", "jpg", "jpeg", "gif", "webp", "bmp", "svg"].includes(ext);
};

export default function RemoteFileEditorModal({
  open,
  sessionId,
  path,
  content,
  modified,
  onChange,
  onClose,
  onSave,
}) {
  const { t } = useTranslation();
  const image = isImage(path);
  const markdown = ["md", "markdown"].includes(getExtension(path));
  const [preview, setPreview] = useState(true);
  const [saveStatus, setSaveStatus] = useState("idle");
  const saveContext = useRef({ saving: false });

  useEffect(() => {
    saveContext.current = { saving: false };
    setSaveStatus("idle");
    return () => { saveContext.current = { saving: false }; };
  }, [open, path, sessionId]);

  useEffect(() => {
    if (saveStatus !== "saved") return;
    const timer = setTimeout(() => setSaveStatus("idle"), 3000);
    return () => clearTimeout(timer);
  }, [saveStatus]);

  const handleSave = useCallback(async () => {
    const context = saveContext.current;
    if (!open || image || !modified || context.saving) return;
    context.saving = true;
    setSaveStatus("saving");
    try {
      await onSave();
      if (saveContext.current === context) setSaveStatus("saved");
    } catch {
      if (saveContext.current === context) setSaveStatus("error");
    } finally {
      context.saving = false;
    }
  }, [open, image, modified, onSave]);

  useEffect(() => {
    setPreview(true);
  }, [open, path, sessionId]);

  useEffect(() => {
    if (!open) return;

    const handleSaveShortcut = (event) => {
      if (!(event.ctrlKey || event.metaKey) || event.altKey || event.shiftKey || event.key.toLowerCase() !== "s") return;
      event.preventDefault();
      event.stopPropagation();
      if (!event.repeat) handleSave();
    };

    document.addEventListener("keydown", handleSaveShortcut, true);
    return () => document.removeEventListener("keydown", handleSaveShortcut, true);
  }, [open, handleSave]);

  if (!open) return null;
  const fileName = path?.split("/").pop();
  const language = getLanguage(path);
  const status = saveStatus === "saving" || saveStatus === "error"
    ? saveStatus
    : modified ? "unsaved" : saveStatus;
  const statusLabels = {
    saving: "files.savingChanges",
    saved: "files.changesSaved",
    unsaved: "files.unsavedChanges",
    error: "files.saveChangesFailed",
  };

  return createPortal(
    <div className="remote-editor-backdrop">
      <div className="remote-editor-window">
        <div className="remote-editor-header">
          <div>
            <div className="remote-editor-title">
              {fileName}
              {modified && !image && (
                <span className="editor-modified">●</span>
              )}
            </div>
            <div className="remote-editor-path">{path}</div>
          </div>
          <button className="remote-editor-close" onClick={onClose}>
            ×
          </button>
        </div>
        {markdown && (
          <div className="remote-markdown-toolbar">
            <button className="btn-secondary" aria-pressed={preview} onClick={() => setPreview(true)}>
              {t("files.preview")}
            </button>
            <button className="btn-secondary" aria-pressed={!preview} onClick={() => setPreview(false)}>
              {t("common.edit")}
            </button>
          </div>
        )}
        <div className="remote-editor-body">
          {image ? (
            <div className="remote-image-viewer">
              <img src={`data:image/*;base64,${content}`} alt={fileName} />
            </div>
          ) : markdown && preview ? (
            <RemoteMarkdownPreview content={content} path={path} sessionId={sessionId} />
          ) : (
            <Editor
              height="100%"
              theme="vs-dark"
              language={language}
              value={content}
              options={{
                minimap: { enabled: false },
                fontSize: 14,
                automaticLayout: true,
                scrollBeyondLastLine: false,
                wordWrap: "on",
                padding: { top: 12 },
              }}
              onChange={(v) => {
                if (!saveContext.current.saving) setSaveStatus("idle");
                onChange(v ?? "");
              }}
            />
          )}
        </div>
        <div className="remote-editor-footer">
          {!image && (
            <span className="remote-editor-save-status" data-state={status} role="status" aria-live="polite">
              {statusLabels[status] ? t(statusLabels[status]) : ""}
            </span>
          )}
          <button className="btn-secondary" onClick={onClose}>
            {t("common.close")}
          </button>
          {!image && (
            <button
              className="btn-primary"
              disabled={!modified || saveStatus === "saving"}
              onClick={handleSave}
            >
              {t("common.save")}
            </button>
          )}
        </div>
      </div>
    </div>,
    document.body,
  );
}
