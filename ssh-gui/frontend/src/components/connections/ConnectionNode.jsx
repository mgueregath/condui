export default function ConnectionNode({ connection, onOpen, onEdit, onDelete }) {
  return (
    <div className="drawer-conn-item" onDoubleClick={() => onOpen(connection)}>
      <span className={`conn-dot ${connection.online ? "online" : "offline"}`} />
      <div className="conn-info">
        <div className="conn-name">{connection.name}</div>
        <div className="conn-host">{connection.username}@{connection.host}</div>
      </div>
      <div className="conn-actions">
        <button className="conn-action-btn" title="Connect" onClick={(e) => { e.stopPropagation(); onOpen(connection); }}>▶</button>
        <button className="conn-action-btn" title="Edit" onClick={(e) => { e.stopPropagation(); onEdit(connection); }}>✏</button>
        <button className="conn-action-btn delete" title="Delete" onClick={(e) => {
          e.stopPropagation();
          if (confirm(`Delete "${connection.name}"?`)) onDelete(connection);
        }}>🗑</button>
      </div>
    </div>
  );
}
