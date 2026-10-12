import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import "./ResourceTooltip.css";

export default function ResourceTooltip({ children, content }) {
  const id = useId();
  const trigger = useRef(null);
  const timer = useRef(null);
  const [position, setPosition] = useState(null);

  const hide = () => {
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setPosition(null), 120);
  };
  const show = () => {
    clearTimeout(timer.current);
    const rect = trigger.current.getBoundingClientRect();
    const width = Math.min(380, window.innerWidth - 16);
    setPosition({
      left: Math.max(8, Math.min(rect.left, window.innerWidth - width - 8)),
      bottom: window.innerHeight - rect.top + 8,
      width,
      maxHeight: Math.max(80, rect.top - 16),
    });
  };

  useEffect(() => () => clearTimeout(timer.current), []);
  useEffect(() => {
    if (!position) return;
    const close = () => setPosition(null);
    const escape = (event) => { if (event.key === "Escape") close(); };
    window.addEventListener("resize", close);
    window.addEventListener("keydown", escape);
    return () => {
      window.removeEventListener("resize", close);
      window.removeEventListener("keydown", escape);
    };
  }, [position]);

  return (
    <>
      <div ref={trigger} className="resource-tooltip-trigger" tabIndex={0}
        aria-describedby={position ? id : undefined}
        onMouseEnter={show} onMouseLeave={hide} onFocus={show} onBlur={hide}>
        {children}
      </div>
      {position && createPortal(
        <div id={id} role="tooltip" className="resource-tooltip" style={position}
          onMouseEnter={() => clearTimeout(timer.current)} onMouseLeave={hide}>
          {content}
        </div>, document.body,
      )}
    </>
  );
}
