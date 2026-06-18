import { useState, useEffect } from "react";
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
import { FaTrash, FaDocker, FaEdit, FaPlay, FaStop, FaDatabase, FaSearch, FaPlus, FaRedoAlt, FaDesktop, FaSave, FaPause } from "react-icons/fa";
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
import { LuLogs, LuNetwork } from "react-icons/lu";
import { BiTransfer } from "react-icons/bi";
import { GiWarpPipe } from "react-icons/gi";
import { IoIosAdd } from "react-icons/io";
import { GrRefresh, GrAdd, GrFormRefresh } from "react-icons/gr";

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
  { id: "logs",       label: "Logs",       icon: <LuLogs /> },
  { id: "transfers",  label: "Transfers",  icon: <BiTransfer /> },
  { id: "tunnels",    label: "Tunnels",    icon: <GiWarpPipe /> },
  { id: "ports",      label: "Ports",      icon: <LuNetwork /> },
  { id: "docker",     label: "Docker",     icon: <FaDocker /> },
  { id: "virtualbox", label: "VirtualBox", icon: <FaDesktop /> },
  { id: "databases",  label: "Databases",  icon: <FaDatabase />, pro: true },
];

function StatPill({ label, value, sub, warn = false }) {
  const color = warn ? "var(--red)" : "var(--accent)";
  const bg    = warn ? "rgba(239,68,68,0.1)" : "rgba(99,102,241,0.08)";
  return (
    <span style={{
      display: "inline-flex", alignItems: "baseline", gap: 4,
      padding: "1px 7px", borderRadius: 4,
      background: bg, fontSize: 10.5,
    }}>
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

export default function BottomPanel({ sessionId, accountStatus, onUpgrade }) {
  const isPro = accountStatus?.tier === "pro";
  const [activeTab, setActiveTab] = useState("logs");

  // Estados de datos
  const [logs, setLogs] = useState([]);
  const [transfers, setTransfers] = useState({});
  const [tunnels, setTunnels] = useState([]);
  const [containers, setContainers] = useState([]);
  const [ports, setPorts] = useState([]);
  const [databases, setDatabases] = useState([]);
  const [vms, setVms] = useState([]);
  const [vboxError, setVboxError] = useState(null);
  const [vboxActionLoading, setVboxActionLoading] = useState(null);
  const [dockerStats, setDockerStats] = useState({}); // map containerID → stats


  // Estados para la Modal del Túnel (Crea y Edita)
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingTunnelId, setEditingTunnelId] = useState(null);
  const [formLocalPort, setFormLocalPort] = useState("");
  const [formRemoteHost, setFormRemoteHost] = useState("127.0.0.1");
  const [formRemotePort, setFormRemotePort] = useState("");

  const stateOrder = {
    running: 0,
    exited: 1,
  };

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
      const intStats      = setInterval(fetchStats, 3000);
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
      alert("Error al procesar operación del túnel: " + err);
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
    if (!window.confirm("¿Seguro que deseas eliminar este túnel configurado?"))
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
      // Ordenar por número de puerto
      const sorted = (res || []).sort((a, b) => a.port - b.port);
      setPorts(sorted);
    } catch (err) {
      console.error("Error al obtener puertos:", err);
    }
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
      setVboxError(typeof err === "string" ? err : err?.message || "VirtualBox no disponible");
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
    } catch (_) {}
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
        {TABS.map((t) => {
          const locked = t.pro && !isPro;
          const isActive = activeTab === t.id;
          return (
            <div
              key={t.id}
              className={isActive ? "bottom-tab active" : "bottom-tab"}
              onClick={() => locked ? onUpgrade?.() : setActiveTab(t.id)}
              title={locked ? "Disponible en plan Pro" : undefined}
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
              {t.icon}
              {t.label}
              {locked && (
                <span style={{ fontSize: 9, marginLeft: 2, opacity: 0.8 }}>🔒</span>
              )}
              {t.id === "transfers" &&
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

        <div
          className="bottom-tab-actions"
          style={{
            marginLeft: "auto",
            display: "flex",
            alignItems: "center",
            paddingRight: "10px",
            gap: "8px",
          }}
        >
          {activeTab === "tunnels" && (
            <button
            className="bottom-action-btn"
              onClick={openCreateModal}
              style={{
                background: "var(--accent)",
                color: "#fff",
                border: "none",
                borderRadius: "4px",
                padding: "4px 10px",
                fontSize: "11px",
                fontWeight: "600",
                cursor: "pointer",
              }}
            >
              <FaPlus /> Nuevo Túnel
            </button>
          )}
          {activeTab === "logs" && (
            <button
              className="bottom-action-btn"
              onClick={() => setLogs([])}
              style={{
                background: "none",
                border: "1px solid var(--border)",
                borderRadius: "4px",
                padding: "4px 10px",
                fontSize: "11px",
                fontWeight: "600",
                cursor: "pointer",
                color: "var(--text-secondary)",
              }}
            >
              <FaTrash /> Clear
            </button>
          )}
          {(activeTab === "tunnels" || activeTab === "docker" || activeTab === "ports" || activeTab === "databases" || activeTab === "virtualbox") && (
            <button
              className="bottom-action-btn"
              onClick={
                activeTab === "tunnels"    ? fetchTunnels   :
                activeTab === "ports"      ? fetchPorts     :
                activeTab === "databases"  ? fetchDatabases :
                activeTab === "virtualbox" ? fetchVMs       :
                fetchContainers
              }
              style={{
                background: "none",
                border: "1px solid var(--border)",
                borderRadius: "4px",
                padding: "4px 8px",
                fontSize: "11px",
                cursor: "pointer",
                color: "var(--text-secondary)",
              }}
            >
              <FaRedoAlt /> Refresh
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
                No hay eventos registrados en esta sesión.
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
                No se registran transferencias de archivos.
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
                    {t.direction === "upload" ? "📤" : "📥"}
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
                No hay túneles configurados. Pulsa "Nuevo Túnel" para añadir
                uno.
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
                    <th style={{ padding: "8px 12px" }}>Puerto Local (PC)</th>
                    <th>Destino Remoto</th>
                    <th>Estado</th>
                    <th style={{ textAlign: "right", paddingRight: "12px" }}>
                      Acciones
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {tunnels.map((t) => (
                    <tr
                      key={t.id}
                      style={{ borderBottom: "1px solid var(--border-subtle)" }}
                    >
                      <td
                        style={{
                          padding: "10px 12px",
                          fontWeight: "600",
                          color: "var(--blue)",
                        }}
                      >
                        127.0.0.1:{t.localPort}
                      </td>
                      <td style={{ color: "var(--text-primary)" }}>
                        {t.remoteHost}:{t.remotePort}
                      </td>
                      <td>
                        <span
                          style={{
                            padding: "3px 8px",
                            borderRadius: "12px",
                            fontSize: "11px",
                            fontWeight: "500",
                            backgroundColor: t.active
                              ? "rgba(34,197,94,0.12)"
                              : "rgba(107,114,128,0.1)",
                            color: t.active
                              ? "var(--green)"
                              : "var(--text-secondary)",
                          }}
                        >
                          {t.active ? "● Escuchando" : "○ Inactivo"}
                        </span>
                      </td>
                      <td style={{ textAlign: "right", paddingRight: "12px" }}>
                        <button
                          onClick={() => handleToggleTunnel(t)}
                          style={{
                            padding: "3px 10px",
                            cursor: "pointer",
                            backgroundColor: t.active
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
                          {t.active ? "Apagar" : "Encender"}
                        </button>
                        <button
                          onClick={() => openEditModal(t)}
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
                          title="Editar"
                        >
                          ✏️
                        </button>
                        <button
                          onClick={() => handleDeleteTunnel(t.id)}
                          style={{
                            background: "none",
                            border: "1px solid var(--border)",
                            color: "var(--text-secondary)",
                            borderRadius: "4px",
                            padding: "2px 6px",
                            cursor: "pointer",
                            fontSize: "11px",
                          }}
                          title="Eliminar"
                        >
                          🗑️
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
                No se encontraron puertos en escucha.
              </div>
            ) : (
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "12px" }}>
                <thead>
                  <tr style={{ color: "var(--text-secondary)", background: "var(--bg-hover)", borderBottom: "1px solid var(--border)" }}>
                    <th style={{ padding: "7px 12px", textAlign: "left", width: "70px" }}>Proto</th>
                    <th style={{ padding: "7px 12px", textAlign: "left", width: "80px" }}>Port</th>
                    <th style={{ padding: "7px 12px", textAlign: "left" }}>Address</th>
                    <th style={{ padding: "7px 12px", textAlign: "left" }}>Process</th>
                  </tr>
                </thead>
                <tbody>
                  {ports.map((p, i) => (
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
              padding: "10px 14px",
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
                No se encontraron contenedores remotos.
              </div>
            ) : (
              containers
              .sort((a,b) => stateOrder[a.state] - stateOrder[b.state] || a.names.localeCompare(b.names))
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
                        flex: "1 1 40%",
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
                      {c.ports && (
                        <div style={{ display: "flex", flexWrap: "wrap", gap: "3px", marginTop: "2px" }}>
                          {c.ports.split(", ").filter(p => p && !p.startsWith(":::")).map((p, i) => (
                            <span key={i} className="docker-port-badge">
                              {p.replace("0.0.0.0:", "").replace(/->(\d+)\/tcp/, "→$1")}
                            </span>
                          ))}
                        </div>
                      )}
                      {/* Stats de CPU y RAM (solo contenedores running) */}
                      {st && (
                        <div style={{ display: "flex", gap: 6, marginTop: 4, flexWrap: "wrap" }}>
                          <StatPill
                            label="CPU"
                            value={st.cpuPerc}
                            warn={parseFloat(st.cpuPerc) > 80}
                          />
                          <StatPill
                            label="RAM"
                            value={st.memUsage}
                            sub={st.memPerc}
                            warn={parseFloat(st.memPerc) > 80}
                          />
                        </div>
                      )}
                    </div>

                    {/* 2. ESTADO INTEGRADO (Badge + Mensaje juntos en un bloque) */}
                    <div
                      style={{
                        flex: "1 1 40%",
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
                        title="Ver logs en ventana externa"
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
                        {isRunning ? <FaStop /> : <FaPlay /> }
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
              })
            )}
          </div>
        )}

        {/* ===================== TAB: VIRTUALBOX ===================== */}
        {activeTab === "virtualbox" && (
          <div style={{ padding: "10px 14px", display: "flex", flexDirection: "column", gap: 8 }}>
            {vboxError ? (
              <div style={{ padding: "32px", textAlign: "center", color: "var(--text-muted)", fontSize: 13 }}>
                <FaDesktop style={{ fontSize: 28, opacity: 0.3, display: "block", margin: "0 auto 10px" }} />
                VirtualBox no está instalado en este servidor
                <div style={{ fontSize: 11, marginTop: 6, opacity: 0.7 }}>{vboxError}</div>
              </div>
            ) : vms.length === 0 ? (
              <div style={{ padding: "32px", textAlign: "center", color: "var(--text-muted)", fontSize: 13 }}>
                No se encontraron máquinas virtuales
              </div>
            ) : vms.map(vm => {
              const isRunning  = vm.state === "running";
              const isPaused   = vm.state === "paused";
              const isSaved    = vm.state === "saved";
              const isStopped  = vm.state === "stopped" || vm.state === "aborted";
              const isStuck    = isRunning || isPaused; // can force-stop
              const isLoading  = vboxActionLoading?.startsWith(vm.name + ":");

              const stateColor = isRunning  ? "var(--green)"
                : isPaused   ? "var(--yellow)"
                : isSaved    ? "#60a5fa"
                : vm.state === "error" || vm.state === "aborted" ? "var(--red)"
                : "var(--text-muted)";

              const stateBg = isRunning  ? "rgba(34,197,94,0.12)"
                : isPaused   ? "rgba(250,204,21,0.12)"
                : isSaved    ? "rgba(96,165,250,0.12)"
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
                    opacity: isLoading ? 0.65 : 1,
                    transition: "opacity .2s",
                  }}
                >
                  {/* ── Fila superior: nombre + estado + forzar apagado ── */}
                  <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8 }}>
                    <FaDesktop style={{ color: stateColor, fontSize: 14, flexShrink: 0 }} />

                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontWeight: 700, fontSize: 13, color: "var(--text-primary)",
                        overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        {vm.name}
                      </div>
                      {vm.uuid && (
                        <div style={{ fontSize: 9.5, fontFamily: "monospace", color: "var(--text-muted)",
                          overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", marginTop: 1 }}>
                          {vm.uuid}
                        </div>
                      )}
                    </div>

                    <span style={{
                      padding: "2px 8px", borderRadius: 4, flexShrink: 0,
                      fontSize: 10, fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.5px",
                      background: stateBg, color: stateColor,
                    }}>
                      {vm.state}
                    </span>

                    {/* Forzar apagado — siempre visible si la VM no está detenida */}
                    {isStuck && (
                      <ActionBtn
                        title="Forzar apagado (poweroff) — úsalo si la VM está colgada"
                        color="var(--red)"
                        disabled={isLoading}
                        onClick={() => {
                          if (window.confirm(`¿Forzar apagado de "${vm.name}"?\n\nEquivalente a desenchufar la corriente — se perderán los cambios no guardados.`))
                            handleVMAction(vm.name, "stop-force");
                        }}
                      >
                        ⚡
                      </ActionBtn>
                    )}
                  </div>

                  {/* ── Specs ── */}
                  <div style={{ display: "flex", flexWrap: "wrap", gap: "6px 16px", marginBottom: 10 }}>
                    {ram && <Spec icon="🧠" label={ram} />}
                    {vm.cpus > 0 && <Spec icon="⚙️" label={`${vm.cpus} vCPU${vm.cpus !== 1 ? "s" : ""}`} />}
                    {vm.os && <Spec icon="💿" label={vm.os} />}
                    {vm.ip && <Spec icon="🌐" label={vm.ip} mono />}
                    {isRunning && !vm.ip && (
                      <Spec icon="🌐" label="IP no disponible (Guest Additions)" muted />
                    )}
                  </div>

                  {/* ── Acciones ── */}
                  <div style={{ display: "flex", alignItems: "center", gap: 5, flexWrap: "wrap" }}>
                    {(isStopped || isSaved) && (
                      <>
                        <ActionBtn title="Iniciar (con interfaz gráfica)" color="var(--green)"
                          disabled={isLoading} onClick={() => handleVMAction(vm.name, "start-gui")}>
                          <FaPlay style={{ marginRight: 4 }} /> GUI
                        </ActionBtn>
                        <ActionBtn title="Iniciar en modo headless (sin pantalla)" color="var(--accent)"
                          disabled={isLoading} onClick={() => handleVMAction(vm.name, "start-headless")}>
                          <FaDesktop style={{ marginRight: 4 }} /> Headless
                        </ActionBtn>
                      </>
                    )}

                    {isRunning && (
                      <>
                        <ActionBtn title="Pausar" color="var(--yellow)" disabled={isLoading}
                          onClick={() => handleVMAction(vm.name, "pause")}>
                          <FaPause style={{ marginRight: 4 }} /> Pausar
                        </ActionBtn>
                        <ActionBtn title="Guardar estado y apagar" color="#60a5fa" disabled={isLoading}
                          onClick={() => handleVMAction(vm.name, "savestate")}>
                          <FaSave style={{ marginRight: 4 }} /> Guardar estado
                        </ActionBtn>
                        <ActionBtn title="Reiniciar (equivale a resetear el hardware)" color="var(--yellow)"
                          disabled={isLoading} onClick={() => handleVMAction(vm.name, "reset")}>
                          <GrFormRefresh style={{ marginRight: 4 }} /> Reiniciar
                        </ActionBtn>
                        <ActionBtn title="Enviar señal ACPI de apagado (apagado suave)" color="var(--red)"
                          disabled={isLoading} onClick={() => handleVMAction(vm.name, "stop-acpi")}>
                          <FaStop style={{ marginRight: 4 }} /> Apagar
                        </ActionBtn>
                      </>
                    )}

                    {isPaused && (
                      <ActionBtn title="Reanudar ejecución" color="var(--green)" disabled={isLoading}
                        onClick={() => handleVMAction(vm.name, "resume")}>
                        <FaPlay style={{ marginRight: 4 }} /> Reanudar
                      </ActionBtn>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* ===================== TAB: DATABASES ===================== */}
        {activeTab === "databases" && !isPro && (
          <div style={{
            display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center",
            height: "100%", gap: 12, padding: 32, textAlign: "center",
          }}>
            <FaDatabase style={{ fontSize: 28, color: "var(--text-muted)", opacity: 0.5 }} />
            <div style={{ fontWeight: 600, fontSize: 14, color: "var(--text-primary)" }}>
              Database Explorer — Plan Pro
            </div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", maxWidth: 280, lineHeight: 1.6 }}>
              Detecta y explora bases de datos en tus servidores remotos. Disponible en el plan Pro.
            </div>
            <button
              onClick={onUpgrade}
              style={{
                marginTop: 4, padding: "6px 18px", borderRadius: 8,
                background: "var(--accent)", color: "#fff",
                border: "none", cursor: "pointer", fontSize: 12, fontWeight: 600,
              }}
            >
              Ver planes →
            </button>
          </div>
        )}
        {activeTab === "databases" && isPro && (
          <div className="docker-tab-content">
            {databases.length === 0 ? (
              <div style={{ padding: "32px", textAlign: "center", color: "var(--text-muted)", fontSize: "13px" }}>
                No se detectaron bases de datos activas
              </div>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
                {databases.map((db, i) => {
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
                      {db.port > 0 && (
                        <div style={{ textAlign: "right", flexShrink: 0 }}>
                          <div style={{ fontSize: "10px", color: "var(--text-muted)", marginBottom: "2px" }}>Puerto</div>
                          <div style={{ fontFamily: "var(--font-mono, monospace)", fontSize: "13px", fontWeight: "600", color }}>
                            {db.port}
                          </div>
                          {db.address && !db.address.startsWith("0.0.0.0") && (
                            <div style={{ fontSize: "9.5px", fontFamily: "var(--font-mono, monospace)", color: "var(--text-muted)" }}>
                              {db.address}
                            </div>
                          )}
                        </div>
                      )}

                      {/* Fuente badge + Explorar */}
                      <div style={{ flexShrink: 0, display: "flex", flexDirection: "column", alignItems: "flex-end", gap: "6px" }}>
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
                          {isDocker ? "Docker" : "Sistema"}
                        </span>
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
                          <FaSearch style={{ fontSize: 9 }} /> Explorar
                        </button>
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
                ? <FaEdit /> + " Editar Parámetros del Túnel"
                : <IoIosAdd /> + "➕ Configurar Nuevo Túnel Port-Forwarding"}
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
                  Puerto Local de Escucha (En tu máquina)
                </label>
                <input
                  type="number"
                  placeholder="Ej: 8080"
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
                  Host Remoto Destino (Desde la perspectiva del servidor)
                </label>
                <input
                  type="text"
                  placeholder="Ej: 127.0.0.1 o localhost"
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
                  Puerto Remoto Destino
                </label>
                <input
                  type="number"
                  placeholder="Ej: 80 o 3306"
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
                  Cancelar
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
                  {editingTunnelId ? "Guardar Cambios" : "Agregar Túnel"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}


    </div>
  );
}
