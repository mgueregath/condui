import { useEffect, useRef } from "react";
import { createPortal } from "react-dom";

export default function ContextMenu({ x, y, items, onClose }) {
  const ref = useRef(null);

  const menuWidth = 200;
  const menuHeight = items.length * 34;
  const adjustedX = x + menuWidth > window.innerWidth  ? x - menuWidth : x;
  const adjustedY = y + menuHeight > window.innerHeight ? y - menuHeight : y;

  useEffect(() => {
    const handleDown   = (e) => { if (ref.current && !ref.current.contains(e.target)) onClose(); };
    const handleEsc    = (e) => { if (e.key === "Escape") onClose(); };
    const handleScroll = ()  => onClose();
    document.addEventListener("mousedown", handleDown);
    document.addEventListener("keydown",   handleEsc);
    document.addEventListener("scroll",    handleScroll, true);
    return () => {
      document.removeEventListener("mousedown", handleDown);
      document.removeEventListener("keydown",   handleEsc);
      document.removeEventListener("scroll",    handleScroll, true);
    };
  }, [onClose]);

  return createPortal(
    <div
      ref={ref}
      className="ctx-menu"
      style={{ left: adjustedX, top: adjustedY }}
      onContextMenu={e => e.preventDefault()}
    >
      {items.map((item, i) =>
        item.divider
          ? <div key={i} className="ctx-divider" />
          : <button
              key={i}
              className={`ctx-item${item.danger ? " danger" : ""}`}
              onClick={() => { item.onClick(); onClose(); }}
            >
              <span className="ctx-icon">{item.icon}</span>
              {item.label}
            </button>
      )}
    </div>,
    document.body
  );
}