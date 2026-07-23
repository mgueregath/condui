import { useState, useEffect, useMemo } from "react";
import { Events } from "@wailsio/runtime";
import {
  GetTunnels,
  ToggleTunnel,
  AddTunnel,
  DeleteTunnel,
  EditTunnel,
  GetDockerContainers,
  ToggleContainer,
  GetListeningPorts,
  GetDatabases,
  OpenDockerLogWindow,
  OpenDbExplorerWindow,
  GetVirtualBoxVMs,
  VirtualBoxAction,
  GetDockerStats,
} from "../../bindings/ssh-gui/app";
import { FaArrowDown, FaArrowRight, FaArrowUp, FaLock, FaTrash, FaDocker, FaEdit, FaPlay, FaStop, FaDatabase, FaSearch, FaPlus, FaRedoAlt, FaDesktop, FaSave, FaPause } from "react-icons/fa";
import {
  SiPostgresql,
  SiTimescale,
  SiMysql,
  SiMariadb,
  SiMongodb,
  SiRedis,
  SiElastic,
  SiInfluxdb,
  SiNeo4J,
  SiCockroachlabs,
  SiApachecouchdb,
  SiApachecassandra,
  SiScylladb,
  SiOpensearch,
} from "react-icons/si";
import { LuLogs, LuNetwork, LuDownload, LuUpload } from "react-icons/lu";
import { BiTransfer } from "react-icons/bi";
import { GiWarpPipe } from "react-icons/gi";
import { IoIosAdd } from "react-icons/io";
import { PiCpuFill, PiMemoryFill, PiDiscBold } from "react-icons/pi";
import { GrRefresh, GrFormRefresh } from "react-icons/gr";
import { TbNetwork, TbBackground } from "react-icons/tb";
import { TiFlashOutline } from "react-icons/ti";
import { useTranslation } from "react-i18next";

const DB_TYPES = {
  PostgreSQL: {
    color: "#3b82f6",
    icon: SiPostgresql,
  },

  TimescaleDB: {
    color: "#b6c41d",
    icon: SiTimescale,
  },

  MySQL: {
    color: "#f97316",
    icon: SiMysql,
  },

  "MySQL / MariaDB": {
    color: "#f97316",
    icon: SiMysql,
  },

  MariaDB: {
    color: "#c084fc",
    icon: SiMariadb,
  },

  MongoDB: {
    color: "#4ade80",
    icon: SiMongodb,
  },

  Redis: {
    color: "#ef4444",
    icon: SiRedis,
  },

  Elasticsearch: {
    color: "#fbbf24",
    icon: SiElastic,
  },

  OpenSearch: {
    color: "#fbbf24",
    icon: SiOpensearch,
  },

  Cassandra: {
    color: "#a855f7",
    icon: SiApachecassandra,
  },

  ScyllaDB: {
    color: "#a855f7",
    icon: SiScylladb,
  },

  CouchDB: {
    color: "#fb923c",
    icon: SiApachecouchdb,
  },

  InfluxDB: {
    color: "#22d3ee",
    icon: SiInfluxdb,
  },

  "SQL Server": {
    color: "#64748b",
    icon: FaDatabase,
  },

  Oracle: {
    color: "#f43f5e",
    icon: FaDatabase,
  },

  Neo4j: {
    color: "#4ade80",
    icon: SiNeo4J,
  },

  ClickHouse: {
    color: "#facc15",
    icon: FaDatabase,
  },

  CockroachDB: {
    color: "#3b82f6",
    icon: SiCockroachlabs,
  },

  RethinkDB: {
    color: "#f97316",
    icon: FaDatabase,
  },

  ArangoDB: {
    color: "#a855f7",
    icon: FaDatabase,
  },
};

const TABS = [
  { id: "logs", labelKey: "panel.logs", icon: <LuLogs /> },
  { id: "transfers", labelKey: "panel.transfers", icon: <BiTransfer /> },
  { id: "tunnels", labelKey: "panel.tunnels", icon: <GiWarpPipe /> },
  { id: "ports", labelKey: "panel.ports", icon: <LuNetwork /> },
  { id: "docker", labelKey: "panel.docker", icon: <FaDocker /> },
  { id: "virtualbox", labelKey: "panel.virtualbox", icon: <FaDesktop /> },
  { id: "databases", labelKey: "panel.databases", icon: <FaDatabase />, pro: true },
];

const DOCKER_STATE_SORT_ORDER = [
  "running",
  "paused",
  "restarting",
  "created",
  "removing",
  "exited",
  "dead",
];

const DOCKER_STATUS_TIME_UNITS = {
  second: 1,
  minute: 60,
  hour: 60 * 60,
  day: 60 * 60 * 24,
  week: 60 * 60 * 24 * 7,
  month: 60 * 60 * 24 * 30,
  year: 60 * 60 * 24 * 365,
};

function StatPill({ label, value, sub, warn = false, icon = null }) {
  const color = warn ? "var(--red)" : "var(--accent)";
  const bg = warn ? "rgba(239,68,68,0.1)" : "rgba(99,102,241,0.08)";
  return (
    <span style={{
      display: "inline-flex", alignItems: "baseline", gap: 4,
      padding: "1px 7px", borderRadius: 4,
      background: bg, fontSize: 10.5,
    }}>
      {icon && <span>{icon}</span>}
      <span style={{ color: "var(--text-muted)", fontWeight: 600 }}>{label}</span>
      <span style={{ color, fontWeight: 700 }}>{value}</span>
      {sub && <span style={{ color: "var(--text-muted)", fontSize: 9.5 }}>({sub})</span>}
    </span>
  );
}

function Spec({ icon, label, mono = false, muted = false }) {
  return (
    <span style={{
      display: "flex", alignItems: "center", gap: 4,
      fontSize: 11,
      color: muted ? "var(--text-muted)" : "var(--text-secondary)",
      fontFamily: mono ? "monospace" : undefined,
    }}>
      <span style={{ fontSize: 10 }}>{icon}</span>
      {label}
    </span>
  );
}

function ActionBtn({ children, title, color, onClick, disabled, style = {} }) {
  return (
    <button
      title={title}
      onClick={onClick}
      disabled={disabled}
      style={{
        padding: "5px 7px", cursor: disabled ? "default" : "pointer",
        background: color, color: "#fff", border: "none",
        borderRadius: 4, fontSize: 11, display: "flex",
        alignItems: "center", justifyContent: "center",
        opacity: disabled ? 0.5 : 1,
        transition: "opacity .15s",
        ...style,
      }}
      onMouseEnter={e => !disabled && (e.currentTarget.style.opacity = "0.82")}
      onMouseLeave={e => !disabled && (e.currentTarget.style.opacity = "1")}
    >
      {children}
    </button>
  );
}

