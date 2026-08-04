import { useState } from "react";
import { FaTrash, FaPlay, FaPlug, FaEdit, FaShareAlt, FaNetworkWired, FaLock, FaTimes } from "react-icons/fa";
import { MdOpenInNew } from "react-icons/md";
import ContextMenu from "./ContextMenu";
import { useTranslation } from "react-i18next";

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
  onCancelConnect,
  onEdit,
  onDelete,
  onShare,
  onConnectVia,
  onUnlockPending,
}) {
  const { t } = useTranslation();
  const [ctx, setCtx] = useState(null);

  const connect = (e) => {
    e?.stopPropagation();
    if (!connecting) onOpen(connection);
  };

  const cancelConnect = (e) => {
    e?.stopPropagation();
    onCancelConnect?.(connection);
  };

  const handleContextMenu = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setCtx({ x: e.clientX, y: e.clientY });
  };

  const viaSubmenu = onConnectVia ? buildViaSubmenu(connection, connections, folders, onConnectVia) : null;

  const menuItems = [
    {
      icon: <FaPlug />,
      label: t("connection.connect"),
      onClick: () => !connecting && onOpen(connection),
    },
    {
      icon: <MdOpenInNew />,
      label: t("connection.newSession"),
      onClick: () => !connecting && onOpen(connection, true),
    },
    { divider: true },
    {
      icon: <FaEdit />,
      label: t("common.edit"),
      onClick: () => onEdit(connection),
    },
    ...(onShare ? [{
      icon: <FaShareAlt />,
      label: t("connection.share"),
      onClick: () => onShare(connection),
    }] : []),
    ...(viaSubmenu && viaSubmenu.length > 0 ? [
      { divider: true },
      {
        icon: <FaNetworkWired />,
        label: t("connection.connectVia"),
        submenu: viaSubmenu,
      },
    ] : []),
    { divider: true },
    {
      icon: <FaTrash />,
      label: t("common.delete"),
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
            <button
              type="button"
              className="conn-pending-badge"
              title={t("connection.passwordPending")}
              onClick={(e) => {
                e.stopPropagation();
                onUnlockPending?.(connection);
              }}
            >
              <FaLock />
            </button>
          )}
        </div>
        <div className="conn-host">
          {connection.username}@{connection.host}
        </div>
      </div>

      <div className={`conn-actions ${connecting ? "connecting" : ""}`}>
        <button
          className={`conn-action-btn ${connecting ? "cancel" : ""}`}
          title={connecting ? t("connection.cancelConnect") : t("connection.connect")}
          onClick={connecting ? cancelConnect : connect}
        >
          {connecting ? <FaTimes /> : <FaPlug />}
        </button>

        <button
          className="conn-action-btn"
          title={t("common.edit")}
          disabled={connecting}
          onClick={(e) => { e.stopPropagation(); onEdit(connection); }}
        >
          <FaEdit />
        </button>

        {onShare && (
          <button
            className="conn-action-btn"
            title={t("connection.share")}
            disabled={connecting}
            onClick={(e) => { e.stopPropagation(); onShare(connection); }}
          >
            <FaShareAlt />
          </button>
        )}

        <button
          className="conn-action-btn delete"
          title={t("common.delete")}
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
