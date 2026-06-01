import { useEffect, useRef } from "react";
import { createPortal } from "react-dom";

export default function FileContextMenu({
  x,
  y,
  visible,
  item,
  onDownload,
  onDelete,
  onRename,
  onNewFolder,
  onClose,
}) {
  const ref = useRef(null);

  useEffect(() => {
    if (!visible) return;

    const handleDown = (e) => {
      if (
        ref.current &&
        !ref.current.contains(e.target)
      ) {
        onClose?.();
      }
    };

    const handleEsc = (e) => {
      if (e.key === "Escape") {
        onClose?.();
      }
    };

    const handleScroll = () => {
      onClose?.();
    };

    document.addEventListener(
      "mousedown",
      handleDown
    );

    document.addEventListener(
      "keydown",
      handleEsc
    );

    document.addEventListener(
      "scroll",
      handleScroll,
      true
    );

    return () => {
      document.removeEventListener(
        "mousedown",
        handleDown
      );

      document.removeEventListener(
        "keydown",
        handleEsc
      );

      document.removeEventListener(
        "scroll",
        handleScroll,
        true
      );
    };
  }, [visible, onClose]);

  if (!visible || !item) {
    return null;
  }

  const items = [];

  /*
   * Click derecho sobre espacio vacío
   */
  if (item.isBackground) {
    items.push({
      icon: "📁",
      label: "New Folder",
      onClick: () => onNewFolder(item),
    });
  }

  /*
   * Click derecho sobre archivo
   */
  else if (!item.isDirectory) {
    items.push({
      icon: "⬇",
      label: "Download",
      onClick: () => onDownload(item),
    });

    items.push({
      icon: "✏️",
      label: "Rename",
      onClick: () => onRename(item),
    });

    items.push({
      divider: true,
    });

    items.push({
      icon: "🗑",
      label: "Delete",
      danger: true,
      onClick: () => onDelete(item),
    });
  }

  /*
   * Click derecho sobre carpeta
   */
  else {
    items.push({
      icon: "📁",
      label: "New Folder",
      onClick: () => onNewFolder(item),
    });

    items.push({
      icon: "✏️",
      label: "Rename",
      onClick: () => onRename(item),
    });

    items.push({
      divider: true,
    });

    items.push({
      icon: "🗑",
      label: "Delete",
      danger: true,
      onClick: () => onDelete(item),
    });
  }

  const menuWidth = 200;
  const menuHeight = items.length * 34;

  const adjustedX =
    x + menuWidth > window.innerWidth
      ? x - menuWidth
      : x;

  const adjustedY =
    y + menuHeight > window.innerHeight
      ? y - menuHeight
      : y;

  return createPortal(
    <div
      ref={ref}
      className="ctx-menu"
      style={{
        left: adjustedX,
        top: adjustedY,
      }}
      onContextMenu={(e) =>
        e.preventDefault()
      }
    >
      {items.map((menuItem, i) =>
        menuItem.divider ? (
          <div
            key={i}
            className="ctx-divider"
          />
        ) : (
          <button
            key={i}
            className={`ctx-item${
              menuItem.danger
                ? " danger"
                : ""
            }`}
            onClick={() => {
              menuItem.onClick();
              onClose?.();
            }}
          >
            <span className="ctx-icon">
              {menuItem.icon}
            </span>

            {menuItem.label}
          </button>
        )
      )}
    </div>,
    document.body
  );
}