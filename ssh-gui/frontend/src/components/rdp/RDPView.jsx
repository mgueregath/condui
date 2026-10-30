import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import { closeRDP, getRDPFrame } from "../../services/rdp";
import { bindRDPInput } from "./rdpInput";
import "./RDPView.css";

export default function RDPView({ session, active, setTabs }) {
  const { t } = useTranslation();
  const canvas = useRef(null);
  const input = useRef(null);
  const [ready, setReady] = useState(false);
  const id = session.id;
  useEffect(() => {
    let cleanup;
    const start = setTimeout(() => {
      let disposed = false, finished = false, drawing = false, pending, sequence = 0;
      const target = canvas.current;
      const finish = () => {
        if (disposed || finished) return;
        finished = true;
        setTabs(tabs => tabs.map(tab => tab.id === id ? { ...tab, disconnected: true } : tab));
        void closeRDP(id).catch(() => {});
      };
      const draw = () => {
        if (disposed || drawing || !pending) return;
        const frame = pending; pending = undefined; drawing = true;
        const image = new Image();
        image.onload = () => {
          if (disposed) return;
          target.width = frame.width; target.height = frame.height;
          target.getContext("2d").drawImage(image, 0, 0);
          setReady(true); drawing = false; draw();
        };
        image.onerror = finish;
        image.src = `data:image/png;base64,${frame.image}`;
      };
      const receive = frame => {
        if (disposed || finished || !frame || frame.sessionId !== id) return;
        if (frame.cursor) target.style.cursor = frame.cursor;
        if (!frame.image || frame.sequence <= sequence) return;
        sequence = frame.sequence; pending = frame; draw();
      };
      const offFrame = Events.On("rdp-frame", event => receive(event.data));
      const offCursor = Events.On("rdp-cursor", event => {
        if (event.data?.sessionId === id && !disposed) target.style.cursor = event.data.cursor || "default";
      });
      input.current = bindRDPInput(target, id, finish);
      getRDPFrame(id).then(receive).catch(finish);
      cleanup = () => {
        disposed = true; offFrame(); offCursor(); input.current?.dispose(); input.current = null;
        void closeRDP(id).catch(() => {});
      };
    }, 0);
    return () => { clearTimeout(start); cleanup?.(); };
  }, [id, setTabs]);
  useEffect(() => {
    if (!active) { input.current?.release(); canvas.current?.blur(); }
  }, [active]);
  return <section className="rdp-view" style={{ display: active ? "flex" : "none" }} aria-label={t("connection.type.rdp")}>
    {!ready && <div className="rdp-status" role="status">{t("connection.rdp.connecting")}</div>}
    <canvas ref={canvas} width="1280" height="720" tabIndex={0} aria-label={t("connection.rdp.screen")} />
  </section>;
}
