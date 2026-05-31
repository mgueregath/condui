import { useMemo, useState } from "react";
import FolderNode from "./FolderNode";
import ConnectionNode from "./ConnectionNode";

export default function ConnectionDrawer({
  folders, connections, expandedFolders,
  onToggleFolder, onOpenConnection, onEditConnection, onDeleteConnection,
  onEditFolder, onDeleteFolder,
}) {
  const [search, setSearch] = useState("");

  const filtered = useMemo(() => {
    const v = search.trim().toLowerCase();
    if (!v) return connections;
    return connections.filter(c =>
      c.name?.toLowerCase().includes(v) ||
      c.host?.toLowerCase().includes(v) ||
      c.username?.toLowerCase().includes(v)
    );
  }, [search, connections]);

  const ungrouped = filtered.filter(c => !c.folderId || c.folderId === "");

  return (
    <div style={{ height: "100%", display: "flex", flexDirection: "column" }}>
      <div className="drawer-actions">
        <button className="drawer-btn" onClick={() => {}}>+ Folder</button>
        <button className="drawer-btn" onClick={() => {}}>+ Connection</button>
      </div>
      <input
        className="drawer-search"
        placeholder="Search connections..."
        value={search}
        onChange={e => setSearch(e.target.value)}
      />
      <div className="drawer-count">{filtered.length} connection(s)</div>
      <div className="drawer-body">
        {ungrouped.length > 0 && (
          <div>
            <div className="drawer-group-label">Connections</div>
            {ungrouped.map(c => (
              <ConnectionNode
                key={c.id} connection={c}
                onOpen={onOpenConnection} onEdit={onEditConnection} onDelete={onDeleteConnection}
              />
            ))}
          </div>
        )}
        {folders.map(folder => {
          const fConns = filtered.filter(c => c.folderId === folder.id);
          if (fConns.length === 0 && search) return null;
          return (
            <FolderNode
              key={folder.id} folder={folder}
              expanded={expandedFolders.includes(folder.id)}
              onToggle={onToggleFolder} onEdit={onEditFolder} onDelete={onDeleteFolder}
            >
              {fConns.map(c => (
                <ConnectionNode
                  key={c.id} connection={c}
                  onOpen={onOpenConnection} onEdit={onEditConnection} onDelete={onDeleteConnection}
                />
              ))}
            </FolderNode>
          );
        })}
      </div>
    </div>
  );
}
