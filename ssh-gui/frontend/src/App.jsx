import { useEffect, useRef, useState } from "react";

import { Terminal } from "xterm";
import { FitAddon } from "xterm-addon-fit";

import "xterm/css/xterm.css";

import { EventsOn } from "../wailsjs/runtime/runtime";

import { ConnectSSH, SendInput, ResizeTerminal } from "../wailsjs/go/main/App";

import TabBar from "./components/TabBar";
import BottomPanel from "./components/BottomPanel";

import "./components/Layout.css";

function App() {
  const terminalRef = useRef(null);
  const termRef = useRef(null);

  const terminalBuffers = useRef({});

  const [tabs, setTabs] = useState([]);
  const [activeTab, setActiveTab] = useState(null);

  const [connectionsOpen, setConnectionsOpen] = useState(false);

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

  const connect = async () => {
    const sessionId = await ConnectSSH();

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
                  padding: "12px",
                }}
              >
                ▼ Producción
              </div>

              <div
                style={{
                  paddingLeft: "24px",
                }}
              >
                Servidor 1
              </div>

              <div
                style={{
                  paddingLeft: "24px",
                }}
              >
                Servidor 2
              </div>

              <div
                style={{
                  padding: "12px",
                }}
              >
                ▼ Desarrollo
              </div>

              <div
                style={{
                  paddingLeft: "24px",
                }}
              >
                QA
              </div>
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
          <TabBar tabs={tabs} activeTab={activeTab} onSelect={setActiveTab} />

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

              <div
                style={{
                  padding: "10px",
                }}
              >
                📁 /root
                <br />
                ├── docker-compose.yml
                <br />
                ├── logs
                <br />
                ├── scripts
                <br />
                └── models
              </div>
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
    </div>
  );
}

export default App;
