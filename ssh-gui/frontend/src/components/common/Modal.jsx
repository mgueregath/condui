import { useId, useLayoutEffect, useRef } from "react";
import { createPortal } from "react-dom";

const modalStack = [];
const focusable = 'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], summary, [tabindex="0"]';

export default function Modal({ open, children, onClose = () => {}, className = "" }) {
  const dialog = useRef(null);
  const close = useRef(onClose);
  close.current = onClose;
  const titleId = useId();

  useLayoutEffect(() => {
    if (!open) return;
    const element = dialog.current;
    const previous = document.activeElement;
    const title = element.querySelector("h1, h2, h3");
    if (title) {
      if (!title.id) title.id = titleId;
      element.setAttribute("aria-labelledby", title.id);
    }
    modalStack.push(element);
    const targets = () => [...element.querySelectorAll(focusable)].filter(node => node.getClientRects().length && !node.matches(":disabled"));
    (targets()[0] || element).focus();
    const keydown = (event) => {
      if (modalStack.at(-1) !== element) return;
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        close.current();
      }
      if (event.key === "Tab") {
        const nodes = targets();
        const first = nodes[0] || element;
        const last = nodes.at(-1) || element;
        if (!nodes.length || !element.contains(document.activeElement) || (event.shiftKey ? document.activeElement === first : document.activeElement === last)) {
          event.preventDefault();
          (event.shiftKey ? last : first).focus();
        }
      }
    };
    document.addEventListener("keydown", keydown, true);
    return () => {
      document.removeEventListener("keydown", keydown, true);
      const wasTop = modalStack.at(-1) === element;
      const index = modalStack.indexOf(element);
      if (index >= 0) modalStack.splice(index, 1);
      if (wasTop && previous?.isConnected) previous.focus();
    };
  }, [open, titleId]);

  if (!open) return null;
  return createPortal(
    <div className="modal-overlay" onClick={onClose}>
      <div ref={dialog} role="dialog" aria-modal="true" tabIndex={-1} className={`modern-modal ${className}`.trim()} onClick={(e) => e.stopPropagation()}>
        {children}
      </div>
    </div>,
    document.body,
  );
}
