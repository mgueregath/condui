import { useEffect, useRef, useState } from "react";

import { Terminal } from "xterm";
import { FitAddon } from "xterm-addon-fit";

import "xterm/css/xterm.css";

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
} from "../wailsjs/go/main/App";

import TabBar from "./components/TabBar";
import BottomPanel from "./components/BottomPanel";

import { useConnections } from "./hooks/useConnections";

import Modal from "./components/common/Modal";

import ConnectionDrawer from "./components/connections/ConnectionDrawer";

import FolderModal from "./components/connections/FolderModal";

import ConnectionModal from "./components/connections/ConnectionModal";

import "./components/Layout.css";

function App() {
  const terminalRef = useRef(null);
  const termRef = useRef(null);

  const terminalBuffers = useRef({});

  const [tabs, setTabs] = useState([]);
  const [activeTab, setActiveTab] = useState(null);

  const { folders, connections, reload } = useConnections();

  const [connectionsOpen, setConnectionsOpen] = useState(false);

  const [folderModalOpen, setFolderModalOpen] = useState(false);

  const [connectionModalOpen, setConnectionModalOpen] = useState(false);

  const [expandedFolders, setExpandedFolders] = useState([]);

  const [editingFolder, setEditingFolder] = useState(null);

  const [editingConnection, setEditingConnection] = useState(null);

  useEffect(() => {
    const term = new Terminal({
      cursorBlink: true,
      convertEol: true,
    });

    const fitAddon = new FitAddon();

    term.loadAddon(fitAddon);

    term.open(terminalRef.current);

    fitAddon.fit();

    termRef.current = term;

    const notifyResize = () => {
      if (!activeTab || !termRef.current) {
        return;
      }

      ResizeTerminal(activeTab, term.rows, term.cols);
    };

    term.onData((data) => {
      if (!activeTab) {
        return;
      }

      SendInput(activeTab, data);
    });

    EventsOn("terminal-output", (payload) => {
      if (!terminalBuffers.current[payload.sessionId]) {
        terminalBuffers.current[payload.sessionId] = "";
      }

      terminalBuffers.current[payload.sessionId] += payload.data;

      if (payload.sessionId !== activeTab) {
        return;
      }

      term.write(payload.data);
    });

    const resizeHandler = () => {
      fitAddon.fit();

      notifyResize();
    };

    window.addEventListener("resize", resizeHandler);

    return () => {
      window.removeEventListener("resize", resizeHandler);

      term.dispose();
    };
  }, [activeTab]);

  const connect = async (connectionId) => {
    const sessionId = await ConnectSSH(connectionId);

    setTabs((prev) => [
      ...prev,
      {
        id: sessionId,
        title: `SSH ${prev.length + 1}`,
      },
    ]);

    setActiveTab(sessionId);

    setTimeout(() => {
      if (!termRef.current) {
        return;
      }

      ResizeTerminal(sessionId, termRef.current.rows, termRef.current.cols);
    }, 200);
  };

  useEffect(() => {
    if (!activeTab || !termRef.current) {
      return;
    }

    termRef.current.clear();

    termRef.current.write(terminalBuffers.current[activeTab] || "");

    ResizeTerminal(activeTab, termRef.current.rows, termRef.current.cols);
  }, [activeTab]);

  return (
    <div
      style={{
        width: "100vw",
        height: "100vh",
        display: "flex",
        flexDirection: "column",
        background: "#181a1f",
        color: "#fff",
      }}
    >
      <div
        style={{
          height: "42px",
          display: "flex",
          alignItems: "center",
          gap: "12px",
          padding: "0 12px",
          borderBottom: "1px solid #333",
          background: "#202329",
        }}
      >
        <button onClick={() => setConnectionsOpen(!connectionsOpen)}>☰</button>

        <button onClick={connect}>+</button>

        <button>📁</button>

        <button>🔀</button>

        <button>⚙</button>
      </div>

      <div
        style={{
          flex: 1,
          display: "flex",
          position: "relative",
          overflow: "hidden",
        }}
      >
        {connectionsOpen && (
          <>
            <div
              onClick={() => setConnectionsOpen(false)}
              style={{
                position: "absolute",
                inset: 0,
                background: "rgba(0,0,0,0.35)",
                zIndex: 50,
              }}
            />

            <div
              style={{
                position: "absolute",

                left: 0,
                top: 0,
                bottom: 0,

                width: "320px",

                background: "#202329",

                borderRight: "1px solid #333",

                overflow: "auto",

                zIndex: 100,

                boxShadow: "0 0 25px rgba(0,0,0,0.4)",
              }}
            >
              <div
                style={{
                  padding: "12px",
                  fontWeight: "bold",
                }}
              >
                Connections
              </div>
              <div
                style={{
                  display: "flex",
                  gap: "8px",
                  padding: "12px",
                  borderBottom: "1px solid #333",
                }}
              >
                <button
                  onClick={() => {
                    setEditingFolder(null);

                    setFolderModalOpen(true);
                  }}
                >
                  + Folder
                </button>

                <button
                  onClick={() => {
                    setEditingConnection(null);

                    setConnectionModalOpen(true);
                  }}
                >
                  + Connection
                </button>
              </div>

              <ConnectionDrawer
                folders={folders}
                connections={connections}
                expandedFolders={expandedFolders}
                onToggleFolder={(id) => {
                  if (expandedFolders.includes(id)) {
                    setExpandedFolders((prev) => prev.filter((x) => x !== id));
                  } else {
                    setExpandedFolders((prev) => [...prev, id]);
                  }
                }}
                onEditFolder={(folder) => {
                  setEditingFolder(folder);

                  setFolderModalOpen(true);
                }}
                onDeleteFolder={async (folder) => {
                  try {
                    const confirmed = confirm(
                      `Eliminar carpeta ${folder.name}?`,
                    );

                    if (!confirmed) {
                      return;
                    }

                    await DeleteFolder(folder.id);

                    await reload();
                  } catch (err) {
                    console.error(err);
                  }
                }}
                onOpenConnection={async (c) => {
                  const existingTab = tabs.find((t) => t.connectionId === c.id);

                  if (existingTab) {
                    setActiveTab(existingTab.id);

                    return;
                  }

                  try {
                    const sessionId = await ConnectSSH(c.id);

                    setTabs((prev) => [
                      ...prev,
                      {
                        id: sessionId,
                        connectionId: c.id,
                        title: c.name,
                      },
                    ]);

                    setActiveTab(sessionId);
                  } catch (err) {
                    console.error(err);
                  }
                }}
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
              />
            </div>
          </>
        )}

        <div
          style={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
          }}
        >
          <TabBar
  tabs={tabs}
  activeTab={activeTab}
  onSelect={setActiveTab}
  onClose={(tabId) => {

    setTabs(prev =>
      prev.filter(
        t => t.id !== tabId
      )
    );

    if (activeTab === tabId) {

      const remaining =
        tabs.filter(
          t => t.id !== tabId
        );

      setActiveTab(
        remaining.length
          ? remaining[0].id
          : null
      );

    }

  }}
/>

          <div
            style={{
              flex: 1,
              display: "flex",
            }}
          >
            <div
              style={{
                width: "280px",
                borderRight: "1px solid #333",
                background: "#202329",
              }}
            >
              <div
                style={{
                  padding: "10px",
                  borderBottom: "1px solid #333",
                }}
              >
                Remote Files
              </div>

              <RemoteFileTree sessionId={activeTab} />
            </div>

            <div
              style={{
                flex: 1,
                position: "relative",
              }}
            >
              <div
                ref={terminalRef}
                style={{
                  position: "absolute",
                  inset: 0,
                }}
              />
            </div>
          </div>

          <div
            style={{
              height: "120px",
              borderTop: "1px solid #333",
              background: "#202329",
            }}
          >
            <BottomPanel />
          </div>
        </div>
      </div>

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
    </div>
  );
}

export default App;
