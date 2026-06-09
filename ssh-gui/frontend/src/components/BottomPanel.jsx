import { useState, useEffect } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
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
} from "../../wailsjs/go/main/App";
import { FaTrash, FaDocker, FaEdit, FaPlay, FaStop, FaDatabase } from "react-icons/fa";
import { LuLogs, LuNetwork } from "react-icons/lu";
import { BiTransfer } from "react-icons/bi";
import { GiWarpPipe } from "react-icons/gi";
import { IoIosAdd } from "react-icons/io";
import { GrRefresh } from "react-icons/gr";

const DB_COLORS = {
  "PostgreSQL":    "#3b82f6",
  "TimescaleDB":   "#3b82f6",
  "MySQL":         "#f97316",
  "MySQL / MariaDB": "#f97316",
  "MariaDB":       "#c084fc",
  "MongoDB":       "#4ade80",
  "Redis":         "#ef4444",
  "Elasticsearch": "#fbbf24",
  "OpenSearch":    "#fbbf24",
  "Cassandra":     "#a855f7",
  "ScyllaDB":      "#a855f7",
  "CouchDB":       "#fb923c",
  "InfluxDB":      "#22d3ee",
  "SQL Server":    "#64748b",
  "Oracle":        "#f43f5e",
  "Neo4j":         "#4ade80",
  "ClickHouse":    "#facc15",
  "CockroachDB":   "#3b82f6",
  "RethinkDB":     "#f97316",
  "ArangoDB":      "#a855f7",
};

const TABS = [
  { id: "logs",      label: "Logs",      icon: <LuLogs /> },
  { id: "transfers", label: "Transfers", icon: <BiTransfer /> },
  { id: "tunnels",   label: "Tunnels",   icon: <GiWarpPipe /> },
  { id: "ports",     label: "Ports",     icon: <LuNetwork /> },
  { id: "docker",    label: "Docker",    icon: <FaDocker /> },
  { id: "databases", label: "Databases", icon: <FaDatabase /> },
];

export default function BottomPanel({ sessionId }) {
  const [activeTab, setActiveTab] = useState("logs");

  // Estados de datos
  const [logs, setLogs] = useState([]);
  const [transfers, setTransfers] = useState({});
  const [tunnels, setTunnels] = useState([]);
  const [containers, setContainers] = useState([]);
  const [ports, setPorts] = useState([]);
  const [databases, setDatabases] = useState([]);


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
    const offLog = EventsOn("log-event", (log) => {
      setLogs((prev) => {
        // Evitar duplicados rápidos en renderizado y limitar a 200 líneas
        return [log, ...prev].slice(0, 200);
      });
    });

    const offTransfer = EventsOn("transfer-status", (data) => {
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
      const intervalId = setInterval(fetchContainers, 4000);
      return () => clearInterval(intervalId);
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

  // --- Operaciones de Docker ---
  const fetchContainers = async () => {
    try {
      const res = await GetDockerContainers(sessionId);
      setContainers(res || []);
    } catch (err) {
      console.error("Error al obtener contenedores:", err);
    }
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
        {TABS.map((t) => (
          <div
            key={t.id}
            className={activeTab === t.id ? "bottom-tab active" : "bottom-tab"}
            onClick={() => setActiveTab(t.id)}
            style={{
              padding: "10px 16px",
              cursor: "pointer",
              fontSize: "13px",
              fontWeight: activeTab === t.id ? "600" : "400",
              color:
                activeTab === t.id ? "var(--accent)" : "var(--text-secondary)",
              borderBottom:
                activeTab === t.id
                  ? "2px solid var(--accent)"
                  : "2px solid transparent",
            }}
          >
            {t.icon}
            {t.label}
            {t.id === "transfers" &&
              Object.values(transfers).filter((t) => t.status === "active")
                .length > 0 && (
                <span
                  className="badge-active-count"
                  style={{
                    marginLeft: "6px",
                    background: "var(--accent)",
                    color: "#fff",
                    borderRadius: "10px",
                    padding: "1px 6px",
                    fontSize: "10px",
                  }}
                >
                  {
                    Object.values(transfers).filter(
                      (t) => t.status === "active",
                    ).length
                  }
                </span>
              )}
          </div>
        ))}

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
              ➕ Nuevo Túnel
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
                padding: "4px 8px",
                fontSize: "11px",
                cursor: "pointer",
                color: "var(--text-secondary)",
              }}
            >
              <FaTrash /> Clear
            </button>
          )}
          {(activeTab === "tunnels" || activeTab === "docker" || activeTab === "ports" || activeTab === "databases") && (
            <button
              className="bottom-action-btn"
              onClick={
                activeTab === "tunnels"   ? fetchTunnels   :
                activeTab === "ports"     ? fetchPorts     :
                activeTab === "databases" ? fetchDatabases :
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
              <GrRefresh /> Refresh
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
              padding: "12px",
              fontFamily: "var(--font-mono, monospace)",
              fontSize: "12px",
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
              containers.map((c) => {
                const isRunning = c.state === "running";

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
                            ? "var(--yellow)"
                            : "var(--accent)",
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
                    </div>
                  </div>
                );
              })
            )}
          </div>
        )}

        {/* ===================== TAB: DATABASES ===================== */}
        {activeTab === "databases" && (
          <div className="docker-tab-content">
            {databases.length === 0 ? (
              <div style={{ padding: "32px", textAlign: "center", color: "var(--text-muted)", fontSize: "13px" }}>
                No se detectaron bases de datos activas
              </div>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
                {databases.map((db, i) => {
                  const isDocker = db.source === "docker";
                  const color = DB_COLORS[db.name] || "var(--accent)";
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
                        <FaDatabase />
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

                      {/* Fuente badge */}
                      <div style={{ flexShrink: 0 }}>
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
