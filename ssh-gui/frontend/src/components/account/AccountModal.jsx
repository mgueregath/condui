import { useState, useEffect } from "react";
import {
  GetAccountStatus,
  AccountLogin,
  AccountRegister,
  AccountRequestPin,
  AccountLoginWithPin,
  AccountLogout,
  SyncNow,
  GetAppVersion,
  CheckForUpdates,
  GetTierLimits,
} from "../../../bindings/ssh-gui/app";
import { useTranslation } from "react-i18next";
import { FaArrowRight } from "react-icons/fa";
import LanguageSwitcher from "../common/LanguageSwitcher";

// Empty string falls through to the Go backend's buildconfig.Values.ServerURL
// (backend/buildconfig, embedded from build.config.yaml at compile time).
// VITE_CONDUI_SERVER_URL is inlined by Vite at build time from that same
// build.config.yaml (see build/Taskfile.yml, buildconfig:frontend-env), so
// there's a single source of truth for the default server URL.
const SERVER_URL = import.meta.env.VITE_CONDUI_SERVER_URL || "";

export default function AccountModal({ onClose }) {
  const { t, i18n } = useTranslation();
  const [status, setStatus] = useState(null);
  const [tab, setTab] = useState("login"); // "login" | "register"
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [syncMsg, setSyncMsg] = useState("");
  const [loginMode, setLoginMode] = useState("password"); // "password" | "pin"
  const [pin, setPin] = useState("");
  const [pinSent, setPinSent] = useState(false);
  const [pinLoading, setPinLoading] = useState(false);
  const [appVersion, setAppVersion] = useState("");
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [updateMsg, setUpdateMsg] = useState("");
  const [planLimits, setPlanLimits] = useState(null);

  const refreshStatus = async () => {
    try {
      const s = await GetAccountStatus();
      setStatus(s);
    } catch (_) {}
  };

  useEffect(() => {
    refreshStatus();
    GetAppVersion().then(setAppVersion).catch(() => {});
    GetTierLimits(SERVER_URL).then(setPlanLimits).catch(() => {});
  }, []);

  const formatPlanLimits = (limits) => {
    if (!limits) return "";
    if (limits.connections === -1 && limits.devices === -1) {
      return t("account.planLimitsUnlimited");
    }
    return t("account.planLimitsCapped", {
      connections: limits.connections,
      devices: limits.devices,
    });
  };

  const handleCheckForUpdates = async () => {
    setUpdateMsg("");
    setCheckingUpdate(true);
    try {
      // Opens the framework's built-in updater window (progress, release
      // notes, Restart & Apply) — this call just kicks that flow off.
      await CheckForUpdates();
    } catch (err) {
      setUpdateMsg(t("account.updateCheckFailed", { error: typeof err === "string" ? err : err?.message }));
    } finally {
      setCheckingUpdate(false);
    }
  };

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

  const handleRequestPin = async (e) => {
    e.preventDefault();
    setError("");
    if (!email) {
      setError(t("account.emailRequired"));
      return;
    }
    setPinLoading(true);
    try {
      await AccountRequestPin(SERVER_URL, email);
      setPinSent(true);
      setSyncMsg(t("account.pinSent"));
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || t("account.pinRequestFailed"));
    } finally {
      setPinLoading(false);
    }
  };

  const handleLoginWithPin = async (e) => {
    e.preventDefault();
    setError("");
    setPinLoading(true);
    try {
      await AccountLoginWithPin(SERVER_URL, email, pin);
      await refreshStatus();
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || t("account.pinLoginFailed"));
    } finally {
      setPinLoading(false);
    }
  };

  const switchLoginMode = (mode) => {
    setLoginMode(mode);
    setError("");
    setSyncMsg("");
    setPinSent(false);
    setPin("");
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
          <div className="account-preference-row">
            <span>{t("settings.language")}</span>
            <LanguageSwitcher />
          </div>

          <div className="account-preference-row">
            <span style={{ color: "var(--text-muted)" }}>
              {appVersion ? t("account.version", { version: appVersion }) : ""}
            </span>
            <button className="btn-secondary btn-sm" onClick={handleCheckForUpdates} disabled={checkingUpdate}>
              {checkingUpdate ? t("account.checkingForUpdates") : t("account.checkForUpdates")}
            </button>
          </div>
          {updateMsg && <div className="vault-error">{updateMsg}</div>}

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
              {status.limits?.connections != null && status.limits?.devices != null
                ? t("account.freePlanDescriptionWithLimits", {
                    connections: status.limits.connections,
                    devices: status.limits.devices,
                  })
                : t("account.freePlanDescription")}{" "}
              <a href="https://condui.app/upgrade" target="_blank" rel="noreferrer">
                {t("account.upgrade")} <FaArrowRight />
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
        <div className="account-preference-row">
          <span>{t("settings.language")}</span>
          <LanguageSwitcher />
        </div>

        <div className="account-preference-row">
          <span style={{ color: "var(--text-muted)" }}>
            {appVersion ? t("account.version", { version: appVersion }) : ""}
          </span>
          <button className="btn-secondary btn-sm" onClick={handleCheckForUpdates} disabled={checkingUpdate}>
            {checkingUpdate ? t("account.checkingForUpdates") : t("account.checkForUpdates")}
          </button>
        </div>
        {updateMsg && <div className="vault-error">{updateMsg}</div>}

        <div className="account-tabs">
          <button
            className={`account-tab${tab === "login" ? " active" : ""}`}
            onClick={() => { setTab("login"); switchLoginMode("password"); }}
          >
            {t("account.login")}
          </button>
          <button
            className={`account-tab${tab === "register" ? " active" : ""}`}
            onClick={() => { setTab("register"); switchLoginMode("password"); }}
          >
            {t("account.register")}
          </button>
        </div>

        {tab === "register" && planLimits && (
          <div className="plan-limits-box">
            {["free", "pro"].filter(tier => planLimits[tier]).map(tier => (
              <div className="plan-limits-row" key={tier}>
                <span className={`tier-badge tier-${tier}`}>
                  {tier === "pro" ? t("common.pro") : t("common.free")}
                </span>
                <span>{formatPlanLimits(planLimits[tier])}</span>
              </div>
            ))}
          </div>
        )}

        {tab === "login" && loginMode === "pin" ? (
          <form onSubmit={pinSent ? handleLoginWithPin : handleRequestPin} className="vault-form">
            <input
              className="modern-input"
              type="email"
              placeholder={t("account.email")}
              value={email}
              onChange={e => setEmail(e.target.value)}
              disabled={pinSent}
              autoFocus
            />
            {pinSent && (
              <input
                className="modern-input"
                type="text"
                inputMode="numeric"
                placeholder={t("account.enterPin")}
                value={pin}
                onChange={e => setPin(e.target.value)}
                autoFocus
              />
            )}

            {error && <div className="vault-error">{error}</div>}
            {syncMsg && <div className="vault-success">{syncMsg}</div>}

            <button className="btn-primary" type="submit" disabled={pinLoading}>
              {pinSent
                ? (pinLoading ? t("account.signingIn") : t("app.signIn"))
                : (pinLoading ? t("account.sendingPin") : t("account.sendPin"))}
            </button>

            <button
              type="button"
              className="account-link-btn"
              onClick={() => switchLoginMode("password")}
            >
              {t("account.signInWithPassword")}
            </button>
          </form>
        ) : (
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

            {tab === "login" && (
              <button
                type="button"
                className="account-link-btn"
                onClick={() => switchLoginMode("pin")}
              >
                {t("account.signInWithPin")}
              </button>
            )}
          </form>
        )}

      </div>
      <div className="modal-footer">
        <button className="btn-secondary" onClick={onClose}>{t("common.cancel")}</button>
      </div>
    </div>
  );
}
