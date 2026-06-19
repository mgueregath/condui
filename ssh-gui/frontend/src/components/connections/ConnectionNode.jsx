import { useState } from "react";
import { FaTrash, FaPlay, FaEdit, FaShareAlt, FaNetworkWired, FaLock } from "react-icons/fa";
import { MdOpenInNew } from "react-icons/md";
import ContextMenu from "./ContextMenu";

function buildViaSubmenu(connection, connections, folders, onConnectVia) {
  const others = connections.filter(c => c.id !== connection.id);
  if (others.length === 0) return null;

  const items = [];
  const inFolder = [];

  folders.forEach(folder => {
    const fc = others.filter(c => c.folderId === folder.id);
    if (fc.length === 0) return;
    inFolder.push(...fc.map(c => c.id));
    items.push({ header: true, label: folder.name });
    fc.forEach(c => {
      items.push({
        label: c.name,
        icon: <span style={{ color: c.color || "inherit", fontSize: 9 }}>●</span>,
        onClick: () => onConnectVia(connection, c),
      });
    });
  });

  const ungrouped = others.filter(c => !c.folderId && !inFolder.includes(c.id));
  if (ungrouped.length > 0) {
    if (items.length > 0) items.push({ divider: true });
    ungrouped.forEach(c => {
      items.push({
        label: c.name,
        icon: <span style={{ color: c.color || "inherit", fontSize: 9 }}>●</span>,
        onClick: () => onConnectVia(connection, c),
      });
    });
  }

  return items;
}

export default function ConnectionNode({
  connection,
  connecting = false,
  connections = [],
  folders = [],
  onOpen,
  onEdit,
  onDelete,
  onShare,
  onConnectVia,
}) {
  const [ctx, setCtx] = useState(null);

  const connect = (e) => {
    e?.stopPropagation();
    if (!connecting) onOpen(connection);
  };

  const handleContextMenu = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setCtx({ x: e.clientX, y: e.clientY });
  };

  const viaSubmenu = onConnectVia ? buildViaSubmenu(connection, connections, folders, onConnectVia) : null;

  const menuItems = [
    {
      icon: <FaPlay />,
      label: "Conectar",
      onClick: () => !connecting && onOpen(connection),
    },
    {
      icon: <MdOpenInNew />,
      label: "Nueva sesión",
      onClick: () => !connecting && onOpen(connection, true),
    },
    { divider: true },
    {
      icon: <FaEdit />,
      label: "Editar",
      onClick: () => onEdit(connection),
    },
    ...(onShare ? [{
      icon: <FaShareAlt />,
      label: "Compartir",
      onClick: () => onShare(connection),
    }] : []),
    ...(viaSubmenu && viaSubmenu.length > 0 ? [
      { divider: true },
      {
        icon: <FaNetworkWired />,
        label: "Conectar a través de",
        submenu: viaSubmenu,
      },
    ] : []),
    { divider: true },
    {
      icon: <FaTrash />,
      label: "Eliminar",
      danger: true,
      onClick: () => onDelete(connection),
    },
  ];

  return (
    <div
      className="drawer-conn-item"
      onDoubleClick={connect}
      onContextMenu={handleContextMenu}
      style={{ "--connection-color": connection.color }}
    >
      <div className="conn-status">
        {connecting
          ? <span className="conn-loader" />
          : <>
              <span className={`conn-dot ${connection.online > 0 ? "online" : "offline"}`} />
              {connection.online > 1 && <span className="conn-count">{connection.online}</span>}
            </>
        }
      </div>

      <div className="conn-info">
        <div className="conn-name" style={{ color: connection.color }}>
          {connection.name}
          {connection.passwordPending && (
            <span
              className="conn-pending-badge"
              title="Contraseña pendiente de sincronización. Desbloquea el vault con tu contraseña para activar esta conexión."
            >
              <FaLock />
            </span>
          )}
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
          <FaPlay />
        </button>

        <button
          className="conn-action-btn"
          title="Edit"
          disabled={connecting}
          onClick={(e) => { e.stopPropagation(); onEdit(connection); }}
        >
          <FaEdit />
        </button>

        {onShare && (
          <button
            className="conn-action-btn"
            title="Share"
            disabled={connecting}
            onClick={(e) => { e.stopPropagation(); onShare(connection); }}
          >
            <FaShareAlt />
          </button>
        )}

        <button
          className="conn-action-btn delete"
          title="Delete"
          disabled={connecting}
          onClick={(e) => {
            e.stopPropagation();
            onDelete(connection);
          }}
        >
          <FaTrash />
        </button>
      </div>

      {ctx && (
        <ContextMenu
          x={ctx.x}
          y={ctx.y}
          items={menuItems}
          onClose={() => setCtx(null)}
        />
      )}
    </div>
  );
}
