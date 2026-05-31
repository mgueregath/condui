export default function Sidebar({
  tabs,
  onNewConnection,
  onSelect,
  activeTab,
}) {

  return (
    <div className="sidebar">

      <div className="sidebar-title">
        SERVIDORES
      </div>

      <div className="sidebar-list">

        {tabs.map(tab => (

          <div
            key={tab.id}
            className={
              activeTab === tab.id
                ? "sidebar-item active"
                : "sidebar-item"
            }
            onClick={() => onSelect(tab.id)}
          >
            {tab.title}
          </div>

        ))}

      </div>

      <button
        className="new-connection-btn"
        onClick={onNewConnection}
      >
        + Nueva conexión
      </button>

    </div>
  );
}
