import { useState } from "react";

const TABS = [
  { id: "logs", label: "Logs" },
  { id: "transfers", label: "Transfers" },
  { id: "tunnels", label: "Tunnels" },
  { id: "tasks", label: "Tasks" },
];

const LOGS = [
  { time: "11:02:15", type: "SSH",  msg: "Connected to root@186.64.121.8", cls: "success" },
  { time: "11:02:16", type: "SFTP", msg: "Listed directory /home/root", cls: "" },
  { time: "11:02:17", type: "SSH",  msg: "Command executed: ls -la", cls: "" },
  { time: "11:02:18", type: "SFTP", msg: "Download completed: docker-compose.yml (2.1 KB)", cls: "success" },
  { time: "11:02:20", type: "SSH",  msg: "Disconnected", cls: "warn" },
];

export default function BottomPanel() {
  const [activeTab, setActiveTab] = useState("logs");

  return (
    <div className="bottom-panel">
      <div className="bottom-tabs">
        {TABS.map(t => (
          <div
            key={t.id}
            className={activeTab === t.id ? "bottom-tab active" : "bottom-tab"}
            onClick={() => setActiveTab(t.id)}
          >
            {t.label}
          </div>
        ))}
        <div className="bottom-tab-actions">
          <button className="bottom-action-btn">🗑 Clear</button>
          <button className="bottom-action-btn">↑ Export</button>
          <button className="bottom-action-btn" style={{ border: "none", fontSize: "16px", padding: "0 6px" }}>⋯</button>
        </div>
      </div>
      <div className="bottom-content">
        {activeTab === "logs" && LOGS.map((l, i) => (
          <div className="log-line" key={i}>
            <span className="log-time">{l.time}</span>
            <span className={`log-badge ${l.type.toLowerCase()}`}>{l.type}</span>
            <span className={`log-msg ${l.cls}`}>{l.msg}</span>
          </div>
        ))}
        {activeTab !== "logs" && (
          <div style={{ padding: "20px 16px", color: "var(--text-muted)", fontSize: "12px" }}>
            No data available.
          </div>
        )}
      </div>
    </div>
  );
}
