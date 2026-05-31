export default function ConnectionNode({
  connection,
  onOpen,
  onEdit,
  onDelete,
}) {

  return (

    <div
      onDoubleClick={() =>
        onOpen(connection)
      }
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",

        padding: "12px",

        marginBottom: "10px",

        borderRadius: "14px",

        background: "#ffffff",

        border: "1px solid #e5e7eb",

        boxShadow:
          "0 2px 8px rgba(0,0,0,0.04)",

        cursor: "pointer",

        transition: "all .15s ease",
      }}
    >

      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "12px",
          flex: 1,
          minWidth: 0,
        }}
      >

        <div
          style={{
            width: "12px",
            height: "12px",

            borderRadius: "999px",

            background:
              connection.color ||
              "#2563eb",

            flexShrink: 0,
          }}
        />

        <div
          style={{
            flex: 1,
            minWidth: 0,
          }}
        >

          <div
            style={{
              fontSize: "14px",

              fontWeight: 600,

              color: "#111827",

              whiteSpace: "nowrap",

              overflow: "hidden",

              textOverflow: "ellipsis",
            }}
          >
            {connection.name}
          </div>

          <div
            style={{
              fontSize: "12px",

              color: "#6b7280",

              whiteSpace: "nowrap",

              overflow: "hidden",

              textOverflow: "ellipsis",

              marginTop: "2px",
            }}
          >
            {connection.username}
            @
            {connection.host}
            :
            {connection.port}
          </div>

        </div>

      </div>

      <div
        style={{
          display: "flex",

          gap: "6px",

          marginLeft: "12px",
        }}
      >

        <button
          title="Connect"
          onClick={(e) => {

            e.stopPropagation();

            onOpen(connection);

          }}
          style={{
            width: "34px",
            height: "34px",

            border: "1px solid #dbeafe",

            borderRadius: "10px",

            background: "#eff6ff",

            color: "#2563eb",

            cursor: "pointer",

            fontWeight: "bold",
          }}
        >
          ▶
        </button>

        <button
          title="Edit"
          onClick={(e) => {

            e.stopPropagation();

            onEdit(connection);

          }}
          style={{
            width: "34px",
            height: "34px",

            border: "1px solid #e5e7eb",

            borderRadius: "10px",

            background: "#ffffff",

            cursor: "pointer",
          }}
        >
          ✏
        </button>

        <button
          title="Delete"
          onClick={(e) => {

            e.stopPropagation();

            const confirmed =
              confirm(
                `Delete "${connection.name}"?`
              );

            if (!confirmed) {
              return;
            }

            onDelete(connection);

          }}
          style={{
            width: "34px",
            height: "34px",

            border: "1px solid #fee2e2",

            borderRadius: "10px",

            background: "#fef2f2",

            color: "#dc2626",

            cursor: "pointer",
          }}
        >
          🗑
        </button>

      </div>

    </div>

  );

}