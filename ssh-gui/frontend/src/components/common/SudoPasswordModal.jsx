import { useState } from "react";
import { useTranslation } from "react-i18next";
import Modal from "./Modal";

export default function SudoPasswordModal({ open, onClose, onSubmit }) {
  const { t } = useTranslation();
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  const handleSubmit = async () => {
    setError("");
    try {
      await onSubmit(password);
      setPassword("");
      setError("");
    } catch (err) {
      setError(t("panel.sudoPasswordError"));
    }
  };

  const handleClose = () => {
    setPassword("");
    setError("");
    onClose();
  };

  return (
    <Modal open={open} onClose={handleClose}>
      <div>
        <div className="modal-header">
          <h2>{t("panel.sudoPasswordTitle")}</h2>
        </div>

        <div className="modal-body">
          <p
            style={{
              margin: "0 0 16px 0",
              fontSize: "13px",
              color: "var(--text-secondary)",
              lineHeight: "1.5",
            }}
          >
            {t("panel.sudoPasswordMessage")}
          </p>

          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                handleSubmit();
              }
            }}
            placeholder={t("panel.sudoPasswordPlaceholder")}
            autoFocus
            style={{
              width: "100%",
              padding: "8px 12px",
              background: "var(--bg-secondary)",
              border: "1px solid var(--border)",
              borderRadius: "4px",
              color: "var(--text-primary)",
              fontSize: "13px",
              marginBottom: error ? "8px" : "0",
            }}
          />

          {error && (
            <div
              style={{
                padding: "8px 12px",
                background: "rgba(239, 68, 68, 0.1)",
                border: "1px solid rgba(239, 68, 68, 0.3)",
                borderRadius: "4px",
                color: "#ef4444",
                fontSize: "12px",
              }}
            >
              {error}
            </div>
          )}
        </div>

        <div className="modal-footer">
          <button className="btn-secondary" onClick={handleClose}>
            {t("common.cancel")}
          </button>
          <button className="btn-primary" onClick={handleSubmit}>
            {t("common.confirm")}
          </button>
        </div>
      </div>
    </Modal>
  );
}
