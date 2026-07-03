import { useEffect, useRef, useState } from "react";
import { Terminal } from "xterm";
import { FitAddon } from "xterm-addon-fit";
import "xterm/css/xterm.css";
import conduiLogo from "./assets/images/condui-transparent.png";
import { FaCheck, FaExclamationTriangle, FaFolder, FaInbox, FaTimes, FaUser } from "react-icons/fa";

import { Events } from "@wailsio/runtime";
import RemoteFileTree from "./components/files/RemoteFileTree";
import {
  AcceptShare,
  CancelShare,
  CloseSession,
  ConnectSSH,
  ConnectSSHVia,
  StartLocalTerminal,
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
  UploadDroppedFile,
} from "../bindings/ssh-gui/app";

import VaultUnlock from "./components/account/VaultUnlock";
import AccountModal from "./components/account/AccountModal";
import ShareModal from "./components/account/ShareModal";

import { BsThreeDots } from "react-icons/bs";
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
import { useTranslation } from "react-i18next";

function PendingInviteNode({ invite, accepting, onAccept, onDecline }) {
  const { t } = useTranslation();
  return (
    <div className="drawer-conn-item pending-invite-item">
      <div className="conn-status">
        <span className="pending-invite-dot">
          <FaInbox />
        </span>
      </div>

      <div className="conn-info">
        <div className="conn-name">{t("app.sharedConnection")}</div>
        <div className="conn-host">
          {invite.ownerEmail || t("app.unknownSender")} · {invite.permissions === "write" ? t("app.readWrite") : t("app.readOnly")}
        </div>
      </div>

      <div className={`conn-actions ${accepting ? "connecting" : ""}`}>
        <button
          className="conn-action-btn"
          title={t("app.acceptInvitation")}
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
          title={t("app.declineInvitation")}
          disabled={accepting}
          onClick={(e) => {
            e.stopPropagation();
            if (confirm(t("app.declineInvitationConfirm"))) onDecline(invite);
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
  onConnectVia,
  onAcceptInvite,
  onDeclineInvite,
  onOpenAccount,
  accountStatus,
  activeSessionId,
}) {
  const { t } = useTranslation();
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
          <span className="sidebar-title">{t("app.connections")}</span>
          <div className="sidebar-header-actions">
            <button
              className="sidebar-icon-btn"
              title={t("app.newConnection")}
              onClick={onNewConnection}
            >
              +
            </button>
            <button
              className="sidebar-icon-btn"
              title={t("app.newFolder")}
              onClick={onNewFolder}
            >
              <FaFolder />
            </button>
          </div>
        </div>

        <input
          className="sidebar-search"
          placeholder={t("app.searchConnections")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />

        <div
          className="sidebar-list"
          onContextMenu={handleEmptyAreaContextMenu}
        >
          {filteredInvites.length > 0 && (
            <FolderNode
              folder={{ id: "__pending_invites", name: t("app.pendingInvitations") }}
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
                    connections={connections}
                    folders={folders}
                    onOpen={onOpenConnection}
                    onEdit={onEditConnection}
                    onDelete={onDeleteConnection}
                    onAssignFolder={onAssignFolder}
                    onShare={accountStatus?.loggedIn ? onShareConnection : undefined}
                    onConnectVia={accountStatus?.tier === "pro" ? onConnectVia : undefined}
                  />
                ))}
              </FolderNode>
            );
          })}

          {filtered.filter((c) => !c.folderId || c.folderId === "").length >
            0 && (
            <div className="sidebar-group">
              {folders.length > 0 && (
                <div className="sidebar-group-label">{t("app.ungrouped")}</div>
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
                    connections={connections}
                    folders={folders}
                    onOpen={onOpenConnection}
                    onEdit={onEditConnection}
                    onDelete={onDeleteConnection}
                    onAssignFolder={onAssignFolder}
                    onShare={accountStatus?.loggedIn ? onShareConnection : undefined}
                    onConnectVia={accountStatus?.tier === "pro" ? onConnectVia : undefined}
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
              {search ? t("app.noResults") : t("app.noConnections")}
            </div>
          )}
        </div>

        {emptyCtx && (
          <ContextMenu
            x={emptyCtx.x}
            y={emptyCtx.y}
            items={[
              { icon: "+", label: t("app.newConnection"), onClick: onNewConnection },
              { icon: <FaFolder />, label: t("app.newFolder"), onClick: onNewFolder },
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
  const { t } = useTranslation();
  const terminalRef = useRef(null);
  const termRef = useRef(null);
  const terminalBuffers = useRef({});
  const fileTreeRef = useRef(null);
  const syncInFlightRef = useRef(false);

  const [tabs, setTabs] = useState([]);
  const [activeTab, setActiveTab] = useState(null);
  const [fileTreePaths, setFileTreePaths] = useState({});
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

  const rememberFileTreePath = (sessionId, path) => {
    setFileTreePaths((prev) => {
      if (prev[sessionId] === path) return prev;
      return { ...prev, [sessionId]: path };
    });
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
      const connectResult = parseConnectResult(await ConnectSSH(tab.connectionId));
      const newSessionId = connectResult.sessionId;
      setFileTreePaths((prev) => {
        const next = { ...prev };
        if (prev[tab.id]) {
          next[newSessionId] = prev[tab.id];
        }
        delete next[tab.id];
        return next;
      });
      delete terminalBuffers.current[tab.id];
      setTabs((prev) =>
        prev.map((t) =>
          t.id === tab.id
            ? { ...t, id: newSessionId, homePath: connectResult.homePath, disconnected: false }
            : t,
        ),
      );
      setActiveTab(newSessionId);
    } catch (err) {
      setSshError({
        title: t("app.reconnectionFailed"),
        message:
          typeof err === "string" ? err : err?.message || t("app.unableToConnect"),
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
        foreground: "#dfdfdf",
        cursor: "#6366f1",
        selectionBackground: "rgba(99, 101, 241, 0.07)",
        black: "#1f2937",
        red: "#f87171",
        green: "#27b65b",
        yellow: "#fcd34d",
        blue: "#3d7cc5",
        magenta: "#c084fc",
        cyan: "#00ddfa",
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
  const isLocalActive = activeTabData?.type === "local";

  const parseConnectResult = (result) => {
    if (typeof result === "string") {
      return { sessionId: result, homePath: "/" };
    }
    return {
      sessionId: result?.sessionId || result?.SessionID || "",
      homePath: result?.homePath || result?.HomePath || "/",
    };
  };

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
      const connectResult = parseConnectResult(await ConnectSSH(c.id));
      const sessionId = connectResult.sessionId;

      setTabs((prev) => [
        ...prev,
        {
          id: sessionId,
          connectionId: c.id,
          title: c.name,
          color: c.color,
          homePath: connectResult.homePath,
        },
      ]);

      setActiveTab(sessionId);
    } catch (err) {
      setSshError({
        title: t("app.connectionFailed"),

        message:
          typeof err === "string" ? err : err?.message || t("app.unableToConnect"),

        connection: c.name,
      });
    } finally {
      setConnectingId(null);
    }
  };

  const handleOpenLocalTerminal = async () => {
    setConnectingId("__local");

    try {
      const connectResult = parseConnectResult(await StartLocalTerminal());
      const sessionId = connectResult.sessionId;

      setTabs((prev) => [
        ...prev,
        {
          id: sessionId,
          title: t("app.localTerminal"),
          type: "local",
          color: "var(--green)",
          homePath: connectResult.homePath,
        },
      ]);

      setActiveTab(sessionId);
    } catch (err) {
      setSshError({
        title: t("app.localTerminalFailed"),
        message: typeof err === "string" ? err : err?.message || t("app.unableToConnect"),
        connection: t("app.localTerminal"),
      });
    } finally {
      setConnectingId(null);
    }
  };

  const handleConnectVia = async (connection, jumpHost) => {
    setConnectingId(connection.id);
    try {
      const connectResult = parseConnectResult(await ConnectSSHVia(connection.id, jumpHost.id));
      const sessionId = connectResult.sessionId;
      setTabs((prev) => [
        ...prev,
        {
          id: sessionId,
          connectionId: connection.id,
          title: `${connection.name} via ${jumpHost.name}`,
          color: connection.color,
          homePath: connectResult.homePath,
        },
      ]);
      setActiveTab(sessionId);
    } catch (err) {
      setSshError({
        title: t("app.connectionFailed"),
        message: typeof err === "string" ? err : err?.message || t("app.unableToConnect"),
        connection: `${connection.name} via ${jumpHost.name}`,
      });
    } finally {
      setConnectingId(null);
    }
  };



  const getUploadErrorMessage = (err) =>
    typeof err === "string" ? err : err?.message || String(err);

  const uploadFile = async () => {
    // Si no hay una sesión SSH activa, detenemos la operación
    if (!activeTab) {
      alert(t("app.selectActiveSession"));
      return;
    }

    const currentRemotePath = fileTreeRef.current?.currentPath || "/";

    try {
      await UploadFile(activeTab, currentRemotePath);

      fileTreeRef.current?.refresh();
    } catch (err) {
      const message = getUploadErrorMessage(err);
      if (message.includes("cancelad")) {
        console.log("Subida cancelada por el usuario.");
        return;
      }
      console.error("Error al subir archivo:", err);
      alert(t("app.uploadError", { error: message }));
    }
  };

  useEffect(() => {
    const unsubscribe = Events.On(
      "remote-files-dropped",
      async (event) => {
        const payload = event.data || {};
        const targetAttrs = payload.details?.Attributes || payload.details?.attributes || {};
        if (targetAttrs["data-remote-file-drop-target"] !== "true") return;
        if (!activeTab || isLocalActive) {
          alert(t("app.selectActiveSession"));
          return;
        }

        const localPaths = (payload.files || []).filter(Boolean);
        if (localPaths.length === 0) return;

        const currentRemotePath = fileTreeRef.current?.currentPath || "/";
        try {
          for (const localPath of localPaths) {
            await UploadDroppedFile(activeTab, currentRemotePath, localPath);
          }
          fileTreeRef.current?.refresh();
        } catch (err) {
          const message = getUploadErrorMessage(err);
          console.error("Error al subir archivo arrastrado:", err);
          alert(t("app.uploadError", { error: message }));
        }
      },
    );

    return () => unsubscribe();
  }, [activeTab, isLocalActive, t]);

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
      alert(typeof err === "string" ? err : err?.message || t("app.acceptInviteError"));
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
      alert(typeof err === "string" ? err : err?.message || t("app.declineInviteError"));
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
      setDeleteError(typeof err === "string" ? err : err?.message || t("app.unableDeleteConnection"));
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
          <span className="topbar-logo-tagline">{t("app.sshManager")}</span>
        </div>
        <div className="topbar-account">
          <button
            className="topbar-account-btn"
            onClick={() => setAccountModalOpen(true)}
            title={accountStatus?.loggedIn ? accountStatus.email : t("app.signInToSync")}
          >
            {accountStatus?.loggedIn ? (
              <>
                <span className="topbar-avatar-sm">
                  {accountStatus.email?.[0]?.toUpperCase()}
                </span>
                <span className="topbar-account-email">{accountStatus.email}</span>
                <span className={`tier-badge tier-${accountStatus.tier}`}>
                  {accountStatus.tier === "pro" ? t("common.pro") : t("common.free")}
                </span>
              </>
            ) : (
              <>
                <FaUser style={{ fontSize: 12 }} />
                <span>{t("app.signIn")}</span>
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
          <button className="topbar-btn">Upload</button>
          <button className="topbar-btn">Download</button>
          <button className="topbar-btn">Tunnels</button>
          <button className="topbar-btn">Settings</button>
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
          <button className="topbar-icon-btn">Alerts</button>
          <button className="topbar-icon-btn">Layout</button>
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
          onConnectVia={handleConnectVia}
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
            onClose={async (tabId) => {
              try {
                await CloseSession(tabId);
              } catch (err) {
                console.error("Failed to close SSH session", err);
              }

              delete terminalBuffers.current[tabId];
              setFileTreePaths((prev) => {
                const next = { ...prev };
                delete next[tabId];
                return next;
              });
              setTabs((prev) => prev.filter((t) => t.id !== tabId));
              if (activeTab === tabId) {
                const remaining = tabs.filter((t) => t.id !== tabId);
                setActiveTab(remaining.length ? remaining[0].id : null);
              }
            }}
            onReconnect={handleReconnect}
            onOpenLocalTerminal={handleOpenLocalTerminal}
            connectingId={connectingId}
          />
          <div
            className="card-inner"
            style={{ display: tabs.length > 0 ? "flex" : "none" }}
          >
            <div className="card-mid">
              {!isLocalActive && (
              <div
                className="files-panel"
                data-file-drop-target
                data-remote-file-drop-target="true"
              >
                <div className="files-drop-hint" aria-hidden="true">
                  <MdUploadFile />
                  <span>{t("app.dropUploadRemote")}</span>
                </div>
                <div className="files-header">
                  {t("app.remoteFiles")}
                  <div className="files-header-actions">
                    <button
                      className="files-header-btn"
                      title={t("app.parentDirectory")}
                      onClick={() => fileTreeRef.current?.goParent()}
                    >
                      <FaLevelUpAlt />
                    </button>
                    <button
                      className="files-header-btn"
                      title={t("app.uploadHere")}
                      onClick={uploadFile}
                    >
                      <MdUploadFile />
                    </button>
                    <button
                      className="files-header-btn"
                      onClick={() => fileTreeRef.current?.refresh()}
                    >
                      <GrRefresh />
                    </button>
                    <button className="files-header-btn">
                      <BsThreeDots />
                    </button>
                  </div>
                </div>
                <RemoteFileTree
                  sessionId={activeTab}
                  initialPath={fileTreePaths[activeTab] || activeTabData?.homePath || "/"}
                  onPathChange={rememberFileTreePath}
                  ref={fileTreeRef}
                />
              </div>
              )}
              <div className="terminal-card">
                <div className="terminal-titlebar">
                  <span
                    className={
                      "terminal-titlebar-dot" +
                      (activeTabData?.disconnected ? " disconnected" : "")
                    }
                  />
                  <span className="terminal-title">
                    {activeTabData ? activeTabData.title : t("app.noSession")}
                  </span>
                  {activeTabData?.disconnected && (
                    <span className="terminal-titlebar-status">
                      {t("app.disconnected")}
                    </span>
                  )}
                  <div className="terminal-titlebar-actions" />
                </div>
                <div className="terminal-container">
                  <div ref={terminalRef} />
                  {activeTabData?.disconnected && (
                    <div className="terminal-disconnect-overlay">
                      <div className="terminal-disconnect-icon">
                        <FaExclamationTriangle />
                      </div>
                      <div className="terminal-disconnect-message">
                        {t("app.sessionDisconnected")}
                      </div>
                      {!isLocalActive && (
                        <button
                          className="terminal-reconnect-btn"
                          disabled={connectingId === activeTabData?.connectionId}
                          onClick={() => handleReconnect(activeTabData)}
                        >
                          {connectingId === activeTabData?.connectionId
                            ? t("app.connecting")
                            : t("app.reconnect")}
                        </button>
                      )}
                    </div>
                  )}
                </div>
                {!isLocalActive && (
                  <ResourceBar
                    sessionId={activeTab}
                    disconnected={activeTabData?.disconnected}
                  />
                )}
              </div>
            </div>
            {!isLocalActive && (
              <BottomPanel sessionId={activeTab} accountStatus={accountStatus} onUpgrade={() => setAccountModalOpen(true)} />
            )}
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
                  {t("app.noActiveSessions")}
                </div>
                <div style={{ fontSize: "12px" }}>
                  {t("app.doubleClickToStart")}
                </div>
                <button
                  className="terminal-reconnect-btn"
                  style={{ marginTop: "16px" }}
                  onClick={handleOpenLocalTerminal}
                  disabled={connectingId === "__local"}
                >
                  {connectingId === "__local"
                    ? t("app.connecting")
                    : t("app.openLocalTerminal")}
                </button>
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
            <h2>{t("app.deleteConnection")}</h2>
            <p>{deleteTarget?.name}</p>
          </div>
          <div className="modal-body">
            <div className="ssh-error-box" style={{ borderColor: "var(--red)" }}>
              <div className="ssh-error-title" style={{ color: "var(--red)" }}>
                {t("app.cannotUndo")}
              </div>
              <p style={{ marginTop: 8, fontSize: 12, color: "var(--text-muted)" }}>
                {t("app.deleteConnectionDescription")}
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
              {t("common.cancel")}
            </button>
            <button
              className="btn-primary"
              disabled={deleteBusy}
              onClick={confirmDeleteConnection}
            >
              {deleteBusy ? t("app.deleting") : t("common.delete")}
            </button>
          </div>
        </div>
      </Modal>
      <Modal open={!!sshError} onClose={() => setSshError(null)}>
        <div>
          <div className="modal-header">
            <h2>{t("app.connectionFailed")}</h2>

            <p>{t("app.unableEstablishSsh")}</p>
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
              <h2>{t("app.unknownHostKey")}</h2>
              <p>{t("app.firstConnectionTo", { host: hostKeyPrompt.hostname, port: hostKeyPrompt.port })}</p>
            </div>
            <div className="modal-body">
              <div className="ssh-error-box" style={{ borderColor: "var(--yellow)" }}>
                <div className="ssh-error-title" style={{ color: "var(--yellow)" }}>
                  {t("app.verifyFingerprint")}
                </div>
                <div style={{ fontFamily: "monospace", fontSize: 12, marginTop: 8, wordBreak: "break-all" }}>
                  {hostKeyPrompt.fingerprint}
                </div>
                <p style={{ marginTop: 12, fontSize: 12, color: "var(--text-muted)" }}>
                  {t("app.trustHostHelp")}
                </p>
              </div>
            </div>
            <div className="modal-footer">
              <button className="btn-secondary" onClick={() => {
                ApproveHostKey(hostKeyPrompt.channelKey, false);
                setHostKeyPrompt(null);
              }}>{t("common.cancel")}</button>
              <button className="btn-primary" onClick={() => {
                ApproveHostKey(hostKeyPrompt.channelKey, true);
                setHostKeyPrompt(null);
              }}>{t("app.trustAndConnect")}</button>
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
            <h2>{t("app.connectionAlreadyActive")}</h2>

            <p>{t("app.chooseHowContinue")}</p>
          </div>

          <div className="modal-body">
            <div className="connection-choice-card">
              <strong>{connectionChoice?.connection?.name}</strong>

              <span>{t("app.serverAlreadyActive")}</span>
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
              {t("app.goToSession")}
            </button>

            <button
              className="btn-primary"
              onClick={() => {
                const c = connectionChoice.connection;

                setConnectionChoice(null);

                handleOpenConnection(c, true);
              }}
            >
              {t("app.openNew")}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

export default App;
