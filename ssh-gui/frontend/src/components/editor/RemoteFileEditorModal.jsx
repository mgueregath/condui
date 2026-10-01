import { useEffect } from "react";
import Editor from "@monaco-editor/react";
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
  path,
  content,
  modified,
  onChange,
  onClose,
  onSave,
}) {
  const { t } = useTranslation();
  const image = isImage(path);

  useEffect(() => {
    if (!open) return;

    const handleSaveShortcut = (event) => {
      if (!(event.ctrlKey || event.metaKey) || event.altKey || event.shiftKey || event.key.toLowerCase() !== "s") return;
      event.preventDefault();
      event.stopPropagation();
      if (modified && !image && !event.repeat) onSave();
    };

    document.addEventListener("keydown", handleSaveShortcut, true);
    return () => document.removeEventListener("keydown", handleSaveShortcut, true);
  }, [open, modified, image, onSave]);

  if (!open) return null;
  const fileName = path?.split("/").pop();
  const language = getLanguage(path);

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
        <div className="remote-editor-body">
          {image ? (
            <div className="remote-image-viewer">
              <img src={`data:image/*;base64,${content}`} alt={fileName} />
            </div>
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
              onChange={(v) => onChange(v ?? "")}
            />
          )}
        </div>
        <div className="remote-editor-footer">
          <button className="btn-secondary" onClick={onClose}>
            {t("common.close")}
          </button>
          {!image && (
            <button
              className="btn-primary"
              disabled={!modified}
              onClick={onSave}
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
