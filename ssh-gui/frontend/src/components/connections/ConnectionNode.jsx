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
    <div className="drawer-conn-item" onDoubleClick={connect} 
    style={{
      "--connection-color": connection.color,
    }}>
      <div className="conn-status">
        <span
          className={`conn-dot ${connection.online > 0 ? "online" : "offline"}`}
        />

        {connection.online > 1 && (
          <span className="conn-count">{connection.online}</span>
        )}
      </div>
      <div className="conn-info">
        <div className="conn-name" style={{ color: connection.color }}>
          {connection.name}
        </div>

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

