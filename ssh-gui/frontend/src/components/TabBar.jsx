export default function TabBar({
  tabs,
  activeTab,
  onSelect,
  onClose,
}) {

  return (
    <div className="tabbar">

      {tabs.length === 0 && (

        <div
          style={{
            padding: "0 12px",
            color: "#6b7280",
            fontSize: "14px",
          }}
        >
          No active sessions
        </div>

      )}

      {tabs.map((tab) => (

        <div
          key={tab.id}
          className={
            activeTab === tab.id
              ? "tab active"
              : "tab"
          }
          onClick={() =>
            onSelect(tab.id)
          }
        >

          <span
            style={{
              width: "8px",
              height: "8px",
              borderRadius: "50%",
              background:
                tab.color ||
                "#22c55e",
              display: "inline-block",
            }}
          />

          <span>
            {tab.title}
          </span>

          {onClose && (

            <button
              onClick={(e) => {

                e.stopPropagation();

                onClose(tab.id);

              }}
              style={{
                border: "none",
                background: "transparent",
                cursor: "pointer",
                color: "#6b7280",
                fontSize: "14px",
                padding: 0,
                marginLeft: "4px",
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