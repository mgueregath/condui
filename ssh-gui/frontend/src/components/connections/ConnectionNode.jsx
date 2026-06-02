import { FaTrash, FaPlay, FaEdit } from "react-icons/fa";

export default function ConnectionNode({
  connection,
  connecting = false,
  onOpen,
  onEdit,
  onDelete,
}) {
  const connect = (e) => {
    e?.stopPropagation();

    if (!connecting) {
      onOpen(connection);
    }
  };

  return (
    <div className="drawer-conn-item" onDoubleClick={connect}>
      <span
        className={`conn-dot ${connection.online ? "online" : "offline"}`}
      />

      <div className="conn-info">
        <div className="conn-name">{connection.name}</div>

        <div className="conn-host">
          {connection.username}@{connection.host}
        </div>
      </div>

      <div className={`conn-actions ${connecting ? "connecting" : ""}`}>
        <button
          className="conn-action-btn"
          title={connecting ? "Connecting..." : "Connect"}
          disabled={connecting}
          onClick={connect}
        >
          {connecting ? <span className="conn-loader" /> : <FaPlay />}
        </button>

        <button
          className="conn-action-btn"
          title="Edit"
          disabled={connecting}
          onClick={(e) => {
            e.stopPropagation();
            onEdit(connection);
          }}
        >
          <FaEdit />
        </button>

        <button
          className="conn-action-btn delete"
          title="Delete"
          disabled={connecting}
          onClick={(e) => {
            e.stopPropagation();

            if (confirm(`Delete "${connection.name}"?`)) {
              onDelete(connection);
            }
          }}
        >
          <FaTrash />
        </button>
      </div>
    </div>
  );
}
