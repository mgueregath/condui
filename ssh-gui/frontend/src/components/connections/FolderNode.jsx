import { useState } from "react";
import ContextMenu from "./ContextMenu";
import { FaFolder, FaFolderOpen, FaFile, FaAngleLeft, FaAngleDown } from "react-icons/fa";

export default function FolderNode({ folder, expanded, onToggle, onEdit, onDelete, children }) {
  const [ctx, setCtx] = useState(null);

  const handleContextMenu = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setCtx({ x: e.clientX, y: e.clientY });
  };

  const menuItems = [
    { icon: "✏", label: "Rename folder", onClick: () => onEdit(folder) },
    { divider: true },
    { icon: "🗑", label: "Delete folder", danger: true, onClick: () => {
      if (confirm(`Delete folder "${folder.name}"?`)) onDelete(folder);
    }},
  ];

  return (
    <div style={{ marginBottom: "2px" }}>
      <div
        className="sidebar-item"
        onClick={() => onToggle(folder.id)}
        onContextMenu={handleContextMenu}
      >
        <span style={{ fontSize: "14px", flexShrink: 0 }}>
          {expanded ? <FaFolderOpen /> : <FaFolder />}
        </span>
        <div className="conn-info">
          <div className="conn-name">{folder.name}</div>
        </div>

        <span style={{ fontSize: "9px", color: "var(--text-muted)", width: "10px", flexShrink: 0 }}>
          {expanded ? <FaAngleDown /> : <FaAngleLeft />}
        </span>
      </div>

      {expanded && <div style={{ paddingLeft: "14px" }}>{children}</div>}

      {ctx && (
        <ContextMenu
          x={ctx.x} y={ctx.y}
          items={menuItems}
          onClose={() => setCtx(null)}
        />
      )}
    </div>
  );
}