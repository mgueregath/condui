import { useState, useEffect } from "react";
import {
  GetAccountStatus,
  AccountLogin,
  AccountRegister,
  AccountLogout,
  SyncNow,
} from "../../../bindings/ssh-gui/app";
import { useTranslation } from "react-i18next";

const SERVER_URL = import.meta.env.VITE_CONDUI_SERVER_URL || "https://sync.condui.app";

export default function AccountModal({ onClose }) {
  const { t, i18n } = useTranslation();
  const [status, setStatus] = useState(null);
  const [tab, setTab] = useState("login"); // "login" | "register"
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [syncMsg, setSyncMsg] = useState("");

  const refreshStatus = async () => {
    try {
      const s = await GetAccountStatus();
      setStatus(s);
    } catch (_) {}
  };

  useEffect(() => { refreshStatus(); }, []);

  const handleLogin = async (e) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await AccountLogin(SERVER_URL, email, password);
      await refreshStatus();
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || t("account.loginFailed"));
    } finally {
      setLoading(false);
    }
  };

  const handleRegister = async (e) => {
    e.preventDefault();
    setError("");
    if (password.length < 8) {
      setError(t("vault.passwordMinError"));
      return;
    }
    setLoading(true);
    try {
      await AccountRegister(SERVER_URL, email, password);
      setError("");
      setTab("login");
      setSyncMsg(t("account.accountCreated"));
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || t("account.registrationFailed"));
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = async () => {
    setLoading(true);
    try {
      await AccountLogout();
      await refreshStatus();
    } catch (_) {} finally {
      setLoading(false);
    }
  };

  const handleSync = async () => {
    setSyncMsg("");
    setLoading(true);
    try {
      await SyncNow();
      setSyncMsg(t("account.synced"));
      await refreshStatus();
    } catch (err) {
      setSyncMsg(t("account.syncFailed", { error: typeof err === "string" ? err : err?.message }));
    } finally {
      setLoading(false);
    }
  };

  if (!status) {
    return (
      <div className="modal-body" style={{ textAlign: "center", padding: 40 }}>
        <span style={{ color: "var(--text-muted)" }}>{t("common.loading")}</span>
      </div>
    );
  }

  if (status.loggedIn) {
    return (
      <div>
        <div className="modal-header">
          <h2>{t("account.account")}</h2>
          <p>Condui Sync</p>
        </div>
        <div className="modal-body">
          <div className="account-info-card">
            <div className="account-avatar">{status.email?.[0]?.toUpperCase() || "?"}</div>
            <div className="account-info-details">
              <div className="account-email">{status.email}</div>
              <span className={`tier-badge tier-${status.tier}`}>
                {status.tier === "pro" ? t("common.pro") : t("common.free")}
              </span>
            </div>
          </div>

          <div className="account-sync-row">
            <div style={{ fontSize: 12, color: "var(--text-muted)" }}>
              {status.lastSync
                ? t("account.lastSync", { date: new Date(status.lastSync).toLocaleString(i18n.language) })
                : t("account.neverSynced")}
            </div>
            <button className="btn-secondary btn-sm" onClick={handleSync} disabled={loading}>
              {loading ? t("account.syncing") : t("account.syncNow")}
            </button>
          </div>

          {syncMsg && (
            <div className={`vault-${syncMsg.includes("fail") ? "error" : "success"}`}>
              {syncMsg}
            </div>
          )}

          {status.tier === "free" && (
            <div className="upgrade-banner">
              {t("account.freePlanDescription")}{" "}
              <a href="https://condui.app/upgrade" target="_blank" rel="noreferrer">
                {t("account.upgrade")}
              </a>
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="btn-secondary" onClick={handleLogout} disabled={loading}>
            {t("account.logout")}
          </button>
          <button className="btn-primary" onClick={onClose}>{t("common.done")}</button>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="modal-header">
        <h2>{t("account.signInTitle")}</h2>
        <p>{t("account.signInDescription")}</p>
      </div>
      <div className="modal-body">
        <div className="account-tabs">
          <button
            className={`account-tab${tab === "login" ? " active" : ""}`}
            onClick={() => { setTab("login"); setError(""); }}
          >
            {t("account.login")}
          </button>
          <button
            className={`account-tab${tab === "register" ? " active" : ""}`}
            onClick={() => { setTab("register"); setError(""); }}
          >
            {t("account.register")}
          </button>
        </div>

        <form onSubmit={tab === "login" ? handleLogin : handleRegister} className="vault-form">
          <input
            className="modern-input"
            type="email"
            placeholder={t("account.email")}
            value={email}
            onChange={e => setEmail(e.target.value)}
            autoFocus
          />
          <input
            className="modern-input"
            type="password"
            placeholder={t("account.password")}
            value={password}
            onChange={e => setPassword(e.target.value)}
          />

          {error && <div className="vault-error">{error}</div>}
          {syncMsg && <div className="vault-success">{syncMsg}</div>}

          <button className="btn-primary" type="submit" disabled={loading}>
            {loading
              ? (tab === "login" ? t("account.signingIn") : t("account.creatingAccount"))
              : (tab === "login" ? t("app.signIn") : t("account.createAccount"))}
          </button>
        </form>

      </div>
      <div className="modal-footer">
        <button className="btn-secondary" onClick={onClose}>{t("common.cancel")}</button>
      </div>
    </div>
  );
}
