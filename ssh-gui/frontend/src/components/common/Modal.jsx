import { createPortal } from "react-dom";

export default function Modal({ open, children, onClose }) {
  if (!open) return null;
  return createPortal(
    <div className="modal-overlay" onClick={onClose}>
      <div className="modern-modal" onClick={e => e.stopPropagation()}>
        {children}
      </div>
    </div>,
    document.body
  );
}