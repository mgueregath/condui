import { useState, useEffect, useRef } from "react";
import { Events } from "@wailsio/runtime";
import { StartDockerLogs, StopDockerLogs } from "../../bindings/ssh-gui/app";

const ANSI_RE = /\x1B\[[0-9;]*[mGKHFABCDJsunhl]|\x1B\][^\x07]*\x07|\x1B[()][AB012]|\r/g;

export default function DockerLogModal({ open, sessionId, containerId, containerName, onClose }) {
  const [lines, setLines] = useState([]);
  const [following, setFollowing] = useState(true);
  const [ended, setEnded] = useState(false);
  const [pos, setPos] = useState(null); // null = centered on first open
  const bodyRef = useRef(null);
  const followingRef = useRef(true);
  const dragRef = useRef({ active: false, startX: 0, startY: 0, origX: 0, origY: 0 });
  const panelRef = useRef(null);

  // Center on first open
  useEffect(() => {
    if (open && pos === null) {
      setPos({
        x: Math.max(0, Math.round(window.innerWidth / 2 - 480)),
        y: Math.max(0, Math.round(window.innerHeight / 2 - 320)),
      });
    }
  }, [open]);

  useEffect(() => {
    if (!open || !containerId || !sessionId) return;

    setLines([]);
    setEnded(false);
    setFollowing(true);
    followingRef.current = true;

    StartDockerLogs(sessionId, containerId).catch(console.error);

    const offLine = Events.On("docker-log-" + containerId, (line) => {
      setLines((prev) => {
        const next = [...prev, String(line).replace(ANSI_RE, "")];
        return next.length > 2000 ? next.slice(next.length - 2000) : next;
      });
    });

    const offEnd = Events.On("docker-log-end-" + containerId, () => setEnded(true));

    return () => {
      offLine?.();
      offEnd?.();
      StopDockerLogs(sessionId, containerId);
    };
  }, [open, sessionId, containerId]);

  useEffect(() => {
    if (followingRef.current && bodyRef.current) {
      bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
    }
  }, [lines]);

  // Drag logic
  useEffect(() => {
    const onMove = (e) => {
      if (!dragRef.current.active) return;
      setPos({
        x: dragRef.current.origX + e.clientX - dragRef.current.startX,
        y: dragRef.current.origY + e.clientY - dragRef.current.startY,
      });
    };
    const onUp = () => { dragRef.current.active = false; };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    return () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
  }, []);

  const startDrag = (e) => {
    if (e.button !== 0) return;
    e.preventDefault();
    dragRef.current = { active: true, startX: e.clientX, startY: e.clientY, origX: pos.x, origY: pos.y };
  };

  const handleScroll = () => {
    if (!bodyRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = bodyRef.current;
    const atBottom = scrollHeight - scrollTop - clientHeight < 48;
    followingRef.current = atBottom;
    setFollowing(atBottom);
  };

  const scrollToBottom = () => {
    followingRef.current = true;
    setFollowing(true);
    bodyRef.current?.scrollTo({ top: bodyRef.current.scrollHeight, behavior: "smooth" });
  };

  if (!open || pos === null) return null;

  return (
    <div
      ref={panelRef}
      className="docker-log-panel"
      style={{ left: pos.x, top: pos.y }}
    >
      <div className="docker-log-header" onMouseDown={startDrag}>
        <div style={{ display: "flex", alignItems: "center", gap: "10px", minWidth: 0 }}>
          <span className="docker-log-title">{containerName}</span>
          {ended ? (
            <span className="docker-log-badge stopped">■ STOPPED</span>
          ) : (
            <span className="docker-log-badge live">● LIVE</span>
          )}
          <span className="docker-log-count">{lines.length} líneas</span>
        </div>
        <div style={{ display: "flex", gap: "6px", flexShrink: 0 }}>
          <button className="docker-log-btn" onClick={() => setLines([])}>Limpiar</button>
          {!following && !ended && (
            <button className="docker-log-btn accent" onClick={scrollToBottom}>↓ Seguir</button>
          )}
          <button className="docker-log-btn danger" onClick={onClose}>✕</button>
        </div>
      </div>

      <div className="docker-log-body" ref={bodyRef} onScroll={handleScroll}>
        {lines.length === 0 ? (
          <span className="docker-log-empty">Conectando al contenedor...</span>
        ) : (
          lines.map((line, i) => (
            <div key={i} className="docker-log-line">{line || " "}</div>
          ))
        )}
        {ended && lines.length > 0 && (
          <div className="docker-log-end-marker">— fin del log —</div>
        )}
      </div>
    </div>
  );
}
