import { useState } from "react";
import { UnlockPendingConnection } from "../../../bindings/ssh-gui/app";
import { useTranslation } from "react-i18next";

export default function UnlockPendingConnectionModal({ connection, onClose, onUnlocked }) {
  const { t } = useTranslation();
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleUnlock = async (e) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await UnlockPendingConnection(connection.id, password);
      onUnlocked?.();
      onClose?.();
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || t("connection.unlockPendingFailed"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleUnlock}>
      <div className="modal-header">
        <h2>{t("connection.unlockPendingTitle")}</h2>
        <p>{t("connection.unlockPendingDescription", { name: connection?.name })}</p>
      </div>
      <div className="modal-body">
        <input
          className="modern-input"
          type="password"
          placeholder={t("connection.originVaultPassword")}
          value={password}
          onChange={e => setPassword(e.target.value)}
          autoFocus
        />
        {error && <div className="vault-error">{error}</div>}
      </div>
      <div className="modal-footer">
        <button type="button" className="btn-secondary" onClick={onClose}>
          {t("common.cancel")}
        </button>
        <button className="btn-primary" type="submit" disabled={loading || !password}>
          {loading ? t("connection.unlockingPending") : t("connection.unlockPendingAction")}
        </button>
      </div>
    </form>
  );
}
