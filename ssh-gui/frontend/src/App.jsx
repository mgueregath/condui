import { useEffect, useRef, useState } from "react";
import { Terminal } from "xterm";
import { FitAddon } from "xterm-addon-fit";
import "xterm/css/xterm.css";
import conduiLogo from "./assets/images/condui-transparent.png";

import { EventsOn } from "../wailsjs/runtime/runtime";
import RemoteFileTree from "./components/files/RemoteFileTree";
import {
  ConnectSSH,
  SendInput,
  ResizeTerminal,
  CreateFolder,
  CreateConnection,
  UpdateFolder,
  UpdateConnection,
  DeleteConnection,
  DeleteFolder,
  UploadFile,
  EditTunnel,
} from "../wailsjs/go/main/App";

import { BsUpload,  } from "react-icons/bs";
import { GrRefresh } from "react-icons/gr";
import { MdUploadFile } from "react-icons/md";


import TabBar from "./components/TabBar";
import BottomPanel from "./components/BottomPanel";
import { useConnections } from "./hooks/useConnections";
import Modal from "./components/common/Modal";
import ConnectionNode from "./components/connections/ConnectionNode";
import FolderNode from "./components/connections/FolderNode";
import FolderModal from "./components/connections/FolderModal";
import ConnectionModal from "./components/connections/ConnectionModal";
import AssignFolderModal from "./components/connections/AssignFolderModal";
import ContextMenu from "./components/connections/ContextMenu";
import "./components/Layout.css";

function LeftSidebar({
  folders,
  connections,
  expandedFolders,
  onToggleFolder,
  onOpenConnection,
  onNewConnection,
  onNewFolder,
  onEditConnection,
  onDeleteConnection,
  onAssignFolder,
  onEditFolder,
  onDeleteFolder,
  activeSessionId,
}) {
  const [search, setSearch] = useState("");
  const [emptyCtx, setEmptyCtx] = useState(null);

  const handleEmptyAreaContextMenu = (e) => {
    if (e.target === e.currentTarget) {
      e.preventDefault();
      setEmptyCtx({ x: e.clientX, y: e.clientY });
    }
  };

  const filtered = connections.filter(
    (c) => !search || c.name?.toLowerCase().includes(search.toLowerCase()),
  );

  return (
    <div className="sidebar">
      <div className="app-logo">
        
        <img className="logo-image" src={conduiLogo} />
      </div>
      <div className="sidebar-header">
        <span className="sidebar-title">Connections</span>
        <div className="sidebar-header-actions">
          <button
            className="sidebar-icon-btn"
            title="New connection"
            onClick={onNewConnection}
          >
            +
          </button>
          <button
            className="sidebar-icon-btn"
            title="New folder"
            onClick={onNewFolder}
          >
            📁
          </button>
        </div>
      </div>

      <input
        className="sidebar-search"
        placeholder="Search connections..."
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />

      <div className="sidebar-list" onContextMenu={handleEmptyAreaContextMenu}>
        {folders.map((folder) => {
          const fConns = filtered.filter((c) => c.folderId === folder.id);
          if (fConns.length === 0 && search) return null;
          return (
            <FolderNode
              key={folder.id}
              folder={folder}
              expanded={expandedFolders.includes(folder.id)}
              onToggle={onToggleFolder}
              onEdit={onEditFolder}
              onDelete={onDeleteFolder}
            >
              {fConns.map((c) => (
                <ConnectionNode
                  key={c.id}
                  connection={c}
                  folders={folders}
                  onOpen={onOpenConnection}
                  onEdit={onEditConnection}
                  onDelete={onDeleteConnection}
                  onAssignFolder={onAssignFolder}
                />
              ))}
            </FolderNode>
          );
        })}

        {filtered.filter((c) => !c.folderId || c.folderId === "").length >
          0 && (
          <div className="sidebar-group">
            {folders.length > 0 && (
              <div className="sidebar-group-label">Ungrouped</div>
            )}
            {filtered
              .filter((c) => !c.folderId || c.folderId === "")
              .map((c) => (
                <ConnectionNode
                  key={c.id}
                  connection={c}
                  folders={folders}
                  onOpen={onOpenConnection}
                  onEdit={onEditConnection}
                  onDelete={onDeleteConnection}
                  onAssignFolder={onAssignFolder}
                />
              ))}
          </div>
        )}

        {filtered.length === 0 && (
          <div
            style={{
              padding: "24px 12px",
              textAlign: "center",
              color: "var(--text-muted)",
              fontSize: "12px",
            }}
          >
            {search ? "No results" : "No connections yet"}
          </div>
        )}
      </div>

      {emptyCtx && (
        <ContextMenu
          x={emptyCtx.x}
          y={emptyCtx.y}
          items={[
            { icon: "+", label: "New connection", onClick: onNewConnection },
            { icon: "📁", label: "New folder", onClick: onNewFolder },
          ]}
          onClose={() => setEmptyCtx(null)}
        />
      )}
    </div>
  );
}

