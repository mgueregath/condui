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
} from "../../wailsjs/go/main/App";

const TABS = [
  { id: "logs", label: "Logs" },
  { id: "transfers", label: "Transfers" },
  { id: "tunnels", label: "Tunnels" },
  { id: "docker", label: "Docker" },
];

export default function BottomPanel({ sessionId }) {
  const [activeTab, setActiveTab] = useState("logs");

  // Estados de datos
  const [logs, setLogs] = useState([]);
  const [transfers, setTransfers] = useState({});
  const [tunnels, setTunnels] = useState([]);
  const [containers, setContainers] = useState([]);

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
      fetchContainers(); // Carga inicial inmediata

      // Configura el intervalo de refresco automático cada 4 segundos
      const intervalId = setInterval(() => {
        fetchContainers();
      }, 4000);

      // Limpiar el intervalo cuando cambies de pestaña o se cierre la sesión
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
          background: "var(--bg-surface)",
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
              🗑 Clear
            </button>
          )}
          {(activeTab === "tunnels" || activeTab === "docker") && (
            <button
              className="bottom-action-btn"
              onClick={activeTab === "tunnels" ? fetchTunnels : fetchContainers}
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
              🔄 Refresh
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
          background: "var(--bg-panel)",
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
                      }}
                    >
                      <button
                        onClick={() => handleToggleContainer(c.id, c.state)}
                        style={{
                          padding: "5px 12px",
                          cursor: "pointer",
                          backgroundColor: isRunning
                            ? "var(--yellow)"
                            : "var(--accent)",
                          color: "#fff",
                          border: "none",
                          borderRadius: "4px",
                          fontSize: "11px",
                          fontWeight: "600",
                          minWidth: "75px",
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
                        {isRunning ? "🛑 Stop" : "▶ Start"}
                      </button>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        )}
        .
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
                ? "✏️ Editar Parámetros del Túnel"
                : "➕ Configurar Nuevo Túnel Port-Forwarding"}
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
