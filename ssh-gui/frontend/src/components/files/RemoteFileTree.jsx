import { useEffect, useState } from "react";
import { ListDirectory } from "../../../wailsjs/go/main/App";
import RemoteFileNode from "./RemoteFileNode";

export default function RemoteFileTree({ sessionId }) {
  const [path, setPath] = useState("/");
  const [files, setFiles] = useState([]);

  const load = async (targetPath) => {
    if (!sessionId) return;
    try {
      const result = await ListDirectory(sessionId, targetPath);
      setFiles(result || []);
      setPath(targetPath);
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => { load("/"); }, [sessionId]);

  const parts = path.split("/").filter(Boolean);

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <div className="files-breadcrumb">
        <span className="breadcrumb-item" onClick={() => load("/")}>/ </span>
        {parts.map((p, i) => (
          <span key={i} style={{ display: "flex", alignItems: "center", gap: "4px" }}>
            <span className="breadcrumb-sep">›</span>
            <span
              className={`breadcrumb-item ${i === parts.length - 1 ? "current" : ""}`}
              onClick={() => i < parts.length - 1 ? load("/" + parts.slice(0, i + 1).join("/")) : undefined}
            >{p}</span>
          </span>
        ))}
      </div>
      <div className="files-cols">
        <span className="files-col-label">Name</span>
        <span className="files-col-label">Size</span>
        <span className="files-col-label">Modified</span>
      </div>
      <div className="files-list">
        {files.map(item => (
          <RemoteFileNode
            key={item.path} item={item}
            onOpen={(node) => { if (node.isDirectory) load(node.path); }}
          />
        ))}
      </div>
      <div className="files-footer">
        <span>{files.length} items</span>
        <span>{files.reduce((a, f) => a + (f.sizeBytes || 0), 0) > 0
          ? (files.reduce((a, f) => a + (f.sizeBytes || 0), 0) / 1024).toFixed(1) + " KB"
          : ""}</span>
      </div>
    </div>
  );
}
