import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import RFB from "@novnc/novnc";
import { closeVNC } from "../../services/vnc";
import "./VNCView.css";

export default function VNCView({ session, active, setTabs }) {
  const { t } = useTranslation();
  const screen = useRef(null);
  const [status, setStatus] = useState("connecting");
  const { id, url, token } = session;

  useEffect(() => {
    let dispose;
    const start = setTimeout(() => {
      let disposed = false;
      let finished = false;
      let viewer;
      let timeout;
      const socket = new WebSocket(url);
      const finish = (state) => {
        if (disposed || finished) return;
        finished = true;
        clearTimeout(timeout);
        setStatus(previous => previous === "error" ? previous : state);
        setTabs(tabs => tabs.map(tab => tab.id === id ? { ...tab, disconnected: true } : tab));
        void closeVNC(id).catch(() => {});
      };
      timeout = setTimeout(() => { socket.close(); finish("error"); }, 20000);
      socket.addEventListener("open", () => {
        if (disposed) return;
        // The ephemeral credential is sent as data, never in the URL.
        socket.send(token);
        try {
          viewer = new RFB(screen.current, socket);
          viewer.scaleViewport = true;
          viewer.resizeSession = false;
          // Keep a visible pointer when the server sends an empty cursor.
          viewer.showDotCursor = true;
          viewer.addEventListener("connect", () => {
            clearTimeout(timeout);
            if (!disposed) setStatus("connected");
          });
          viewer.addEventListener("disconnect", event => finish(event.detail.clean ? "disconnected" : "error"));
          viewer.addEventListener("securityfailure", () => finish("error"));
        } catch {
          socket.close();
          finish("error");
        }
      }, { once: true });
      socket.addEventListener("error", () => finish("error"));
      socket.addEventListener("close", event => finish(event.wasClean ? "disconnected" : "error"));
      dispose = () => {
        disposed = true;
        clearTimeout(timeout);
        viewer?.disconnect();
        socket.close();
        void closeVNC(id).catch(() => {});
      };
    }, 0);
    return () => { clearTimeout(start); dispose?.(); };
  }, [id, url, token, setTabs]);

  return (
    <section className="vnc-view" style={{ display: active ? "flex" : "none" }} aria-label={t("connection.type.vnc")}>
      {/* <div className="vnc-status" role="status">{t(`connection.vnc.${status}`)}</div> */}
      <div className="vnc-screen" ref={screen} />
    </section>
  );
}
