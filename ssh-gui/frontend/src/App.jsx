import { useEffect, useRef, useState } from "react";
import { Terminal } from "xterm";
import { FitAddon } from "xterm-addon-fit";
import "xterm/css/xterm.css";
import conduiLogo from "./assets/images/condui-transparent.png";
import { FaCheck, FaFile, FaFolder, FaFolderOpen, FaInbox, FaTimes, FaUser } from "react-icons/fa";

import { Events } from "@wailsio/runtime";
import RemoteFileTree from "./components/files/RemoteFileTree";
import {
  AcceptShare,
  CancelShare,
  ConnectSSH,
  TestConnection,
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
  ApproveHostKey,
  GetAccountStatus,
  GetIncomingShares,
  SyncNow,
} from "../bindings/ssh-gui/app";

import VaultUnlock from "./components/account/VaultUnlock";
import AccountModal from "./components/account/AccountModal";
import ShareModal from "./components/account/ShareModal";

import { BsUpload } from "react-icons/bs";
import { GrRefresh } from "react-icons/gr";
import { MdUploadFile } from "react-icons/md";
import { FaLevelUpAlt } from "react-icons/fa";
import { TiFlashOutline } from "react-icons/ti";

import TabBar from "./components/TabBar";
import BottomPanel from "./components/BottomPanel";
import ResourceBar from "./components/ResourceBar";
import { useConnections } from "./hooks/useConnections";
import Modal from "./components/common/Modal";
import ConnectionNode from "./components/connections/ConnectionNode";
import FolderNode from "./components/connections/FolderNode";
import FolderModal from "./components/connections/FolderModal";
import ConnectionModal from "./components/connections/ConnectionModal";
import AssignFolderModal from "./components/connections/AssignFolderModal";
import ContextMenu from "./components/connections/ContextMenu";
import "./components/Layout.css";

function PendingInviteNode({ invite, accepting, onAccept, onDecline }) {
  return (
    <div className="drawer-conn-item pending-invite-item">
      <div className="conn-status">
        <span className="pending-invite-dot">
          <FaInbox />
        </span>
      </div>

      <div className="conn-info">
        <div className="conn-name">Shared connection</div>
        <div className="conn-host">
          {invite.ownerEmail || "Unknown sender"} · {invite.permissions === "write" ? "Read & write" : "Read-only"}
        </div>
      </div>

      <div className={`conn-actions ${accepting ? "connecting" : ""}`}>
        <button
          className="conn-action-btn"
          title="Accept invitation"
          disabled={accepting}
          onClick={(e) => {
            e.stopPropagation();
            onAccept(invite);
          }}
        >
          {accepting ? <span className="conn-loader" /> : <FaCheck />}
        </button>
        <button
          className="conn-action-btn delete"
          title="Decline invitation"
          disabled={accepting}
          onClick={(e) => {
            e.stopPropagation();
            if (confirm("Decline this shared connection invitation?")) onDecline(invite);
          }}
        >
          <FaTimes />
        </button>
      </div>
    </div>
  );
}

