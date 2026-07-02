import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { BsFillCaretRightFill } from "react-icons/bs";

function SubmenuPanel({ items, anchorRef, onClose, onMouseEnter, onMouseLeave }) {
  const ref = useRef(null);

  useEffect(() => {
    if (!ref.current || !anchorRef.current) return;
    const anchor = anchorRef.current.getBoundingClientRect();
    const panel = ref.current;
    const panelW = panel.offsetWidth || 220;
    const panelH = panel.offsetHeight || items.length * 34;

    const spaceRight = window.innerWidth - anchor.right;
    const left = spaceRight >= panelW + 4 ? anchor.right + 4 : anchor.left - panelW - 4;

    let top = anchor.top;
    if (top + panelH > window.innerHeight) top = window.innerHeight - panelH - 4;

    panel.style.left = `${left}px`;
    panel.style.top = `${top}px`;
  }, [items]);

  return createPortal(
    <div
      ref={ref}
      className="ctx-menu ctx-submenu-panel"
      onContextMenu={e => e.preventDefault()}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
    >
      {items.map((item, i) =>
        item.divider ? (
          <div key={i} className="ctx-divider" />
        ) : item.header ? (
          <div key={i} className="ctx-group-header">{item.label}</div>
        ) : (
          <button
            key={i}
            className={`ctx-item${item.danger ? " danger" : ""}`}
            onClick={() => { item.onClick(); onClose(); }}
          >
            {item.icon && <span className="ctx-icon">{item.icon}</span>}
            {item.label}
          </button>
        )
      )}
    </div>,
    document.body
  );
}

function SubmenuItem({ item, onClose }) {
  const [open, setOpen] = useState(false);
  const ref = useRef(null);
  const closeTimer = useRef(null);

  const cancelClose = () => {
    if (closeTimer.current) clearTimeout(closeTimer.current);
  };

  const scheduleClose = () => {
    closeTimer.current = setTimeout(() => setOpen(false), 100);
  };

  useEffect(() => () => { if (closeTimer.current) clearTimeout(closeTimer.current); }, []);

  return (
    <div
      ref={ref}
      className="ctx-item ctx-has-submenu"
      onMouseEnter={() => { cancelClose(); setOpen(true); }}
      onMouseLeave={scheduleClose}
    >
      <span className="ctx-icon">{item.icon}</span>
      {item.label}
      <span className="ctx-arrow"><BsFillCaretRightFill /></span>
      {open && (
        <SubmenuPanel
          items={item.submenu}
          anchorRef={ref}
          onClose={onClose}
          onMouseEnter={cancelClose}
          onMouseLeave={scheduleClose}
        />
      )}
    </div>
  );
}

export default function ContextMenu({ x, y, items, onClose }) {
  const ref = useRef(null);

  const menuWidth = 210;
  const menuHeight = items.length * 34;
  const adjustedX = x + menuWidth > window.innerWidth ? x - menuWidth : x;
  const adjustedY = y + menuHeight > window.innerHeight ? y - menuHeight : y;

  useEffect(() => {
    const handleDown = (e) => {
      // Don't close if the click is inside the main menu OR inside a submenu panel portal.
      if (ref.current?.contains(e.target)) return;
      if (e.target.closest?.(".ctx-submenu-panel")) return;
      onClose();
    };
    const handleEsc    = (e) => { if (e.key === "Escape") onClose(); };
    const handleScroll = ()  => onClose();
    document.addEventListener("mousedown", handleDown);
    document.addEventListener("keydown", handleEsc);
    document.addEventListener("scroll", handleScroll, true);
    return () => {
      document.removeEventListener("mousedown", handleDown);
      document.removeEventListener("keydown", handleEsc);
      document.removeEventListener("scroll", handleScroll, true);
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
        item.divider ? (
          <div key={i} className="ctx-divider" />
        ) : item.submenu ? (
          <SubmenuItem key={i} item={item} onClose={onClose} />
        ) : (
          <button
            key={i}
            className={`ctx-item${item.danger ? " danger" : ""}`}
            onClick={() => { item.onClick(); onClose(); }}
          >
            <span className="ctx-icon">{item.icon}</span>
            {item.label}
          </button>
        )
      )}
    </div>,
    document.body
  );
}
