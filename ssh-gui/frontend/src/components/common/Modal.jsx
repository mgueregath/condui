import { createPortal } from "react-dom";

export default function Modal({ open, children, onClose = () => {}, className = "" }) {
  if (!open) {
    return null;
  }

  return createPortal(
    <div className="modal-overlay" onClick={onClose}>
      <div className={`modern-modal ${className}`.trim()} onClick={(e) => e.stopPropagation()}>
        {children}
      </div>
    </div>,
    document.body,
  );
}
