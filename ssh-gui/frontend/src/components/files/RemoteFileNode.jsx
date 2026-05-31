export default function RemoteFileNode({ item, onOpen }) {
  const isDir = item.isDirectory;
  return (
    <div className="file-node" onDoubleClick={() => onOpen(item)}>
      <div className="file-node-name">
        <span className={`file-icon ${isDir ? "dir" : ""}`}>
          {isDir ? "📁" : "📄"}
        </span>
        <span>{item.name}</span>
      </div>
      <span className="file-size">{item.size || "–"}</span>
      <span className="file-date">{item.modTime ? item.modTime.substring(0, 10) : "–"}</span>
    </div>
  );
}
