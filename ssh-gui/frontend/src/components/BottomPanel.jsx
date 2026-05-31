import { useState } from "react";

export default function BottomPanel() {

  const [activeTab,
    setActiveTab] =
    useState("logs");

  const tabs = [
    {
      id: "logs",
      label: "Logs",
      icon: "📋",
    },
    {
      id: "transfers",
      label: "Transfers",
      icon: "📦",
    },
    {
      id: "tunnels",
      label: "Tunnels",
      icon: "🔀",
    },
    {
      id: "metrics",
      label: "Metrics",
      icon: "📊",
    },
  ];

  return (

    <div className="bottom-panel">

      <div className="bottom-tabs">

        {tabs.map(tab => (

          <div
            key={tab.id}
            onClick={() =>
              setActiveTab(tab.id)
            }
            style={{
              display: "flex",
              alignItems: "center",
              gap: "8px",

              cursor: "pointer",

              padding:
                "8px 14px",

              borderRadius:
                "10px",

              background:
                activeTab ===
                tab.id
                  ? "#dbeafe"
                  : "transparent",

              color:
                activeTab ===
                tab.id
                  ? "#2563eb"
                  : "#6b7280",

              fontWeight:
                activeTab ===
                tab.id
                  ? 600
                  : 500,
            }}
          >

            <span>
              {tab.icon}
            </span>

            <span>
              {tab.label}
            </span>

          </div>

        ))}

      </div>

      <div
        className="bottom-content"
        style={{
          height: "100%",
        }}
      >

        {activeTab ===
          "logs" && (

          <div>

            <div
              style={{
                fontWeight: 600,
                marginBottom:
                  "12px",
              }}
            >
              Session Activity
            </div>

            <div
              style={{
                color:
                  "#6b7280",
                fontSize:
                  "14px",
              }}
            >
              Connection logs
              will appear here.
            </div>

          </div>

        )}

        {activeTab ===
          "transfers" && (

          <div>

            <div
              style={{
                fontWeight: 600,
                marginBottom:
                  "12px",
              }}
            >
              File Transfers
            </div>

            <div
              style={{
                color:
                  "#6b7280",
                fontSize:
                  "14px",
              }}
            >
              Uploads and
              downloads will
              appear here.
            </div>

          </div>

        )}

        {activeTab ===
          "tunnels" && (

          <div>

            <div
              style={{
                fontWeight: 600,
                marginBottom:
                  "12px",
              }}
            >
              SSH Tunnels
            </div>

            <div
              style={{
                color:
                  "#6b7280",
                fontSize:
                  "14px",
              }}
            >
              Active tunnels
              will appear here.
            </div>

          </div>

        )}

        {activeTab ===
          "metrics" && (

          <div>

            <div
              style={{
                display: "flex",
                gap: "16px",
              }}
            >

              <MetricCard
                label="Sessions"
                value="1"
              />

              <MetricCard
                label="Transfers"
                value="0"
              />

              <MetricCard
                label="Tunnels"
                value="0"
              />

              <MetricCard
                label="Latency"
                value="18ms"
              />

            </div>

          </div>

        )}

      </div>

    </div>

  );

}

function MetricCard({
  label,
  value,
}) {

  return (

    <div
      style={{
        minWidth: "120px",

        padding: "14px",

        border:
          "1px solid #e5e7eb",

        borderRadius:
          "12px",

        background:
          "#ffffff",
      }}
    >

      <div
        style={{
          fontSize: "12px",

          color: "#6b7280",
        }}
      >
        {label}
      </div>

      <div
        style={{
          fontSize: "22px",

          fontWeight: 700,

          marginTop: "4px",
        }}
      >
        {value}
      </div>

    </div>

  );

}