export default function FolderNode({
  folder,
  expanded,
  onToggle,
  onEdit,
  onDelete,
  children,
}) {

  return (

    <div
      style={{
        marginBottom: "14px",

        border: "1px solid #e5e7eb",

        borderRadius: "14px",

        background: "#ffffff",

        overflow: "hidden",

        boxShadow:
          "0 2px 8px rgba(0,0,0,0.04)",
      }}
    >

      <div
        style={{
          display: "flex",

          alignItems: "center",

          justifyContent:
            "space-between",

          padding: "12px 14px",

          background:
            expanded
              ? "#f8fafc"
              : "#ffffff",

          borderBottom:
            expanded
              ? "1px solid #e5e7eb"
              : "none",
        }}
      >

        <div
          onClick={() =>
            onToggle(folder.id)
          }
          style={{
            flex: 1,

            display: "flex",

            alignItems: "center",

            gap: "10px",

            cursor: "pointer",

            userSelect: "none",
          }}
        >

          <span
            style={{
              color: "#6b7280",

              fontSize: "12px",

              width: "12px",
            }}
          >
            {expanded
              ? "▼"
              : "▶"}
          </span>

          <span
            style={{
              fontSize: "14px",

              fontWeight: "600",

              color: "#111827",
            }}
          >
            📁 {folder.name}
          </span>

        </div>

        <div
          style={{
            display: "flex",

            gap: "6px",

            marginLeft: "10px",
          }}
        >

          <button
            title="Edit folder"
            onClick={() =>
              onEdit(folder)
            }
            style={{
              width: "32px",
              height: "32px",

              border:
                "1px solid #e5e7eb",

              borderRadius: "8px",

              background:
                "#ffffff",

              cursor: "pointer",
            }}
          >
            ✏
          </button>

          <button
            title="Delete folder"
            onClick={() => {

              const confirmed =
                confirm(
                  `Delete folder "${folder.name}"?`
                );

              if (!confirmed) {
                return;
              }

              onDelete(folder);

            }}
            style={{
              width: "32px",
              height: "32px",

              border:
                "1px solid #fee2e2",

              borderRadius: "8px",

              background:
                "#fef2f2",

              color: "#dc2626",

              cursor: "pointer",
            }}
          >
            🗑
          </button>

        </div>

      </div>

      {expanded && (

        <div
          style={{
            padding: "10px",

            background:
              "#ffffff",
          }}
        >
          {children}
        </div>

      )}

    </div>

  );

}