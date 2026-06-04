export default function TabBar({ tabs, activeTab, onSelect, onClose }) {
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
          className={activeTab === tab.id ? "tab active" : "tab"}
          onClick={() => onSelect(tab.id)}
        >
          <span style={{ color: tab.color || "var(--bg-hover)" }}>
            {tab.title}
          </span>
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
      {/* 
            <button className="tab-add" title="New tab">+</button>

*/}
    </div>
  );
}
