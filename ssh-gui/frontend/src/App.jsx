import { useEffect, useRef, useState } from "react";

import { Terminal } from "xterm";
import { FitAddon } from "xterm-addon-fit";

import "xterm/css/xterm.css";

import { EventsOn } from "../wailsjs/runtime/runtime";

import { ConnectSSH, SendInput, ResizeTerminal } from "../wailsjs/go/main/App";

function App() {
  const terminalRef = useRef(null);

  const terminalBuffers = useRef({});

  const [tabs, setTabs] = useState([]);

  const [activeTab, setActiveTab] = useState(null);

  const termRef = useRef(null);

  useEffect(() => {
    const term = new Terminal({
      cursorBlink: true,
      convertEol: true,
    });

    const fitAddon = new FitAddon();

    term.loadAddon(fitAddon);

    term.open(terminalRef.current);

    fitAddon.fit();

    const notifyResize = () => {
      if (!activeTab || !termRef.current) {
        return;
      }

      ResizeTerminal(activeTab, term.rows, term.cols);
    };

    termRef.current = term;

    term.writeln("ModernTerm");

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
        background: "#1e1f22",
      }}
    >
      <div
        style={{
          padding: "10px",
          borderBottom: "1px solid #333",
        }}
      >
        <button onClick={connect}>Nueva conexión</button>
      </div>

      <div
        style={{
          display: "flex",
          borderBottom: "1px solid #333",
        }}
      >
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            style={{
              padding: "8px 16px",
              background: activeTab === tab.id ? "#2d2f34" : "#1e1f22",
              color: "#fff",
              border: "none",
            }}
          >
            {tab.title}
          </button>
        ))}
      </div>

      <div
        ref={terminalRef}
        style={{
          flex: 1,
          overflow: "hidden",
        }}
      />
    </div>
  );
}

export default App;