function LeftSidebar({
  tabs,
  folders,
  connections,
  pendingInvites,
  acceptingInviteId,
  connectingId,
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
  onShareConnection,
  onAcceptInvite,
  onDeclineInvite,
  onOpenAccount,
  accountStatus,
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
  const filteredInvites = pendingInvites.filter((invite) => {
    const q = search.trim().toLowerCase();
    if (!q) return true;
    return (
      invite.ownerEmail?.toLowerCase().includes(q) ||
      invite.permissions?.toLowerCase().includes(q)
    );
  });

  return (
    <div className="sidebar-container">
      <div className="sidebar">
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
              <FaFolder />
            </button>
          </div>
        </div>

        <input
          className="sidebar-search"
          placeholder="Search connections..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />

        <div
          className="sidebar-list"
          onContextMenu={handleEmptyAreaContextMenu}
        >
          {filteredInvites.length > 0 && (
            <FolderNode
              folder={{ id: "__pending_invites", name: "Invitaciones pendientes" }}
              expanded={expandedFolders.includes("__pending_invites")}
              onToggle={onToggleFolder}
              virtual
            >
              {filteredInvites.map((invite) => (
                <PendingInviteNode
                  key={invite.id}
                  invite={invite}
                  accepting={acceptingInviteId === invite.id}
                  onAccept={onAcceptInvite}
                  onDecline={onDeclineInvite}
                />
              ))}
            </FolderNode>
          )}

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
                    connection={{
                      ...c,
                      online: tabs.filter((tab) => tab.connectionId === c.id).length,
                    }}
                    connecting={connectingId === c.id}
                    folders={folders}
                    onOpen={onOpenConnection}
                    onEdit={onEditConnection}
                    onDelete={onDeleteConnection}
                    onAssignFolder={onAssignFolder}
                    onShare={accountStatus?.loggedIn ? onShareConnection : undefined}
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
                    connection={{
                      ...c,
                      online: tabs.filter((tab) => tab.connectionId === c.id).length,
                    }}
                    connecting={connectingId === c.id}
                    folders={folders}
                    onOpen={onOpenConnection}
                    onEdit={onEditConnection}
                    onDelete={onDeleteConnection}
                    onAssignFolder={onAssignFolder}
                    onShare={accountStatus?.loggedIn ? onShareConnection : undefined}
                  />
                ))}
            </div>
          )}

          {filtered.length === 0 && filteredInvites.length === 0 && (
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
              { icon: <FaFolder />, label: "New folder", onClick: onNewFolder },
            ]}
            onClose={() => setEmptyCtx(null)}
          />
        )}
      </div>

      <div className="sidebar-footer">
        <img className="logo-image-sm" src={conduiLogo} />
      </div>
    </div>
  );
}

