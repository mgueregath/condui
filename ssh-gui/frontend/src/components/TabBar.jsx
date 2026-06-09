export default function TabBar({
  tabs,
  activeTab,
  onSelect,
  onClose,
  onReconnect,
  connectingId,
}) {
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
          No active sessions
        </div>
      )}
      {tabs.map((tab) => (
        <div
          key={tab.id}
          className={
            "tab" +
            (activeTab === tab.id ? " active" : "") +
            (tab.disconnected ? " disconnected" : "")
          }
          onClick={() => onSelect(tab.id)}
        >
          {tab.disconnected && <span className="tab-disconnect-dot" />}
          <span style={{ color: tab.color || "var(--bg-hover)" }}>
            {tab.title}
          </span>
          {tab.disconnected && onReconnect && (
            <button
              className="tab-reconnect"
              title="Reconnect"
              disabled={connectingId === tab.connectionId}
              onClick={(e) => {
                e.stopPropagation();
                onReconnect(tab);
              }}
            >
              ↺
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
              ×
            </button>
          )}
        </div>
      ))}
    </div>
  );
}