export default function BottomPanel({ sessionId, accountStatus, features, onUpgrade }) {
  const { t } = useTranslation();
  const isPro = accountStatus?.tier === "pro";
  const dbManagerEnabled = !!features?.dbManager;
  const [activeTab, setActiveTab] = useState("logs");

  // Estados de datos
  const [logs, setLogs] = useState([]);
  const [transfers, setTransfers] = useState({});
  const [tunnels, setTunnels] = useState([]);
  const [containers, setContainers] = useState([]);
  const [ports, setPorts] = useState([]);
  const [panelSearch, setPanelSearch] = useState("");
  const [portSort, setPortSort] = useState({ key: "port", direction: "asc" });
  const [dockerSort, setDockerSort] = useState({ key: "state", direction: "asc" });
  const [vboxSort, setVboxSort] = useState({ key: "state", direction: "asc" });
  const [databaseSort, setDatabaseSort] = useState({ key: "name", direction: "asc" });
  const [databases, setDatabases] = useState([]);
  const [vms, setVms] = useState([]);
  const [vboxError, setVboxError] = useState(null);
  const [vboxActionLoading, setVboxActionLoading] = useState(null);
  const [dockerStats, setDockerStats] = useState({}); // map containerID to stats


  // Estados para la Modal del Túnel (Crea y Edita)
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingTunnelId, setEditingTunnelId] = useState(null);
  const [formLocalPort, setFormLocalPort] = useState("");
  const [formRemoteHost, setFormRemoteHost] = useState("127.0.0.1");
  const [formRemotePort, setFormRemotePort] = useState("");

  // 1. ESCUCHAR EVENTOS GLOBALES DE WAILS (Corregido con referencias limpias)
  useEffect(() => {
    // Usamos actualizaciones funcionales (prev) => ... para que React no necesite
    // meter el estado en las dependencias del useEffect y no se pierdan los eventos
    const offLog = Events.On("log-event", (event) => {
      const log = event.data;
      setLogs((prev) => {
        // Evitar duplicados rápidos en renderizado y limitar a 200 líneas
        return [log, ...prev].slice(0, 200);
      });
    });

    const offTransfer = Events.On("transfer-status", (event) => {
      const data = event.data;
      setTransfers((prev) => ({
        ...prev,
        [data.id]: data,
      }));
    });

    return () => {
      if (offLog) offLog();
      if (offTransfer) offTransfer();
    };
  }, []); // Se mantiene ejecutado una sola vez al montar el componente global

  // 2. Cargar datos dinámicos bajo demanda y Auto-refresh de Docker
  useEffect(() => {
    if (!sessionId) return;

    if (activeTab === "tunnels") {
      fetchTunnels();
    }

    if (activeTab === "docker") {
      fetchContainers();
      fetchStats();
      const intContainers = setInterval(fetchContainers, 4000);
      const intStats = setInterval(fetchStats, 3000);
      return () => { clearInterval(intContainers); clearInterval(intStats); };
    }

    if (activeTab === "ports") {
      fetchPorts();
      const intervalId = setInterval(fetchPorts, 5000);
      return () => clearInterval(intervalId);
    }

    if (activeTab === "databases") {
      fetchDatabases();
      const intervalId = setInterval(fetchDatabases, 10000);
      return () => clearInterval(intervalId);
    }

    if (activeTab === "virtualbox") {
      fetchVMs();
      const intervalId = setInterval(fetchVMs, 8000);
      return () => clearInterval(intervalId);
    }
  }, [activeTab, sessionId]);

  // --- Operaciones de Tunnels ---
  const fetchTunnels = async () => {
    try {
      const res = await GetTunnels(sessionId);
      setTunnels(res || []);
    } catch (err) {
      console.error("Error al obtener túneles:", err);
    }
  };

  const openCreateModal = () => {
    setEditingTunnelId(null);
    setFormLocalPort("");
    setFormRemoteHost("127.0.0.1");
    setFormRemotePort("");
    setIsModalOpen(true);
  };

  const openEditModal = (t) => {
    setEditingTunnelId(t.id);
    setFormLocalPort(t.localPort);
    setFormRemoteHost(t.remoteHost);
    setFormRemotePort(t.remotePort);
    setIsModalOpen(true);
  };

  const handleSaveTunnel = async (e) => {
    e.preventDefault();
    if (!sessionId || !formLocalPort || !formRemoteHost || !formRemotePort)
      return;

    try {
      if (editingTunnelId) {
        await EditTunnel(
          sessionId,
          editingTunnelId,
          parseInt(formLocalPort, 10),
          formRemoteHost,
          parseInt(formRemotePort, 10),
        );
      } else {
        await AddTunnel(
          sessionId,
          parseInt(formLocalPort, 10),
          formRemoteHost,
          parseInt(formRemotePort, 10),
        );
      }
      setIsModalOpen(false);
      await fetchTunnels();
    } catch (err) {
      alert(t("panel.tunnelError", { error: err }));
    }
  };

  const handleToggleTunnel = async (tunnel) => {
    try {
      await ToggleTunnel(
        sessionId,
        tunnel.id,
        tunnel.localPort,
        tunnel.remoteHost,
        tunnel.remotePort,
        !tunnel.active,
      );
      await fetchTunnels();
    } catch (err) {
      console.error("Error al conmutar túnel:", err);
    }
  };

  const handleDeleteTunnel = async (tunnelId) => {
    if (!window.confirm(t("panel.deleteTunnelConfirm")))
      return;
    try {
      await DeleteTunnel(sessionId, tunnelId);
      await fetchTunnels();
    } catch (err) {
      console.error("Error al eliminar túnel:", err);
    }
  };

  // --- Puertos ---
  const fetchPorts = async () => {
    try {
      const res = await GetListeningPorts(sessionId);
      setPorts(res || []);
    } catch (err) {
      console.error("Error al obtener puertos:", err);
    }
  };

  const handlePortSort = (key) => {
    setPortSort((prev) => ({
      key,
      direction: prev.key === key && prev.direction === "asc" ? "desc" : "asc",
    }));
  };

  const handleDockerSort = (key) => {
    setDockerSort((prev) => ({
      key,
      direction: prev.key === key && prev.direction === "asc" ? "desc" : "asc",
    }));
  };

  const handleVboxSort = (key) => {
    setVboxSort((prev) => ({
      key,
      direction: prev.key === key && prev.direction === "asc" ? "desc" : "asc",
    }));
  };

  const handleDatabaseSort = (key) => {
    setDatabaseSort((prev) => ({
      key,
      direction: prev.key === key && prev.direction === "asc" ? "desc" : "asc",
    }));
  };

  const compareText = (left, right) =>
    String(left ?? "").localeCompare(String(right ?? ""), undefined, {
      numeric: true,
      sensitivity: "base",
    });

  const compareDockerState = (left, right) => {
    const leftIndex = DOCKER_STATE_SORT_ORDER.indexOf(String(left ?? "").toLowerCase());
    const rightIndex = DOCKER_STATE_SORT_ORDER.indexOf(String(right ?? "").toLowerCase());
    const normalizedLeft = leftIndex === -1 ? DOCKER_STATE_SORT_ORDER.length : leftIndex;
    const normalizedRight = rightIndex === -1 ? DOCKER_STATE_SORT_ORDER.length : rightIndex;
    return normalizedLeft - normalizedRight || compareText(left, right);
  };

  const parseDockerStatusTime = (status) => {
    const normalized = String(status ?? "").toLowerCase();
    if (!normalized) return null;
    if (normalized.includes("less than a second")) return 0;

    const wordAmountMatch = normalized.match(/\b(a|an)\s+(second|minute|hour|day|week|month|year)s?\b/);
    if (wordAmountMatch) {
      return DOCKER_STATUS_TIME_UNITS[wordAmountMatch[2]];
    }

    const amountMatch = normalized.match(/\b(\d+)\s+(second|minute|hour|day|week|month|year)s?\b/);
    if (!amountMatch) return null;

    return Number(amountMatch[1]) * DOCKER_STATUS_TIME_UNITS[amountMatch[2]];
  };

  const compareDockerStatus = (left, right) => {
    const leftSeconds = parseDockerStatusTime(left);
    const rightSeconds = parseDockerStatusTime(right);

    if (leftSeconds !== null && rightSeconds !== null && leftSeconds !== rightSeconds) {
      return leftSeconds - rightSeconds;
    }

    if (leftSeconds !== null && rightSeconds === null) return -1;
    if (leftSeconds === null && rightSeconds !== null) return 1;

    return compareText(left, right);
  };

  const matchesSearch = (values) => {
    const query = panelSearch.trim().toLowerCase();
    if (!query) return true;
    return values.some((value) => String(value ?? "").toLowerCase().includes(query));
  };

  const searchPlaceholder = {
    docker: t("panel.searchContainers"),
    ports: t("panel.searchPorts"),
    virtualbox: t("panel.searchVms"),
    databases: t("panel.searchDatabases"),
  }[activeTab];

  const showPanelSearch =
    activeTab === "docker" ||
    activeTab === "ports" ||
    activeTab === "virtualbox" ||
    (activeTab === "databases" && isPro);

  const visiblePorts = useMemo(() => {
    const filtered = ports.filter((p) =>
      matchesSearch([p.proto, p.port, p.address, p.process]),
    );

    return [...filtered].sort((a, b) => {
      const direction = portSort.direction === "asc" ? 1 : -1;
      if (portSort.key === "port") {
        return ((Number(a.port) || 0) - (Number(b.port) || 0)) * direction;
      }

      const left = String(a[portSort.key] ?? "").toLowerCase();
      const right = String(b[portSort.key] ?? "").toLowerCase();
      return left.localeCompare(right, undefined, { numeric: true, sensitivity: "base" }) * direction;
    });
  }, [ports, panelSearch, portSort]);

  const sortedContainers = useMemo(() => (
    containers.filter((c) =>
      matchesSearch([c.names, c.image, c.state, c.status, c.ports]),
    ).sort((a, b) => {
      const direction = dockerSort.direction === "asc" ? 1 : -1;
      if (dockerSort.key === "state") {
        return (compareDockerState(a.state, b.state) || compareDockerStatus(a.status, b.status) || compareText(a.names, b.names)) * direction;
      }
      if (dockerSort.key === "ports") {
        return (compareText(a.ports, b.ports) || compareText(a.names, b.names)) * direction;
      }
      return (compareText(a.names, b.names) || compareDockerState(a.state, b.state)) * direction;
    })
  ), [containers, panelSearch, dockerSort]);

  const sortedVms = useMemo(() => (
    vms.filter((vm) =>
      matchesSearch([vm.name, vm.uuid, vm.state, vm.memoryMb, vm.cpus, vm.os, vm.ip]),
    ).sort((a, b) => {
      const direction = vboxSort.direction === "asc" ? 1 : -1;
      if (vboxSort.key === "state") {
        return (compareText(a.state, b.state) || compareText(a.name, b.name)) * direction;
      }
      return (compareText(a.name, b.name) || compareText(a.state, b.state)) * direction;
    })
  ), [vms, panelSearch, vboxSort]);

  const sortedDatabases = useMemo(() => (
    databases.filter((db) =>
      matchesSearch([db.name, db.port, db.address, db.source, db.container, db.image]),
    ).sort((a, b) => {
      const direction = databaseSort.direction === "asc" ? 1 : -1;
      if (databaseSort.key === "port") {
        return (((Number(a.port) || 0) - (Number(b.port) || 0)) || compareText(a.name, b.name)) * direction;
      }
      return (compareText(a.name, b.name) || ((Number(a.port) || 0) - (Number(b.port) || 0))) * direction;
    })
  ), [databases, panelSearch, databaseSort]);

  const sortIcon = (sort, key) => {
    if (sort.key !== key) return null;
    return sort.direction === "asc" ? <FaArrowUp /> : <FaArrowDown />;
  };

  // --- Bases de datos ---
  const fetchDatabases = async () => {
    try {
      const res = await GetDatabases(sessionId);
      setDatabases(res || []);
    } catch (err) {
      console.error("Error al obtener bases de datos:", err);
    }
  };

  // --- VirtualBox ---
  const fetchVMs = async () => {
    try {
      const res = await GetVirtualBoxVMs(sessionId);
      setVms(res || []);
      setVboxError(null);
    } catch (err) {
      setVboxError(typeof err === "string" ? err : err?.message || t("panel.virtualboxUnavailableFallback"));
      setVms([]);
    }
  };

  const handleVMAction = async (vmName, action) => {
    setVboxActionLoading(vmName + ":" + action);
    try {
      await VirtualBoxAction(sessionId, vmName, action);
      await fetchVMs();
    } catch (err) {
      console.error("VBox action error:", err);
    } finally {
      setVboxActionLoading(null);
    }
  };

  // --- Operaciones de Docker ---
  const fetchContainers = async () => {
    try {
      const res = await GetDockerContainers(sessionId);
      setContainers(res || []);
    } catch (err) {
      console.error("Error al obtener contenedores:", err);
    }
  };

  const fetchStats = async () => {
    try {
      const res = await GetDockerStats(sessionId);
      if (!res) return;
      // Index by container ID (short, 12 chars) for O(1) lookup
      const map = {};
      for (const s of res) map[s.id] = s;
      setDockerStats(map);
    } catch (_) { }
  };

  const handleToggleContainer = async (containerId, currentState) => {
    const action = currentState === "running" ? "stop" : "start";
    try {
      await ToggleContainer(sessionId, containerId, action);
      await fetchContainers();
    } catch (err) {
      console.error(`Error al ejecutar ${action} en contenedor:`, err);
    }
  };

  const handleRestartContainer = async (containerId) => {
    try {
      await ToggleContainer(sessionId, containerId, "restart");
      await fetchContainers();
    } catch (err) {
      console.error("Error al reiniciar contenedor:", err);
    }
  };

  return (
    <div
      className="bottom-panel"
      style={{
        background: "var(--bg-elevated)",
        borderTop: "1px solid var(--border)",
        display: "flex",
        flexDirection: "column",
        flex: 0.3,
        minHeight: "270px",
      }}
    >
      {/* CABECERA DE PESTAÑAS */}
      <div
        className="bottom-tabs"
        style={{
          display: "flex",
          borderBottom: "1px solid var(--border)",
          background: "var(--bg-elevated)",
          flexShrink: 0,
        }}
      >
        {TABS.map((tab) => {
          const locked = tab.pro && !isPro;
          const isActive = activeTab === tab.id;
          return (
            <div
              key={tab.id}
              className={isActive ? "bottom-tab active" : "bottom-tab"}
              onClick={() => locked ? onUpgrade?.() : setActiveTab(tab.id)}
              title={locked ? t("panel.proOnly") : undefined}
              style={{
                padding: "10px 16px",
                cursor: "pointer",
                fontSize: "13px",
                fontWeight: isActive ? "600" : "400",
                color: locked
                  ? "var(--text-muted)"
                  : isActive ? "var(--accent)" : "var(--text-secondary)",
                borderBottom: isActive
                  ? "2px solid var(--accent)"
                  : "2px solid transparent",
                display: "flex",
                alignItems: "center",
                gap: "5px",
                opacity: locked ? 0.6 : 1,
              }}
            >
              {tab.icon}
              {t(tab.labelKey)}
              {locked && (
                <span style={{ fontSize: 9, marginLeft: 2, opacity: 0.8 }}>
                  <FaLock />
                </span>
              )}
              {tab.id === "transfers" &&
                Object.values(transfers).filter((t) => t.status === "active").length > 0 && (
                  <span
                    className="badge-active-count"
                    style={{
                      marginLeft: "4px",
                      background: "var(--accent)",
                      color: "#fff",
                      borderRadius: "10px",
                      padding: "1px 6px",
                      fontSize: "10px",
                    }}
                  >
                    {Object.values(transfers).filter((t) => t.status === "active").length}
                  </span>
                )}
            </div>
          );
        })}

        <div className="bottom-tab-actions">
          {activeTab === "tunnels" && (
            <button
              className="bottom-action-btn"
              onClick={openCreateModal}
              style={{
                background: "var(--accent)",
                color: "#fff",
                border: "none",
                fontWeight: "600",
              }}
            >
              <FaPlus /> {t("panel.newTunnel")}
            </button>
          )}
          {activeTab === "logs" && (
            <button
              className="bottom-action-btn"
              onClick={() => setLogs([])}
              style={{
                fontWeight: "600",
              }}
            >
              <FaTrash /> {t("panel.clear")}
            </button>
          )}
          {showPanelSearch && (
            <div className="ports-search-wrap">
              <FaSearch />
              <input
                className="ports-search-input"
                value={panelSearch}
                onChange={(e) => setPanelSearch(e.target.value)}
                placeholder={searchPlaceholder}
              />
            </div>
          )}
          {(activeTab === "tunnels" || activeTab === "docker" || activeTab === "ports" || activeTab === "databases" || activeTab === "virtualbox") && (
            <button
              className="bottom-action-btn"
              onClick={
                activeTab === "tunnels" ? fetchTunnels :
                  activeTab === "ports" ? fetchPorts :
                    activeTab === "databases" ? fetchDatabases :
                      activeTab === "virtualbox" ? fetchVMs :
                        fetchContainers
              }
            >
              <FaRedoAlt /> {t("common.refresh")}
            </button>
          )}
        </div>
      </div>

      {/* CONTENIDO DE LA PESTAÑA ACTIVA CON SCROLL INDEPENDIENTE (Ajustado a 320px de visualización cómoda) */}
      <div
        className="bottom-content"
        style={{
          height: "320px",
          overflowY: "auto",
          background: "var(--bg-white)",
          position: "relative",
        }}
      >
        {/* PESTAÑA: LOGS */}
        {activeTab === "logs" && (
          <div
            className="logs-tab-content"
            style={{
              padding: "12px"
            }}
          >
            {logs.length === 0 ? (
              <div
                className="empty-state"
                style={{
                  color: "var(--text-muted)",
                  textAlign: "center",
                  padding: "40px",
                }}
              >
                {t("panel.noLogs")}
              </div>
            ) : (
              logs.map((l, i) => (
                <div
                  className="log-line"
                  key={i}
                  style={{ display: "flex", gap: "10px", marginBottom: "4px" }}
                >
                  <span
                    className="log-time"
                    style={{ color: "var(--text-muted)" }}
                  >
                    [{l.time}]
                  </span>
                  <span
                    className={`log-badge ${l.type?.toLowerCase()}`}
                    style={{
                      color: "var(--accent)",
                      fontWeight: "bold",
                      minWidth: "60px",
                    }}
                  >
                    {l.type}
                  </span>
                  <span
                    className={`log-msg ${l.cls}`}
                    style={{
                      color:
                        l.cls === "success"
                          ? "var(--green)"
                          : l.cls === "warn"
                            ? "var(--yellow)"
                            : l.cls === "error"
                              ? "var(--red)"
                              : "var(--text-primary)",
                    }}
                  >
                    {l.msg}
                  </span>
                </div>
              ))
            )}
          </div>
        )}
        {/* PESTAÑA: TRANSFERS */}
        {activeTab === "transfers" && (
          <div className="transfers-tab-content" style={{ padding: "12px" }}>
            {Object.keys(transfers).length === 0 ? (
              <div
                className="empty-state"
                style={{
                  color: "var(--text-muted)",
                  textAlign: "center",
                  padding: "40px",
                }}
              >
                {t("panel.noTransfers")}
              </div>
            ) : (
              Object.values(transfers).map((t) => (
                <div
                  key={t.id}
                  className="transfer-item"
                  style={{
                    display: "flex",
                    alignItems: "center",
                    marginBottom: "8px",
                    gap: "15px",
                    fontSize: "12px",
                    borderBottom: "1px solid var(--border-subtle)",
                    paddingBottom: "6px",
                  }}
                >
                  <span style={{ fontSize: "14px" }}>
                    {t.direction === "upload" ? <LuUpload /> : <LuDownload />}
                  </span>
                  <span
                    style={{
                      flex: 1,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                      color: "var(--text-primary)",
                    }}
                  >
                    {t.name}
                  </span>
                  <div
                    style={{
                      width: "150px",
                      backgroundColor: "var(--bg-hover)",
                      borderRadius: "4px",
                      height: "8px",
                      overflow: "hidden",
                    }}
                  >
                    <div
                      style={{
                        width: `${t.progress}%`,
                        backgroundColor:
                          t.status === "error" ? "var(--red)" : "var(--green)",
                        height: "100%",
                        transition: "width 0.2s",
                      }}
                    ></div>
                  </div>
                  <span
                    style={{
                      width: "40px",
                      textAlign: "right",
                      color: "var(--text-secondary)",
                    }}
                  >
                    {t.progress}%
                  </span>
                </div>
              ))
            )}
          </div>
        )}
        {/* PESTAÑA: TUNNELS */}
        {activeTab === "tunnels" && (
          <div className="tunnels-tab-content" style={{ padding: "12px" }}>
            {tunnels.length === 0 ? (
              <div
                className="empty-state"
                style={{
                  color: "var(--text-muted)",
                  textAlign: "center",
                  padding: "40px",
                }}
              >
                {t("panel.noTunnels")}
              </div>
            ) : (
              <table
                style={{
                  width: "100%",
                  borderCollapse: "collapse",
                  fontSize: "12px",
                  textAlign: "left",
                  background: "var(--bg-surface)",
                  border: "1px solid var(--border)",
                  borderRadius: "var(--card-radius)",
                }}
              >
                <thead>
                  <tr
                    style={{
                      color: "var(--text-secondary)",
                      background: "var(--bg-hover)",
                      borderBottom: "1px solid var(--border)",
                    }}
                  >
                    <th style={{ padding: "8px 12px" }}>{t("panel.localPort")}</th>
                    <th>{t("panel.remoteDestination")}</th>
                    <th>{t("panel.status")}</th>
                    <th style={{ textAlign: "right", paddingRight: "12px" }}>
                      {t("panel.actions")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {tunnels.map((tunnel) => (
                    <tr
                      key={tunnel.id}
                      style={{ borderBottom: "1px solid var(--border-subtle)" }}
                    >
                      <td
                        style={{
                          padding: "10px 12px",
                          fontWeight: "600",
                          color: "var(--blue)",
                        }}
                      >
                        127.0.0.1:{tunnel.localPort}
                      </td>
                      <td style={{ color: "var(--text-primary)" }}>
                        {tunnel.remoteHost}:{tunnel.remotePort}
                      </td>
                      <td>
                        <span
                          style={{
                            padding: "3px 8px",
                            borderRadius: "12px",
                            fontSize: "11px",
                            fontWeight: "500",
                            backgroundColor: tunnel.active
                              ? "rgba(34,197,94,0.12)"
                              : "rgba(107,114,128,0.1)",
                            color: tunnel.active
                              ? "var(--green)"
                              : "var(--text-secondary)",
                          }}
                        >
                          {tunnel.active ? t("panel.listening") : t("panel.inactive")}
                        </span>
                      </td>
                      <td style={{ textAlign: "right", paddingRight: "12px" }}>
                        <button
                          onClick={() => handleToggleTunnel(tunnel)}
                          style={{
                            padding: "3px 10px",
                            cursor: "pointer",
                            backgroundColor: tunnel.active
                              ? "var(--red)"
                              : "var(--green)",
                            color: "#fff",
                            border: "none",
                            borderRadius: "4px",
                            fontSize: "11px",
                            fontWeight: "600",
                            marginRight: "6px",
                          }}
                        >
                          {tunnel.active ? t("panel.turnOff") : t("panel.turnOn")}
                        </button>
                        <button
                          onClick={() => openEditModal(tunnel)}
                          style={{
                            background: "none",
                            border: "1px solid var(--border)",
                            color: "var(--text-secondary)",
                            borderRadius: "4px",
                            padding: "2px 6px",
                            cursor: "pointer",
                            fontSize: "11px",
                            marginRight: "4px",
                          }}
                          title={t("common.edit")}
                        >
                          <FaEdit />
                        </button>
                        <button
                          onClick={() => handleDeleteTunnel(tunnel.id)}
                          style={{
                            background: "none",
                            border: "1px solid var(--border)",
                            color: "var(--text-secondary)",
                            borderRadius: "4px",
                            padding: "2px 6px",
                            cursor: "pointer",
                            fontSize: "11px",
                          }}
                          title={t("common.delete")}
                        >
                          <FaTrash />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}
        {/* PESTAÑA: PORTS */}
        {activeTab === "ports" && (
          <div className="ports-tab-content" style={{ padding: "12px" }}>
            {ports.length === 0 ? (
              <div style={{ color: "var(--text-muted)", textAlign: "center", padding: "40px" }}>
                {t("panel.noPorts")}
              </div>
            ) : visiblePorts.length === 0 ? (
              <div style={{ color: "var(--text-muted)", textAlign: "center", padding: "40px" }}>
                {t("panel.noPortResults")}
              </div>
            ) : (
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "12px" }}>
                <thead>
                  <tr style={{ color: "var(--text-secondary)", background: "var(--bg-hover)", borderBottom: "1px solid var(--border)" }}>
                    <th style={{ padding: "7px 12px", textAlign: "left", width: "70px" }}>
                      <button className="ports-sort-btn" onClick={() => handlePortSort("proto")}>
                        {t("panel.protocol")}
                        <span className="ports-sort-icon">{sortIcon(portSort, "proto")}</span>
                      </button>
                    </th>
                    <th style={{ padding: "7px 12px", textAlign: "left", width: "80px" }}>
                      <button className="ports-sort-btn" onClick={() => handlePortSort("port")}>
                        {t("connection.port")}
                        <span className="ports-sort-icon">{sortIcon(portSort, "port")}</span>
                      </button>
                    </th>
                    <th style={{ padding: "7px 12px", textAlign: "left" }}>{t("panel.address")}</th>
                    <th style={{ padding: "7px 12px", textAlign: "left" }}>
                      <button className="ports-sort-btn" onClick={() => handlePortSort("process")}>
                        {t("panel.process")}
                        <span className="ports-sort-icon">{sortIcon(portSort, "process")}</span>
                      </button>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {visiblePorts.map((p, i) => (
                    <tr key={i} style={{ borderBottom: "1px solid var(--border-subtle)" }}>
                      <td style={{ padding: "7px 12px" }}>
                        <span className={`port-proto-badge ${p.proto.toLowerCase()}`}>
                          {p.proto}
                        </span>
                      </td>
                      <td style={{ padding: "7px 12px", fontWeight: 700, color: "var(--accent)", fontFamily: "monospace" }}>
                        {p.port}
                      </td>
                      <td style={{ padding: "7px 12px", color: "var(--text-secondary)", fontFamily: "monospace", fontSize: "11px" }}>
                        {p.address}
                      </td>
                      <td style={{ padding: "7px 12px", color: p.process === "-" ? "var(--text-muted)" : "var(--text-primary)", fontWeight: p.process !== "-" ? 500 : 400 }}>
                        {p.process}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}
        {/* PESTAÑA: DOCKER */}
        {activeTab === "docker" && (
          <div
            className="docker-tab-content"
            style={{
              padding: "12px",
              display: "flex",
              flexDirection: "column",
              gap: "6px",
            }}
          >
            {containers.length === 0 ? (
              <div
                className="empty-state"
                style={{
                  color: "var(--text-muted)",
                  textAlign: "center",
                  padding: "40px",
                }}
              >
                {t("panel.noContainers")}
              </div>
            ) : sortedContainers.length === 0 ? (
              <div
                className="empty-state"
                style={{
                  color: "var(--text-muted)",
                  textAlign: "center",
                  padding: "40px",
                }}
              >
                {t("panel.noContainerResults")}
              </div>
            ) : (
              <>
                <div className="panel-list-header">
                  <div style={{ flex: "1 1 20%" }}>
                    <button className="panel-sort-btn" onClick={() => handleDockerSort("name")}>
                      {t("panel.container")}
                      <span className="panel-sort-icon">{sortIcon(dockerSort, "name")}</span>
                    </button>
                  </div>
                  <div style={{ flex: "1 1 20%" }}>
                    <button className="panel-sort-btn" onClick={() => handleDockerSort("state")}>
                      {t("panel.status")}
                      <span className="panel-sort-icon">{sortIcon(dockerSort, "state")}</span>
                    </button>
                  </div>
                  <div style={{ flex: "1 1 25%" }}>
                    <button className="panel-sort-btn" onClick={() => handleDockerSort("ports")}>
                      {t("panel.ports")}
                      <span className="panel-sort-icon">{sortIcon(dockerSort, "ports")}</span>
                    </button>
                  </div>
                  <div style={{ flex: "1 1 15%" }}>{t("panel.stats")}</div>
                  <div style={{ flex: "0 0 86px", textAlign: "right" }}>{t("panel.actions")}</div>
                </div>
                {sortedContainers
                  .map((c) => {
                    const isRunning = c.state === "running";
                    const st = dockerStats[c.id.slice(0, 12)] || dockerStats[c.id] || null;

                    return (
                    <div
                      key={c.id}
                      className="docker-container-row"
                      style={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "between",
                        background: "var(--bg-surface)",
                        border: "1px solid var(--border-subtle)",
                        borderRadius: "6px",
                        padding: "8px 12px",
                        gap: "16px",
                        transition: "border-color 0.15s, background-color 0.15s",
                      }}
                      onMouseEnter={(e) => {
                        e.currentTarget.style.borderColor = "var(--border)";
                        e.currentTarget.style.backgroundColor = "var(--bg-hover)";
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.borderColor =
                          "var(--border-subtle)";
                        e.currentTarget.style.backgroundColor =
                          "var(--bg-surface)";
                      }}
                    >
                      {/* 1. IDENTIFICACIÓN DEL CONTENEDOR (Nombre e Imagen) */}
                      <div
                        style={{
                          flex: "1 1 20%",
                          minWidth: 0,
                          display: "flex",
                          flexDirection: "column",
                          gap: "2px",
                        }}
                      >
                        <span
                          style={{
                            fontWeight: "600",
                            fontSize: "13px",
                            color: isRunning
                              ? "var(--green)"
                              : "var(--text-secondary)",
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                        >
                          {c.names}
                        </span>
                        <span
                          style={{
                            fontFamily: "var(--font-mono, monospace)",
                            fontSize: "10.5px",
                            color: "var(--text-muted)",
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                          title={c.image}
                        >
                          {c.image}
                        </span>
                      </div>

                      {/* 2. ESTADO INTEGRADO (Badge + Mensaje juntos en un bloque) */}
                      <div
                        style={{
                          flex: "1 1 20%",
                          minWidth: 0,
                          display: "flex",
                          flexDirection: "column",
                          alignItems: "flex-start",
                          gap: "3px",
                        }}
                      >
                        <span
                          style={{
                            padding: "2px 6px",
                            borderRadius: "4px",
                            fontSize: "10px",
                            fontWeight: "700",
                            textTransform: "uppercase",
                            letterSpacing: "0.5px",
                            backgroundColor: isRunning
                              ? "rgba(34,197,94,0.12)"
                              : "rgba(239,68,68,0.12)",
                            color: isRunning ? "var(--green)" : "var(--red)",
                            display: "inline-block",
                          }}
                        >
                          {c.state}
                        </span>
                        {c.status && (
                          <span
                            style={{
                              fontSize: "11px",
                              color: "var(--text-secondary)",
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whiteSpace: "nowrap",
                            }}
                            title={c.status}
                          >
                            {c.status}
                          </span>
                        )}
                      </div>
                      <div
                        style={{
                          flex: "1 1 25%",
                          minWidth: 0,
                          display: "flex",
                          flexDirection: "column",
                          gap: "2px",
                        }}>
                        {c.ports && (
                          <div style={{ display: "flex", flexWrap: "wrap", gap: "3px", marginTop: "2px" }}>
                            {c.ports.split(", ").filter(p => p && !p.startsWith(":::")).map((p, i) => (
                              <span key={i} className="docker-port-badge">
                                {p.replace("0.0.0.0:", "").replace(/->(\d+)\/tcp/, " -> $1")}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                      <div style={{
                        flex: "1 1 15%",
                        minWidth: 0,
                        display: "flex",
                        flexDirection: "column",
                        gap: "2px",
                      }}>
                        {/* Stats de CPU y RAM (solo contenedores running) */}
                        {st && (
                          <div style={{ display: "flex", gap: 10, marginTop: 4, flexWrap: "wrap" }}>
                            <StatPill
                              label="CPU: "
                              value={st.cpuPerc}
                              warn={parseFloat(st.cpuPerc) > 80}
                              icon={<PiCpuFill />}
                            />
                            <StatPill
                              label="RAM: "
                              value={st.memUsage}
                              sub={st.memPerc}
                              warn={parseFloat(st.memPerc) > 80}
                              icon={<PiMemoryFill />}
                            />
                          </div>
                        )}
                      </div>
                      {/* 3. ACCIÓN DE CONTROL (Fijada a la derecha) */}
                      <div
                        style={{
                          flex: "0 0 auto",
                          display: "flex",
                          alignItems: "center",
                          gap: "6px",
                        }}
                      >
                        <button
                          title={t("panel.viewDockerLogs")}
                          onClick={() => OpenDockerLogWindow(sessionId, c.id, c.names).catch(console.error)}
                          style={{
                            padding: "5px 7px",
                            cursor: "pointer",
                            backgroundColor: "transparent",
                            color: "var(--text-secondary)",
                            border: "1px solid var(--border-subtle)",
                            borderRadius: "4px",
                            fontSize: "12px",
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "center",
                            transition: "all 0.15s",
                          }}
                          onMouseEnter={(e) => {
                            e.currentTarget.style.borderColor = "var(--accent)";
                            e.currentTarget.style.color = "var(--accent)";
                          }}
                          onMouseLeave={(e) => {
                            e.currentTarget.style.borderColor = "var(--border-subtle)";
                            e.currentTarget.style.color = "var(--text-secondary)";
                          }}
                        >
                          <LuLogs />
                        </button>
                        <button
                          onClick={() => handleToggleContainer(c.id, c.state)}
                          style={{
                            padding: "5px 5px",
                            cursor: "pointer",
                            backgroundColor: isRunning
                              ? "var(--red)"
                              : "var(--green)",
                            color: "#fff",
                            border: "none",
                            borderRadius: "4px",
                            fontSize: "11px",
                            fontWeight: "600",
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "center",
                            gap: "4px",
                            boxShadow: "0 1px 2px rgba(0,0,0,0.05)",
                            transition: "opacity 0.15s",
                          }}
                          onMouseEnter={(e) =>
                            (e.currentTarget.style.opacity = "0.85")
                          }
                          onMouseLeave={(e) =>
                            (e.currentTarget.style.opacity = "1")
                          }
                        >
                          {isRunning ? <FaStop /> : <FaPlay />}
                        </button>
                        <button
                          onClick={() => handleRestartContainer(c.id)}
                          style={{
                            padding: "5px 5px",
                            cursor: "pointer",
                            backgroundColor: "var(--yellow)",
                            color: "#fff",
                            border: "none",
                            borderRadius: "4px",
                            fontSize: "11px",
                            fontWeight: "600",
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "center",
                            gap: "4px",
                            boxShadow: "0 1px 2px rgba(0,0,0,0.05)",
                            transition: "opacity 0.15s",
                          }}
                          onMouseEnter={(e) =>
                            (e.currentTarget.style.opacity = "0.85")
                          }
                          onMouseLeave={(e) =>
                            (e.currentTarget.style.opacity = "1")
                          }
                        >
                          <GrRefresh />
                        </button>
                      </div>
                    </div>
                    );
                  })}
              </>
            )}
          </div>
        )}

        {/* ===================== TAB: VIRTUALBOX ===================== */}
        {activeTab === "virtualbox" && (
          <div style={{ padding: "12px", display: "flex", flexDirection: "column", gap: 8 }}>
            {vboxError ? (
              <div style={{ padding: "32px", textAlign: "center", color: "var(--text-muted)", fontSize: 13 }}>
                <FaDesktop style={{ fontSize: 28, opacity: 0.3, display: "block", margin: "0 auto 10px" }} />
                {t("panel.virtualboxUnavailable")}
                <div style={{ fontSize: 11, marginTop: 6, opacity: 0.7 }}>{vboxError}</div>
              </div>
            ) : vms.length === 0 ? (
              <div style={{ padding: "32px", textAlign: "center", color: "var(--text-muted)", fontSize: 13 }}>
                {t("panel.noVms")}
              </div>
            ) : sortedVms.length === 0 ? (
              <div style={{ padding: "32px", textAlign: "center", color: "var(--text-muted)", fontSize: 13 }}>
                {t("panel.noVmResults")}
              </div>
            ) : (
              <>
                <div className="panel-list-header">
                  <div style={{ flex: "1 1 20%" }}>
                    <button className="panel-sort-btn" onClick={() => handleVboxSort("name")}>
                      {t("panel.machine")}
                      <span className="panel-sort-icon">{sortIcon(vboxSort, "name")}</span>
                    </button>
                  </div>
                  <div style={{ flex: "1" }}>
                    <button className="panel-sort-btn" onClick={() => handleVboxSort("state")}>
                      {t("panel.status")}
                      <span className="panel-sort-icon">{sortIcon(vboxSort, "state")}</span>
                    </button>
                  </div>
                  <div style={{ flex: "1" }}>{t("panel.specs")}</div>
                  <div style={{ flex: "2", textAlign: "right" }}>{t("panel.actions")}</div>
                </div>
                {sortedVms.map(vm => {
              const isRunning = vm.state === "running";
              const isPaused = vm.state === "paused";
              const isSaved = vm.state === "saved";
              const isStopped = vm.state === "stopped" || vm.state === "aborted";
              const isStuck = isRunning || isPaused; // can force-stop
              const isLoading = vboxActionLoading?.startsWith(vm.name + ":");

              const stateColor = isRunning ? "var(--green)"
                : isPaused ? "var(--yellow)"
                  : isSaved ? "#60a5fa"
                    : vm.state === "error" || vm.state === "aborted" ? "var(--red)"
                      : "var(--text-muted)";

              const stateBg = isRunning ? "rgba(34,197,94,0.12)"
                : isPaused ? "rgba(250,204,21,0.12)"
                  : isSaved ? "rgba(96,165,250,0.12)"
                    : vm.state === "error" || vm.state === "aborted" ? "rgba(239,68,68,0.12)"
                      : "rgba(107,114,128,0.1)";

              const ram = vm.memoryMb >= 1024
                ? `${(vm.memoryMb / 1024).toFixed(vm.memoryMb % 1024 === 0 ? 0 : 1)} GB`
                : vm.memoryMb > 0 ? `${vm.memoryMb} MB` : null;

              return (
                <div
                  key={vm.uuid || vm.name}
                  style={{
                    background: "var(--bg-surface)",
                    border: "1px solid var(--border-subtle)",
                    borderLeft: `3px solid ${stateColor}`,
                    borderRadius: 7, padding: "10px 14px",
                    alignItems: "center",
                    display: "flex",
                    justifyContent: "between",
                    opacity: isLoading ? 0.65 : 1,
                    transition: "opacity .2s",
                  }}
                >
                  <div
                    style={{
                      flex: "1 1 20%",
                      minWidth: 0,
                      display: "flex",
                      flexDirection: "column",
                      gap: "2px",
                    }}>
                    <FaDesktop style={{ color: stateColor, fontSize: 14, flexShrink: 0 }} />

                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{
                        fontWeight: 700, fontSize: 13, color: "var(--text-primary)",
                        overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap"
                      }}>
                        {vm.name}
                      </div>
                      {vm.uuid && (
                        <div style={{
                          fontSize: 9.5, fontFamily: "monospace", color: "var(--text-muted)",
                          overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", marginTop: 1
                        }}>
                          {vm.uuid}
                        </div>
                      )}
                    </div>
                  </div>
                  <div style={{
                    flex: "1",
                    minWidth: 0,
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "flex-start",
                    gap: "2px",
                  }}>
                    <span
                      style={{
                        padding: "2px 8px", borderRadius: 4, flexShrink: 0,
                        fontSize: "10px",
                        fontWeight: "700",
                        textTransform: "uppercase",
                        letterSpacing: "0.5px",
                        background: stateBg, color: stateColor,
                        display: "inline-block",
                      }}
                    >
                      {vm.state}
                    </span>
                  </div>
                  {/* ── Specs ── */}
                  <div
                    style={{
                      flex: "1",
                      minWidth: 0,
                      display: "flex",
                      flexDirection: "column",
                      gap: "2px",
                    }}>
                    {ram && <Spec icon={<PiMemoryFill />} label={ram} />}
                    {vm.cpus > 0 && <Spec icon={<PiCpuFill />} label={`${vm.cpus} vCPU${vm.cpus !== 1 ? "s" : ""}`} />}
                    {vm.os && <Spec icon={<PiDiscBold />} label={vm.os} />}
                    {vm.ip && <Spec icon={<TbNetwork />} label={vm.ip} mono />}
                    {isRunning && !vm.ip && (
                      <Spec icon={<TbNetwork />} label={t("panel.ipUnavailable")} muted />
                    )}
                  </div>

                  {/* ── Acciones ── */}
                  <div
                    style={{
                      flex: "2",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "flex-end",
                      gap: "6px",
                    }}>
                    {(isStopped || isSaved) && (
                      <>
                        <ActionBtn title={t("panel.startGui")} color="var(--green)"
                          disabled={isLoading} onClick={() => handleVMAction(vm.name, "start-gui")}>
                          <FaDesktop style={{ marginRight: 4 }} /> GUI
                        </ActionBtn>
                        <ActionBtn title={t("panel.startHeadless")} color="var(--accent)"
                          disabled={isLoading} onClick={() => handleVMAction(vm.name, "start-headless")}>
                          <TbBackground style={{ marginRight: 4 }} /> Headless
                        </ActionBtn>
                      </>
                    )}
                    {isRunning && (
                      <>
                        <ActionBtn title={t("panel.pause")} color="var(--yellow)" disabled={isLoading}
                          onClick={() => handleVMAction(vm.name, "pause")}>
                          <FaPause style={{ marginRight: 4 }} /> {t("panel.pause")}
                        </ActionBtn>
                        <ActionBtn title={t("panel.saveStateHelp")} color="#60a5fa" disabled={isLoading}
                          onClick={() => handleVMAction(vm.name, "savestate")}>
                          <FaSave style={{ marginRight: 4 }} /> {t("panel.saveState")}
                        </ActionBtn>
                        <ActionBtn title={t("panel.restartHelp")} color="var(--yellow)"
                          disabled={isLoading} onClick={() => handleVMAction(vm.name, "reset")}>
                          <GrFormRefresh style={{ marginRight: 4 }} /> {t("panel.restart")}
                        </ActionBtn>
                        <ActionBtn title={t("panel.shutdownHelp")} color="var(--red)"
                          disabled={isLoading} onClick={() => handleVMAction(vm.name, "stop-acpi")}>
                          <FaStop style={{ marginRight: 4 }} /> {t("panel.shutdown")}
                        </ActionBtn>
                      </>
                    )}
                    {isPaused && (
                      <ActionBtn title={t("panel.resumeHelp")} color="var(--green)" disabled={isLoading}
                        onClick={() => handleVMAction(vm.name, "resume")}>
                        <FaPlay style={{ marginRight: 4 }} /> {t("panel.resume")}
                      </ActionBtn>
                    )}
                    {/* Forzar apagado — siempre visible si la VM no está detenida */}
                  {isStuck && (
                      <ActionBtn
                        title={t("panel.forceStopHelp")}
                        color="var(--red)"
                        disabled={isLoading}
                        onClick={() => {
                          if (window.confirm(t("panel.forceStopConfirm", { name: vm.name })))
                            handleVMAction(vm.name, "stop-force");
                        }}
                      >
                        &nbsp;<TiFlashOutline /> &nbsp;
                      </ActionBtn>
                  )}
                  </div>

                  
                </div>
              );
                })}
              </>
            )}
          </div>
        )}

        {/* ===================== TAB: DATABASES ===================== */}
        {activeTab === "databases" && !isPro && (
          <div style={{
            display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center",
            height: "100%", gap: 12, padding: "12px", textAlign: "center",
          }}>
            <FaDatabase style={{ fontSize: 28, color: "var(--text-muted)", opacity: 0.5 }} />
            <div style={{ fontWeight: 600, fontSize: 14, color: "var(--text-primary)" }}>
              {t("panel.databaseExplorerPro")}
            </div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", maxWidth: 280, lineHeight: 1.6 }}>
              {t("panel.databaseExplorerProHelp")}
            </div>
            <button
              onClick={onUpgrade}
              style={{
                marginTop: 4, padding: "6px 18px", borderRadius: 8,
                background: "var(--accent)", color: "#fff",
                border: "none", cursor: "pointer", fontSize: 12, fontWeight: 600,
              }}
            >
              {t("panel.viewPlans")} <FaArrowRight style={{ marginLeft: 6 }} />
            </button>
          </div>
        )}
        {activeTab === "databases" && isPro && (
          <div className="docker-tab-content" style={{ padding: "12px" }}>
            {databases.length === 0 ? (
              <div style={{ padding: "32px", textAlign: "center", color: "var(--text-muted)", fontSize: "13px" }}>
                {t("panel.noDatabases")}
              </div>
            ) : sortedDatabases.length === 0 ? (
              <div style={{ padding: "32px", textAlign: "center", color: "var(--text-muted)", fontSize: "13px" }}>
                {t("panel.noDatabaseResults")}
              </div>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
                <div className="panel-list-header">
                  <div style={{ flex: "0 0 20px" }} />
                  <div style={{ flex: "1 1 0" }}>
                    <button className="panel-sort-btn" onClick={() => handleDatabaseSort("name")}>
                      {t("panel.database")}
                      <span className="panel-sort-icon">{sortIcon(databaseSort, "name")}</span>
                    </button>
                  </div>
                  <div style={{ flex: "0 0 92px", textAlign: "right" }}>
                    <button className="panel-sort-btn right" onClick={() => handleDatabaseSort("port")}>
                      {t("connection.port")}
                      <span className="panel-sort-icon">{sortIcon(databaseSort, "port")}</span>
                    </button>
                  </div>
                  <div style={{ flex: "0 0 112px", textAlign: "right" }}>{t("panel.source")}</div>
                </div>
                {sortedDatabases.map((db, i) => {
                  const isDocker = db.source === "docker";
                  const dbMeta = DB_TYPES[db.name] || {};
                  const color = dbMeta.color || "var(--accent)";
                  const Icon = dbMeta.icon || FaDatabase;
                  return (
                    <div
                      key={i}
                      style={{
                        display: "flex",
                        alignItems: "center",
                        background: "var(--bg-surface)",
                        border: "1px solid var(--border-subtle)",
                        borderLeft: `3px solid ${color}`,
                        borderRadius: "6px",
                        padding: "10px 14px",
                        gap: "14px",
                        transition: "border-color 0.15s, background-color 0.15s",
                      }}
                      onMouseEnter={(e) => {
                        e.currentTarget.style.borderColor = "var(--border)";
                        e.currentTarget.style.backgroundColor = "var(--bg-hover)";
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.borderColor = "var(--border-subtle)";
                        e.currentTarget.style.backgroundColor = "var(--bg-surface)";
                      }}
                    >
                      {/* Icono */}
                      <div style={{ fontSize: "20px", color, flexShrink: 0 }}>
                        <Icon />
                      </div>

                      {/* Info principal */}
                      <div style={{ flex: "1 1 0", minWidth: 0 }}>
                        <div style={{ fontWeight: "600", fontSize: "13px", color: "var(--text-primary)", marginBottom: "2px" }}>
                          {db.name}
                        </div>
                        {isDocker && db.container && (
                          <div style={{ fontSize: "10.5px", fontFamily: "var(--font-mono, monospace)", color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                            {db.container}
                            {db.image && <span style={{ opacity: 0.6 }}> · {db.image}</span>}
                          </div>
                        )}
                      </div>

                      {/* Puerto */}
                      <div style={{ textAlign: "right", flex: "0 0 92px" }}>
                        {db.port > 0 ? (
                          <>
                          <div style={{ fontFamily: "var(--font-mono, monospace)", fontSize: "13px", fontWeight: "600", color }}>
                            {db.port}
                          </div>
                          {db.address && !db.address.startsWith("0.0.0.0") && (
                            <div style={{ fontSize: "9.5px", fontFamily: "var(--font-mono, monospace)", color: "var(--text-muted)" }}>
                              {db.address}
                            </div>
                          )}
                          </>
                        ) : (
                          <span style={{ color: "var(--text-muted)" }}>-</span>
                        )}
                      </div>

                      {/* Fuente badge + Explorar */}
                      <div style={{ flex: "0 0 112px", display: "flex", flexDirection: "column", alignItems: "flex-end", gap: "6px" }}>
                        <span style={{
                          padding: "2px 8px",
                          borderRadius: "4px",
                          fontSize: "10px",
                          fontWeight: "700",
                          textTransform: "uppercase",
                          letterSpacing: "0.5px",
                          backgroundColor: isDocker ? "rgba(59,130,246,0.15)" : "rgba(34,197,94,0.12)",
                          color: isDocker ? "#60a5fa" : "var(--green)",
                        }}>
                          {isDocker ? "Docker" : t("panel.system")}
                        </span>
                        {dbManagerEnabled && (
                          <button
                            style={{
                              display: "flex", alignItems: "center", gap: "4px",
                              padding: "3px 10px", borderRadius: "4px",
                              border: "1px solid var(--border-subtle)",
                              background: "transparent",
                              color: "var(--text-secondary)",
                              fontSize: "11px", cursor: "pointer",
                            }}
                            onClick={() => OpenDbExplorerWindow(sessionId, db.name, db.port)}
                          >
                            <FaSearch style={{ fontSize: 9 }} /> {t("panel.explore")}
                          </button>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}
      </div>

      {/* MODAL INTERNA COMPARTIDA: CREACIÓN / EDICIÓN DE TÚNELES */}
      {isModalOpen && (
        <div
          style={{
            position: "fixed",
            inset: 0,
            backgroundColor: "rgba(0, 0, 0, 0.4)",
            backdropFilter: "blur(3px)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 99999,
          }}
        >
          <div
            style={{
              background: "var(--bg-surface)",
              border: "1px solid var(--border)",
              borderRadius: "var(--card-radius)",
              padding: "20px",
              width: "400px",
              boxShadow: "0 10px 25px rgba(0,0,0,0.3)",
            }}
          >
            <h3
              style={{
                fontSize: "14px",
                fontWeight: "600",
                color: "var(--text-primary)",
                marginBottom: "16px",
              }}
            >
              {editingTunnelId
                ? <><FaEdit /> {t("panel.editTunnel")}</>
                : <><IoIosAdd /> {t("panel.configureTunnel")}</>}
            </h3>

            <form
              onSubmit={handleSaveTunnel}
              style={{ display: "flex", flexDirection: "column", gap: "12px" }}
            >
              <div
                style={{ display: "flex", flexDirection: "column", gap: "4px" }}
              >
                <label
                  style={{
                    fontSize: "11px",
                    fontWeight: "600",
                    color: "var(--text-secondary)",
                  }}
                >
                  {t("panel.localListeningPort")}
                </label>
                <input
                  type="number"
                  placeholder={t("panel.localPortPlaceholder")}
                  value={formLocalPort}
                  onChange={(e) => setFormLocalPort(e.target.value)}
                  required
                  style={{
                    background: "var(--bg-elevated)",
                    border: "1px solid var(--border)",
                    borderRadius: "4px",
                    padding: "6px 10px",
                    fontSize: "12px",
                    color: "var(--text-primary)",
                  }}
                />
              </div>

              <div
                style={{ display: "flex", flexDirection: "column", gap: "4px" }}
              >
                <label
                  style={{
                    fontSize: "11px",
                    fontWeight: "600",
                    color: "var(--text-secondary)",
                  }}
                >
                  {t("panel.remoteHost")}
                </label>
                <input
                  type="text"
                  placeholder={t("panel.remoteHostPlaceholder")}
                  value={formRemoteHost}
                  onChange={(e) => setFormRemoteHost(e.target.value)}
                  required
                  style={{
                    background: "var(--bg-elevated)",
                    border: "1px solid var(--border)",
                    borderRadius: "4px",
                    padding: "6px 10px",
                    fontSize: "12px",
                    color: "var(--text-primary)",
                  }}
                />
              </div>

              <div
                style={{ display: "flex", flexDirection: "column", gap: "4px" }}
              >
                <label
                  style={{
                    fontSize: "11px",
                    fontWeight: "600",
                    color: "var(--text-secondary)",
                  }}
                >
                  {t("panel.remotePort")}
                </label>
                <input
                  type="number"
                  placeholder={t("panel.remotePortPlaceholder")}
                  value={formRemotePort}
                  onChange={(e) => setFormRemotePort(e.target.value)}
                  required
                  style={{
                    background: "var(--bg-elevated)",
                    border: "1px solid var(--border)",
                    borderRadius: "4px",
                    padding: "6px 10px",
                    fontSize: "12px",
                    color: "var(--text-primary)",
                  }}
                />
              </div>

              <div
                style={{
                  display: "flex",
                  justifyContent: "flex-end",
                  gap: "8px",
                  marginTop: "12px",
                }}
              >
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  style={{
                    background: "none",
                    border: "1px solid var(--border)",
                    color: "var(--text-secondary)",
                    borderRadius: "4px",
                    padding: "6px 12px",
                    fontSize: "12px",
                    cursor: "pointer",
                  }}
                >
                  {t("common.cancel")}
                </button>
                <button
                  type="submit"
                  style={{
                    background: "var(--accent)",
                    color: "#fff",
                    border: "none",
                    borderRadius: "4px",
                    padding: "6px 16px",
                    fontSize: "12px",
                    fontWeight: "600",
                    cursor: "pointer",
                  }}
                >
                  {editingTunnelId ? t("panel.saveChanges") : t("panel.addTunnel")}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}


    </div>
  );
}