function App() {
  const terminalRef = useRef(null);
  const termRef = useRef(null);
  const terminalBuffers = useRef({});
  const fileTreeRef = useRef(null);
  const syncInFlightRef = useRef(false);

  const [tabs, setTabs] = useState([]);
  const [activeTab, setActiveTab] = useState(null);
  const [connectingId, setConnectingId] = useState(null);
  const [connectionChoice, setConnectionChoice] = useState(null);
  const [sshError, setSshError] = useState(null);
  const { folders, connections, reload } = useConnections();

  // Vault & account state
  const [vaultUnlocked, setVaultUnlocked] = useState(false);
  const [accountModalOpen, setAccountModalOpen] = useState(false);
  const [accountStatus, setAccountStatus] = useState(null);
  const [shareTarget, setShareTarget] = useState(null); // connection to share
  const [pendingInvites, setPendingInvites] = useState([]);
  const [acceptingInviteId, setAcceptingInviteId] = useState(null);
  const [expandedFolders, setExpandedFolders] = useState([]);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  // Host key verification
  const [hostKeyPrompt, setHostKeyPrompt] = useState(null);

  const refreshAccountStatus = async () => {
    try {
      const s = await GetAccountStatus();
      setAccountStatus(s);
    } catch (_) {}
  };

  const refreshPendingInvites = async () => {
    if (!accountStatus?.loggedIn) {
      setPendingInvites([]);
      return;
    }
    try {
      const shares = await GetIncomingShares();
      setPendingInvites((shares || []).filter((share) => share.status === "pending"));
    } catch (err) {
      console.error("Failed to refresh share invitations", err);
    }
  };

  useEffect(() => {
    if (vaultUnlocked) refreshAccountStatus();
  }, [vaultUnlocked]);

  useEffect(() => {
    if (!vaultUnlocked || !accountStatus?.loggedIn) {
      setPendingInvites([]);
      return;
    }

    refreshPendingInvites();
    const timer = window.setInterval(refreshPendingInvites, 30000);
    return () => window.clearInterval(timer);
  }, [vaultUnlocked, accountStatus?.loggedIn]);

  useEffect(() => {
    if (pendingInvites.length === 0) return;
    setExpandedFolders((prev) =>
      prev.includes("__pending_invites") ? prev : [...prev, "__pending_invites"],
    );
  }, [pendingInvites.length]);

  useEffect(() => {
    if (!vaultUnlocked || accountStatus?.tier !== "pro") return;

    const runProSync = async () => {
      if (syncInFlightRef.current) return;
      syncInFlightRef.current = true;
      try {
        await SyncNow();
        await reload();
        await refreshAccountStatus();
      } catch (err) {
        console.error("Pro sync check failed", err);
      } finally {
        syncInFlightRef.current = false;
      }
    };

    const timer = window.setInterval(runProSync, 60000);
    return () => window.clearInterval(timer);
  }, [vaultUnlocked, accountStatus?.tier]);

  useEffect(() => {
    const unsub = Events.On("session-disconnected", (event) => {
      const payload = event.data;
      setTabs((prev) =>
        prev.map((t) =>
          t.id === payload.sessionId ? { ...t, disconnected: true } : t,
        ),
      );
    });
    return () => unsub();
  }, []);

  // Host key verification event
  useEffect(() => {
    const unsub = Events.On("host-key-verify", (event) => {
      setHostKeyPrompt(event.data);
    });
    return () => unsub();
  }, []);

  const handleReconnect = async (tab) => {
    const connection = connections.find((c) => c.id === tab.connectionId);
    if (!connection) return;

    setConnectingId(tab.connectionId);
    try {
      const newSessionId = await ConnectSSH(tab.connectionId);
      delete terminalBuffers.current[tab.id];
      setTabs((prev) =>
        prev.map((t) =>
          t.id === tab.id ? { ...t, id: newSessionId, disconnected: false } : t,
        ),
      );
      setActiveTab(newSessionId);
    } catch (err) {
      setSshError({
        title: "Reconnection failed",
        message:
          typeof err === "string" ? err : err?.message || "Unable to connect",
        connection: tab.title,
      });
    } finally {
      setConnectingId(null);
    }
  };

  const [folderModalOpen, setFolderModalOpen] = useState(false);
  const [connectionModalOpen, setConnectionModalOpen] = useState(false);
  const [assignFolderModalOpen, setAssignFolderModalOpen] = useState(false);
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
    if (!terminalRef.current) return; // Guard: terminal div not mounted (vault screen showing)
    const term = new Terminal({
      cursorBlink: true,
      convertEol: true,
      theme: {
        background: "#111827",
        foreground: "#d1d5db",
        cursor: "#6366f1",
        selectionBackground: "rgba(99, 101, 241, 0.07)",
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

    const unsubscribe = Events.On("terminal-output", (event) => {
      const payload = event.data;
      if (!terminalBuffers.current[payload.sessionId]) {
        terminalBuffers.current[payload.sessionId] = "";
      }
      terminalBuffers.current[payload.sessionId] += payload.data;
      if (payload.sessionId !== activeTab) {
        return;
      }
      term.write(payload.data);
    });

    const doFit = () => {
      fitAddon.fit();
      if (activeTab && termRef.current)
        ResizeTerminal(activeTab, term.rows, term.cols);
    };
    window.addEventListener("resize", doFit);

    // Re-ajusta también cuando el contenedor cambia de tamaño por DOM
    // (p.ej. cuando aparece la ResourceBar después de cargar los stats)
    const ro = new ResizeObserver(doFit);
    ro.observe(terminalRef.current);

    return () => {
      unsubscribe();
      window.removeEventListener("resize", doFit);
      ro.disconnect();
      term.dispose();
    };
  }, [activeTab, vaultUnlocked]);

  useEffect(() => {
    if (!activeTab || !termRef.current) return;
    termRef.current.clear();
    termRef.current.write(terminalBuffers.current[activeTab] || "");
    ResizeTerminal(activeTab, termRef.current.rows, termRef.current.cols);
  }, [activeTab]);

  const activeTabData = tabs.find((t) => t.id === activeTab);

  const handleOpenConnection = async (c, forceNew = false) => {
    const existing = tabs.find((t) => t.connectionId === c.id);

    if (existing && !forceNew) {
      setConnectionChoice({
        connection: c,
        session: existing,
      });

      return;
    }

    setConnectingId(c.id);

    try {
      const sessionId = await ConnectSSH(c.id);

      setTabs((prev) => [
        ...prev,
        {
          id: sessionId,
          connectionId: c.id,
          title: c.name,
          color: c.color,
        },
      ]);

      setActiveTab(sessionId);
    } catch (err) {
      setSshError({
        title: "Connection failed",

        message:
          typeof err === "string" ? err : err?.message || "Unable to connect",

        connection: c.name,
      });
    } finally {
      setConnectingId(null);
    }
  };

  const handleTestConnection = async (conn) => {
    try {
      await TestConnection(conn.id);
      alert("Connection successful!");
    } catch (err) {
      alert(
        "Connection failed: " + (typeof err === "string" ? err : err?.message || "Unknown error"),
      );
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

  const handleAcceptInvite = async (invite) => {
    setAcceptingInviteId(invite.id);
    try {
      await AcceptShare(invite.id, invite.encryptedKey, invite.blobId);
      await reload();
      await refreshPendingInvites();
      setExpandedFolders((prev) =>
        prev.includes("__pending_invites") ? prev : [...prev, "__pending_invites"],
      );
    } catch (err) {
      console.error(err);
      alert(typeof err === "string" ? err : err?.message || "Unable to accept invitation");
    } finally {
      setAcceptingInviteId(null);
    }
  };

  const handleDeclineInvite = async (invite) => {
    setAcceptingInviteId(invite.id);
    try {
      await CancelShare(invite.id);
      await refreshPendingInvites();
    } catch (err) {
      console.error(err);
      alert(typeof err === "string" ? err : err?.message || "Unable to decline invitation");
    } finally {
      setAcceptingInviteId(null);
    }
  };

  const confirmDeleteConnection = async () => {
    if (!deleteTarget) return;
    setDeleteBusy(true);
    setDeleteError("");
    try {
      await DeleteConnection(deleteTarget.id);
      await reload();
      const remainingTabs = tabs.filter((tab) => tab.connectionId !== deleteTarget.id);
      setTabs(remainingTabs);
      if (activeTabData?.connectionId === deleteTarget.id) {
        setActiveTab(remainingTabs.length ? remainingTabs[0].id : null);
      }
      setDeleteTarget(null);
    } catch (err) {
      console.error(err);
      setDeleteError(typeof err === "string" ? err : err?.message || "Unable to delete connection");
    } finally {
      setDeleteBusy(false);
    }
  };

  // Show vault screen before main UI
  if (!vaultUnlocked) {
    return <VaultUnlock onUnlocked={() => setVaultUnlocked(true)} />;
  }

  return (
    <div className="app-shell">
      <div className="topbar">
        <div className="topbar-logo">
          <span className="topbar-logo-wordmark">
            condu<span className="i">i</span>
          </span>
          <span className="topbar-logo-tagline">SSH Manager</span>
        </div>
        <div className="topbar-account">
          <button
            className="topbar-account-btn"
            onClick={() => setAccountModalOpen(true)}
            title={accountStatus?.loggedIn ? accountStatus.email : "Sign in to sync"}
          >
            {accountStatus?.loggedIn ? (
              <>
                <span className="topbar-avatar-sm">
                  {accountStatus.email?.[0]?.toUpperCase()}
                </span>
                <span className="topbar-account-email">{accountStatus.email}</span>
                <span className={`tier-badge tier-${accountStatus.tier}`}>
                  {accountStatus.tier === "pro" ? "Pro" : "Free"}
                </span>
              </>
            ) : (
              <>
                <FaUser style={{ fontSize: 12 }} />
                <span>Sign in</span>
              </>
            )}
          </button>
        </div>
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
          tabs={tabs}
          folders={folders}
          connections={connections}
          pendingInvites={pendingInvites}
          acceptingInviteId={acceptingInviteId}
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
          onDeleteConnection={(c) => {
            setDeleteError("");
            setDeleteTarget(c);
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
          connectingId={connectingId}
          activeSessionId={activeTab}
          onShareConnection={(c) => setShareTarget(c)}
          onAcceptInvite={handleAcceptInvite}
          onDeclineInvite={handleDeclineInvite}
          onOpenAccount={() => setAccountModalOpen(true)}
          accountStatus={accountStatus}
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
            onReconnect={handleReconnect}
            connectingId={connectingId}
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
                      title="Parent directory"
                      onClick={() => fileTreeRef.current?.goParent()}
                    >
                      <FaLevelUpAlt />
                    </button>
                    <button
                      className="files-header-btn"
                      title="Subir archivo aquí"
                      onClick={uploadFile}
                      style={{ background: "var(--primary)" }}
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
                  <span
                    className={
                      "terminal-titlebar-dot" +
                      (activeTabData?.disconnected ? " disconnected" : "")
                    }
                  />
                  <span className="terminal-title">
                    {activeTabData ? activeTabData.title : "No Session"}
                  </span>
                  {activeTabData?.disconnected && (
                    <span className="terminal-titlebar-status">
                      Disconnected
                    </span>
                  )}
                  <div className="terminal-titlebar-actions" />
                </div>
                <div className="terminal-container">
                  <div ref={terminalRef} />
                  {activeTabData?.disconnected && (
                    <div className="terminal-disconnect-overlay">
                      <div className="terminal-disconnect-icon">⚠</div>
                      <div className="terminal-disconnect-message">
                        Session disconnected
                      </div>
                      <button
                        className="terminal-reconnect-btn"
                        disabled={connectingId === activeTabData?.connectionId}
                        onClick={() => handleReconnect(activeTabData)}
                      >
                        {connectingId === activeTabData?.connectionId
                          ? "Connecting…"
                          : "Reconnect"}
                      </button>
                    </div>
                  )}
                </div>
                <ResourceBar
                  sessionId={activeTab}
                  disconnected={activeTabData?.disconnected}
                />
              </div>
            </div>
            <BottomPanel sessionId={activeTab} accountStatus={accountStatus} onUpgrade={() => setAccountModalOpen(true)} />
          </div>

          {tabs.length === 0 && (
            <div
              className="card-inner"
              style={{ alignItems: "center", justifyContent: "center" }}
            >
              <div style={{ textAlign: "center", color: "var(--text-muted)" }}>
                <div style={{ fontSize: "32px", marginBottom: "12px" }}> <TiFlashOutline /></div>
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
          connections={connections}
          accountStatus={accountStatus}
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
          onTestConnection={async (connection) => {
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
              await handleTestConnection(connection);              
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

      <Modal
        open={!!deleteTarget}
        onClose={() => {
          if (!deleteBusy) {
            setDeleteTarget(null);
            setDeleteError("");
          }
        }}
      >
        <div>
          <div className="modal-header">
            <h2>Delete connection</h2>
            <p>{deleteTarget?.name}</p>
          </div>
          <div className="modal-body">
            <div className="ssh-error-box" style={{ borderColor: "var(--red)" }}>
              <div className="ssh-error-title" style={{ color: "var(--red)" }}>
                This action cannot be undone
              </div>
              <p style={{ marginTop: 8, fontSize: 12, color: "var(--text-muted)" }}>
                The connection will be removed from this device and from sync on the next update.
              </p>
            </div>
            {deleteError && <div className="vault-error">{deleteError}</div>}
          </div>
          <div className="modal-footer">
            <button
              className="btn-secondary"
              disabled={deleteBusy}
              onClick={() => {
                setDeleteTarget(null);
                setDeleteError("");
              }}
            >
              Cancel
            </button>
            <button
              className="btn-primary"
              disabled={deleteBusy}
              onClick={confirmDeleteConnection}
            >
              {deleteBusy ? "Deleting..." : "Delete"}
            </button>
          </div>
        </div>
      </Modal>
      <Modal open={!!sshError} onClose={() => setSshError(null)}>
        <div>
          <div className="modal-header">
            <h2>Connection failed</h2>

            <p>Unable to establish SSH connection</p>
          </div>

          <div className="modal-body">
            <div className="ssh-error-box">
              <div className="ssh-error-title">{sshError?.connection}</div>

              <pre>{sshError?.message}</pre>
            </div>
          </div>

          <div className="modal-footer">
            <button className="btn-primary" onClick={() => setSshError(null)}>
              OK
            </button>
          </div>
        </div>
      </Modal>
      {/* Account modal */}
      <Modal open={accountModalOpen} onClose={() => { setAccountModalOpen(false); refreshAccountStatus(); }}>
        <AccountModal onClose={() => { setAccountModalOpen(false); refreshAccountStatus(); }} />
      </Modal>

      {/* Share modal */}
      <Modal open={!!shareTarget} onClose={() => setShareTarget(null)}>
        {shareTarget && (
          <ShareModal connection={shareTarget} onClose={() => setShareTarget(null)} />
        )}
      </Modal>

      {/* Host key verification modal */}
      <Modal open={!!hostKeyPrompt} onClose={() => {
        if (hostKeyPrompt) ApproveHostKey(hostKeyPrompt.channelKey, false);
        setHostKeyPrompt(null);
      }}>
        {hostKeyPrompt && (
          <div>
            <div className="modal-header">
              <h2>Unknown host key</h2>
              <p>First connection to {hostKeyPrompt.hostname}:{hostKeyPrompt.port}</p>
            </div>
            <div className="modal-body">
              <div className="ssh-error-box" style={{ borderColor: "var(--yellow)" }}>
                <div className="ssh-error-title" style={{ color: "var(--yellow)" }}>
                  Verify the host fingerprint
                </div>
                <div style={{ fontFamily: "monospace", fontSize: 12, marginTop: 8, wordBreak: "break-all" }}>
                  {hostKeyPrompt.fingerprint}
                </div>
                <p style={{ marginTop: 12, fontSize: 12, color: "var(--text-muted)" }}>
                  If you trust this host, click <strong>Trust & Connect</strong>.
                  The fingerprint will be saved for future connections.
                </p>
              </div>
            </div>
            <div className="modal-footer">
              <button className="btn-secondary" onClick={() => {
                ApproveHostKey(hostKeyPrompt.channelKey, false);
                setHostKeyPrompt(null);
              }}>Cancel</button>
              <button className="btn-primary" onClick={() => {
                ApproveHostKey(hostKeyPrompt.channelKey, true);
                setHostKeyPrompt(null);
              }}>Trust & Connect</button>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        open={!!connectionChoice}
        onClose={() => setConnectionChoice(null)}
      >
        <div>
          <div className="modal-header">
            <h2>Connection already active</h2>

            <p>Choose how you want to continue</p>
          </div>

          <div className="modal-body">
            <div className="connection-choice-card">
              <strong>{connectionChoice?.connection?.name}</strong>

              <span>This server already has an active SSH session.</span>
            </div>
          </div>

          <div className="modal-footer">
            <button
              className="btn-secondary"
              onClick={() => {
                setActiveTab(connectionChoice.session.id);

                setConnectionChoice(null);
              }}
            >
              Go to session
            </button>

            <button
              className="btn-primary"
              onClick={() => {
                const c = connectionChoice.connection;

                setConnectionChoice(null);

                handleOpenConnection(c, true);
              }}
            >
              Open new
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

export default App;