function App() {
  const terminalRef = useRef(null);
  const termRef = useRef(null);
  const terminalBuffers = useRef({});
  const fileTreeRef = useRef(null);

  const [tabs, setTabs] = useState([]);
  const [activeTab, setActiveTab] = useState(null);
  const { folders, connections, reload } = useConnections();

  const [folderModalOpen, setFolderModalOpen] = useState(false);
  const [connectionModalOpen, setConnectionModalOpen] = useState(false);
  const [assignFolderModalOpen, setAssignFolderModalOpen] = useState(false);
  const [expandedFolders, setExpandedFolders] = useState([]);
  const [editingFolder, setEditingFolder] = useState(null);
  const [editingConnection, setEditingConnection] = useState(null);
  const [assigningConnection, setAssigningConnection] = useState(null);

  const openNewConnection = () => {
    setEditingConnection(null);
    setConnectionModalOpen(true);
  };
  const openNewFolder = () => {
    setEditingFolder(null);
    setFolderModalOpen(true);
  };

  const [editor, setEditor] = useState({
    open: false,
    path: "",
    content: "",
    modified: false,
  });

  useEffect(() => {
    const term = new Terminal({
      cursorBlink: true,
      convertEol: true,
      theme: {
        background: "#111827",
        foreground: "#d1d5db",
        cursor: "#6366f1",
        selectionBackground: "rgba(99,102,241,0.3)",
        black: "#1f2937",
        red: "#f87171",
        green: "#86efac",
        yellow: "#fcd34d",
        blue: "#93c5fd",
        magenta: "#c084fc",
        cyan: "#67e8f9",
        white: "#e5e7eb",
        brightBlack: "#374151",
        brightRed: "#f87171",
        brightGreen: "#86efac",
        brightYellow: "#fcd34d",
        brightBlue: "#93c5fd",
        brightMagenta: "#c084fc",
        brightCyan: "#67e8f9",
        brightWhite: "#f9fafb",
      },
      fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
      fontSize: 13,
      lineHeight: 1.45,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(terminalRef.current);
    fitAddon.fit();
    termRef.current = term;

    term.onData((data) => {
      if (!activeTab) return;
      SendInput(activeTab, data);
    });

    EventsOn("terminal-output", (payload) => {
      if (!terminalBuffers.current[payload.sessionId])
        terminalBuffers.current[payload.sessionId] = "";
      terminalBuffers.current[payload.sessionId] += payload.data;
      if (payload.sessionId !== activeTab) return;
      term.write(payload.data);
    });

    const resizeHandler = () => {
      fitAddon.fit();
      if (activeTab && termRef.current)
        ResizeTerminal(activeTab, term.rows, term.cols);
    };
    window.addEventListener("resize", resizeHandler);
    return () => {
      window.removeEventListener("resize", resizeHandler);
      term.dispose();
    };
  }, [activeTab]);

  useEffect(() => {
    if (!activeTab || !termRef.current) return;
    termRef.current.clear();
    termRef.current.write(terminalBuffers.current[activeTab] || "");
    ResizeTerminal(activeTab, termRef.current.rows, termRef.current.cols);
  }, [activeTab]);

  const activeTabData = tabs.find((t) => t.id === activeTab);

  const handleOpenConnection = async (c) => {
    const existing = tabs.find((t) => t.connectionId === c.id);
    if (existing) {
      setActiveTab(existing.id);
      return;
    }
    try {
      const sessionId = await ConnectSSH(c.id);
      setTabs((prev) => [
        ...prev,
        { id: sessionId, connectionId: c.id, title: c.name, color: c.color },
      ]);
      setActiveTab(sessionId);
    } catch (err) {
      console.error(err);
    }
  };

const uploadFile = async () => {
    // Si no hay una sesión SSH activa, detenemos la operación
    if (!activeTab) {
      alert("Por favor, selecciona una sesión activa primero.");
      return;
    }

    const currentRemotePath = fileTreeRef.current?.currentPath || "/";

    try {
      await UploadFile(activeTab, currentRemotePath); 
      
      fileTreeRef.current?.refresh(); 
    } catch (err) {
      if (err.includes("cancelada")) {
        console.log("Subida cancelada por el usuario.");
        return;
      }
      console.error("Error al subir archivo:", err);
      alert("Error al subir archivo: " + err);
    }
  };

  return (
    <div className="app-shell">
      <div className="topbar">
        <span className="topbar-logo"></span>
        </div>
      {/* TOP BAR */}
      {/*
      <div className="topbar">
        <span className="topbar-logo">ModernTerm</span>
        <div className="topbar-actions">
          <button className="topbar-btn primary" onClick={openNewConnection}>
            + New Connection
          </button>
          <button className="topbar-btn">↑ Upload</button>
          <button className="topbar-btn">↓ Download</button>
          <button className="topbar-btn">⇌ Tunnels</button>
          <button className="topbar-btn">⚙ Settings</button>
        </div>
        <div className="topbar-search-wrap">
          <svg
            width="13"
            height="13"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
          >
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.35-4.35" />
          </svg>
          <input className="topbar-search" placeholder="Search (⌘K)" />
        </div>
        <div className="topbar-right">
          <button className="topbar-icon-btn">🔔</button>
          <button className="topbar-icon-btn">⊞</button>
          <div className="topbar-avatar">M</div>
        </div>
      </div>
      */}

      {/* MAIN */}
      <div className="main-content">
        <LeftSidebar
          folders={folders}
          connections={connections}
          expandedFolders={expandedFolders}
          onToggleFolder={(id) =>
            setExpandedFolders((prev) =>
              prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
            )
          }
          onOpenConnection={handleOpenConnection}
          onNewConnection={openNewConnection}
          onNewFolder={openNewFolder}
          onEditConnection={(c) => {
            setEditingConnection(c);
            setConnectionModalOpen(true);
          }}
          onDeleteConnection={async (c) => {
            try {
              await DeleteConnection(c.id);
              await reload();
            } catch (err) {
              console.error(err);
            }
          }}
          onAssignFolder={(c) => {
            setAssigningConnection(c);
            setAssignFolderModalOpen(true);
          }}
          onEditFolder={(f) => {
            setEditingFolder(f);
            setFolderModalOpen(true);
          }}
          onDeleteFolder={async (f) => {
            try {
              await DeleteFolder(f.id);
              await reload();
            } catch (err) {
              console.error(err);
            }
          }}
          activeSessionId={activeTab}
        />

        {/* Main card — tarjeta única con solapas arriba */}
        <div className="main-card">
          <TabBar
            tabs={tabs}
            activeTab={activeTab}
            onSelect={setActiveTab}
            onClose={(tabId) => {
              setTabs((prev) => prev.filter((t) => t.id !== tabId));
              if (activeTab === tabId) {
                const remaining = tabs.filter((t) => t.id !== tabId);
                setActiveTab(remaining.length ? remaining[0].id : null);
              }
            }}
          />
          <div
            className="card-inner"
            style={{ display: tabs.length > 0 ? "flex" : "none" }}
          >
            <div className="card-mid">
              <div className="files-panel">
                <div className="files-header">
                  Remote Files
                  <div className="files-header-actions">
                    <button
                      className="files-header-btn"
                      title="Subir archivo aquí"
                      onClick={uploadFile}
                      style={{
                        background: "var(--primary)"
                      }}
                    >
                      <MdUploadFile />
                    </button>
                    <button
                      className="files-header-btn"
                      onClick={() => fileTreeRef.current?.refresh()}
                    >
                      <GrRefresh />
                    </button>
                    <button className="files-header-btn">⋯</button>
                  </div>
                </div>
                <RemoteFileTree sessionId={activeTab} ref={fileTreeRef} />
              </div>
              <div className="terminal-card">
                <div className="terminal-titlebar">
                  <span className="terminal-titlebar-dot" />
                  <span className="terminal-title">
                    {activeTabData ? activeTabData.title : "No Session"}
                  </span>
                  <div className="terminal-titlebar-actions">
                    <button className="terminal-titlebar-btn">+</button>
                    <button className="terminal-titlebar-btn">⊞</button>
                    <button className="terminal-titlebar-btn">🗑</button>
                    <button className="terminal-titlebar-btn">⋮</button>
                  </div>
                </div>
                <div className="terminal-container">
                  <div ref={terminalRef} />
                </div>
              </div>
            </div>
            <BottomPanel sessionId={activeTab}/>
          </div>

          {tabs.length === 0 && (
            <div
              className="card-inner"
              style={{ alignItems: "center", justifyContent: "center" }}
            >
              <div style={{ textAlign: "center", color: "var(--text-muted)" }}>
                <div style={{ fontSize: "32px", marginBottom: "12px" }}>⚡</div>
                <div
                  style={{
                    fontSize: "14px",
                    fontWeight: 600,
                    marginBottom: "6px",
                    color: "var(--text-secondary)",
                  }}
                >
                  No active sessions
                </div>
                <div style={{ fontSize: "12px" }}>
                  Double-click a connection to get started
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* MODALS */}
      <Modal
        open={folderModalOpen}
        onClose={() => {
          setFolderModalOpen(false);
          setEditingFolder(null);
        }}
      >
        <FolderModal
          initialName={editingFolder?.name || ""}
          onSave={async (name) => {
            try {
              if (editingFolder) {
                await UpdateFolder(editingFolder.id, name);
              } else {
                await CreateFolder(name);
              }
              await reload();
              setFolderModalOpen(false);
              setEditingFolder(null);
            } catch (err) {
              console.error(err);
            }
          }}
        />
      </Modal>

      <Modal
        open={connectionModalOpen}
        onClose={() => {
          setConnectionModalOpen(false);
          setEditingConnection(null);
        }}
      >
        <ConnectionModal
          folders={folders}
          initialValue={editingConnection}
          onCancel={() => {
    setConnectionModalOpen(false);
    setEditingConnection(null);
  }}
          onSave={async (connection) => {
            try {
              if (editingConnection) {
                await UpdateConnection({
                  ...connection,
                  id: editingConnection.id,
                });
              } else {
                await CreateConnection(connection);
              }
              await reload();
              setConnectionModalOpen(false);
              setEditingConnection(null);
            } catch (err) {
              console.error(err);
            }
          }}
        />
      </Modal>

      <Modal
        open={assignFolderModalOpen}
        onClose={() => {
          setAssignFolderModalOpen(false);
          setAssigningConnection(null);
        }}
      >
        <AssignFolderModal
          connection={assigningConnection}
          folders={folders}
          onSave={async (updated) => {
            try {
              await UpdateConnection(updated);
              await reload();
              setAssignFolderModalOpen(false);
              setAssigningConnection(null);
            } catch (err) {
              console.error(err);
            }
          }}
        />
      </Modal>
    </div>
  );
}

export default App;
