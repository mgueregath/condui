import { FaEdit } from "react-icons/fa";
import { IoIosAdd } from "react-icons/io";
import { useTranslation } from "react-i18next";
import Modal from "./common/Modal";
import "./TunnelModal.css";

export default function TunnelModal({
  open,
  editingTunnelId,
  formLocalPort,
  setFormLocalPort,
  formRemoteHost,
  setFormRemoteHost,
  formRemotePort,
  setFormRemotePort,
  onSubmit,
  onClose,
}) {
  const { t } = useTranslation();

  return (
    <Modal open={open} onClose={onClose} className="tunnel-modal">
      <div style={{ padding: "20px" }}>
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
          onSubmit={onSubmit}
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
              onClick={onClose}
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
    </Modal>
  );
}
