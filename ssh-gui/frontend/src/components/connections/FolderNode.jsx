export default function FolderNode({
  folder,
  expanded,
  onToggle,
  onEdit,
  onDelete,
  children,
}) {

  return (
    <div>

      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "8px 12px",
        }}
      >

        <div
          onClick={() => onToggle(folder.id)}
          style={{
            cursor: "pointer",
            flex: 1,
          }}
        >
          {expanded ? "▼" : "▶"} {folder.name}
        </div>

        <div
          style={{
            display: "flex",
            gap: "4px",
          }}
        >
          <button
            onClick={() => onEdit(folder)}
          >
            ✎
          </button>

          <button
            onClick={() => onDelete(folder)}
          >
            🗑
          </button>
        </div>

      </div>

      {expanded && (
        <div>
          {children}
        </div>
      )}

    </div>
  );
}