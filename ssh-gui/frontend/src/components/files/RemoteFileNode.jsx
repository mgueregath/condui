import { FaFolder, FaFile } from "react-icons/fa";

function fmtSize(bytes, isDir) {
  if (isDir) return "Carpeta";
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log2(Math.max(bytes, 1)) / 10);
  const idx = Math.min(i, units.length - 1);
  const val = bytes / Math.pow(1024, idx);
  return (idx === 0 ? val.toFixed(0) : val.toFixed(1)) + " " + units[idx];
}

function fmtDate(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, { day: "2-digit", month: "short", year: "numeric" });
}

export default function RemoteFileNode({ item, onOpen, onOpenFile, onContextMenu }) {
  const isDir = item.isDirectory;

  return (
    <div
      className="file-node"
      onDoubleClick={() => {
        if (isDir) onOpen(item);
        else onOpenFile(item);
      }}
      onContextMenu={(e) => {
        e.preventDefault();
        onContextMenu?.(item, e.clientX, e.clientY);
      }}
    >
      <div className={"file-node-icon" + (isDir ? " dir" : "")}>
        {isDir ? <FaFolder /> : <FaFile />}
      </div>
      <div className="file-node-info">
        <span className="file-node-name">{item.name}</span>
        <span className="file-node-meta">
          {fmtSize(item.size, isDir)}
          {item.modTime && <span className="file-node-meta-sep">·</span>}
          {fmtDate(item.modTime)}
        </span>
      </div>
    </div>
  );
}
