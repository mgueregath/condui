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
        padding: "8px 10px",
        cursor: "pointer",
        borderRadius: "6px",
        margin: "4px 8px",
        background: "#262a31",
        border: "1px solid #333",
      }}
    >

      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "10px",
          flex: 1,
          overflow: "hidden",
        }}
      >

        <div
          style={{
            width: "10px",
            height: "10px",
            borderRadius: "50%",
            background:
              connection.color ||
              "#3b82f6",
            flexShrink: 0,
          }}
        />

        <div
          style={{
            overflow: "hidden",
          }}
        >

          <div
            style={{
              fontSize: "13px",
              fontWeight: "600",
              whiteSpace: "nowrap",
              overflow: "hidden",
              textOverflow: "ellipsis",
            }}
          >
            {connection.name}
          </div>

          <div
            style={{
              fontSize: "11px",
              color: "#888",
              whiteSpace: "nowrap",
              overflow: "hidden",
              textOverflow: "ellipsis",
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
          gap: "4px",
          marginLeft: "8px",
        }}
      >

        <button
          title="Conectar"
          onClick={(e) => {

            e.stopPropagation();

            onOpen(
              connection
            );

          }}
          style={{
            cursor: "pointer",
          }}
        >
          ▶
        </button>

        <button
          title="Editar"
          onClick={(e) => {

            e.stopPropagation();

            onEdit(
              connection
            );

          }}
          style={{
            cursor: "pointer",
          }}
        >
          ✏
        </button>

        <button
          title="Eliminar"
          onClick={(e) => {

            e.stopPropagation();

            onDelete(
              connection
            );

          }}
          style={{
            cursor: "pointer",
          }}
        >
          🗑
        </button>

      </div>

    </div>

  );

}