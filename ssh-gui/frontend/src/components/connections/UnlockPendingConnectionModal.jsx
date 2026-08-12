import { useState } from "react";
import { UnlockPendingConnection } from "../../../bindings/ssh-gui/app";
import { useTranslation } from "react-i18next";

// `connections` is every connection currently pending cross-vault decryption,
// not just the one whose lock badge was clicked: connections synced from the
// same origin device share the same vault password, so trying that password
// against all of them at once lets the user unlock a whole batch with a
// single prompt instead of repeating it once per connection.
export default function UnlockPendingConnectionModal({ connections, onClose, onUnlocked }) {
  const { t } = useTranslation();
  const [pending, setPending] = useState(connections);
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleUnlock = async (e) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    const stillPending = [];
    let unlockedCount = 0;
    for (const conn of pending) {
      try {
        await UnlockPendingConnection(conn.id, password);
        unlockedCount++;
      } catch {
        stillPending.push(conn);
      }
    }

    setLoading(false);
    setPassword("");

    if (unlockedCount > 0) {
      onUnlocked?.();
    }

    if (stillPending.length === 0) {
      onClose?.();
      return;
    }

    setPending(stillPending);
    setError(
      unlockedCount > 0
        ? t("connection.unlockPendingPartial", { unlocked: unlockedCount, remaining: stillPending.length })
        : t("connection.unlockPendingFailed")
    );
  };

  const count = pending.length;

  return (
    <form onSubmit={handleUnlock}>
      <div className="modal-header">
        <h2>{t("connection.unlockPendingTitle")}</h2>
        <p>
          {count === 1
            ? t("connection.unlockPendingDescription", { name: pending[0]?.name })
            : t("connection.unlockPendingDescriptionMulti", { count })}
        </p>
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
