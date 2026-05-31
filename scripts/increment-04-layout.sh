#!/bin/bash

set -e

echo "================================="
echo "Incremento 4 - Layout Profesional"
echo "================================="

cd ssh-gui/frontend

pnpm add react-resizable-panels react-icons

mkdir -p src/components
mkdir -p src/styles

cat > src/components/Sidebar.jsx <<'EOF'
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
EOF

cat > src/components/TabBar.jsx <<'EOF'
export default function TabBar({
  tabs,
  activeTab,
  onSelect,
}) {

  return (
    <div className="tabbar">

      {tabs.map(tab => (

        <div
          key={tab.id}
          className={
            activeTab === tab.id
              ? "tab active"
              : "tab"
          }
          onClick={() => onSelect(tab.id)}
        >
          {tab.title}
        </div>

      ))}

    </div>
  );
}
EOF

cat > src/components/BottomPanel.jsx <<'EOF'
export default function BottomPanel() {

  return (
    <div className="bottom-panel">

      <div className="bottom-tabs">

        <div>Logs</div>

        <div>Transferencias</div>

        <div>Túneles</div>

      </div>

      <div className="bottom-content">
        Próximamente...
      </div>

    </div>
  );
}
EOF

cat > src/components/Layout.css <<'EOF'
:root {

  --bg: #1e1f22;

  --sidebar: #25262b;

  --panel: #2b2d31;

  --border: #3a3c42;

  --text: #d9d9d9;

  --muted: #8b8d93;

}

html,
body,
#root {

  margin: 0;
  padding: 0;

  width: 100%;
  height: 100%;

}

body {
  background: var(--bg);
  color: var(--text);
}

.sidebar {

  height: 100%;

  display: flex;
  flex-direction: column;

  background: var(--sidebar);

}

.sidebar-title {

  padding: 16px;

  font-weight: bold;

  border-bottom: 1px solid var(--border);

}

.sidebar-list {

  flex: 1;

  overflow: auto;

}

.sidebar-item {

  padding: 10px 16px;

  cursor: pointer;

}

.sidebar-item:hover {

  background: #303136;

}

.sidebar-item.active {

  background: #3b3d44;

}

.new-connection-btn {

  margin: 12px;

  padding: 10px;

}

.tabbar {

  display: flex;

  border-bottom: 1px solid var(--border);

}

.tab {

  padding: 10px 16px;

  cursor: pointer;

}

.tab.active {

  background: #303136;

}

.bottom-panel {

  height: 100%;

  display: flex;

  flex-direction: column;

}

.bottom-tabs {

  display: flex;

  gap: 20px;

  padding: 8px;

  border-bottom: 1px solid var(--border);

}

.bottom-content {

  flex: 1;

  padding: 12px;

}
EOF

echo ""
echo "Componentes creados:"
echo ""
echo "src/components/Sidebar.jsx"
echo "src/components/TabBar.jsx"
echo "src/components/BottomPanel.jsx"
echo "src/components/Layout.css"
echo ""
echo "Incremento 4 preparado."