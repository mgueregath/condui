import { connectionKey } from "./connections/connectionForm";
import ProtocolIcon from "./connections/ProtocolIcon";
import { useTranslation } from "react-i18next";
import { FaTerminal, FaRedoAlt, FaTimes } from "react-icons/fa";

export default function TabBar({
  tabs,
  activeTab,
  onSelect,
  onClose,
  onReconnect,
  onOpenLocalTerminal,
  connectingIds,
}) {
  const { t } = useTranslation();
  return (
    <div className="tabbar">
      {tabs.length === 0 && (
        <div
          style={{
            padding: "0 8px",
            color: "var(--text-muted)",
            fontSize: "12px",
          }}
        >
          {t("tabs.noActiveSessions")}
        </div>
      )}
      {tabs.map((tab) => (
        <div
          key={tab.id}
          className={
            "tab" +
            (activeTab === tab.id ? " active" : "") +
            (tab.disconnected ? " disconnected" : "") +
            (tab.type === "db" ? " tab-db" : "")
          }
          onClick={() => onSelect(tab.id)}
        >
          {tab.type !== "db" && <ProtocolIcon protocol={tab.type === "vnc" ? "vnc" : "ssh"} />}
          {tab.disconnected && <span className="tab-disconnect-dot" />}
          {tab.type === "db" && (
            <span style={{ fontSize: "10px", opacity: 0.7, marginRight: "3px" }}>⬡</span>
          )}
          <span style={{ color: tab.type === "db" ? "#818cf8" : (tab.color || "var(--bg-hover)") }}>
            {tab.title}
          </span>
          {tab.disconnected && tab.connectionId && onReconnect && (
            <button
              className="tab-reconnect"
              title={t("tabs.reconnect")}
              disabled={!!connectingIds[connectionKey(tab.connectionId, tab.type || "ssh")]}
              onClick={(e) => {
                e.stopPropagation();
                onReconnect(tab);
              }}
            >
              <FaRedoAlt />
            </button>
          )}
          {onClose && (
            <button
              className="tab-close"
              onClick={(e) => {
                e.stopPropagation();
                onClose(tab.id);
              }}
            >
              <FaTimes />
            </button>
          )}
        </div>
      ))}
      {onOpenLocalTerminal && (
        <button
          className="tab-add"
          title={t("app.openLocalTerminal")}
          onClick={onOpenLocalTerminal}
          disabled={!!connectingIds["__local"]}
        >
          <FaTerminal />
        </button>
      )}
    </div>
  );
}
