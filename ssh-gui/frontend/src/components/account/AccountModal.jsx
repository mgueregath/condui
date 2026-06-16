import { useState, useEffect } from "react";
import {
  GetAccountStatus,
  AccountLogin,
  AccountRegister,
  AccountLogout,
  SyncNow,
} from "../../../bindings/ssh-gui/app";

const SERVER_URL = import.meta.env.VITE_CONDUI_SERVER_URL || "https://sync.condui.app";

export default function AccountModal({ onClose }) {
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
      setError(typeof err === "string" ? err : err?.message || "Login failed");
    } finally {
      setLoading(false);
    }
  };

  const handleRegister = async (e) => {
    e.preventDefault();
    setError("");
    if (password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    setLoading(true);
    try {
      await AccountRegister(SERVER_URL, email, password);
      setError("");
      setTab("login");
      setSyncMsg("Account created. Please log in.");
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || "Registration failed");
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
      setSyncMsg("Synced successfully");
      await refreshStatus();
    } catch (err) {
      setSyncMsg("Sync failed: " + (typeof err === "string" ? err : err?.message));
    } finally {
      setLoading(false);
    }
  };

  if (!status) {
    return (
      <div className="modal-body" style={{ textAlign: "center", padding: 40 }}>
        <span style={{ color: "var(--text-muted)" }}>Loading…</span>
      </div>
    );
  }

  if (status.loggedIn) {
    return (
      <div>
        <div className="modal-header">
          <h2>Account</h2>
          <p>Condui Sync</p>
        </div>
        <div className="modal-body">
          <div className="account-info-card">
            <div className="account-avatar">{status.email?.[0]?.toUpperCase() || "?"}</div>
            <div className="account-info-details">
              <div className="account-email">{status.email}</div>
              <span className={`tier-badge tier-${status.tier}`}>
                {status.tier === "pro" ? "Pro" : "Free"}
              </span>
            </div>
          </div>

          <div className="account-sync-row">
            <div style={{ fontSize: 12, color: "var(--text-muted)" }}>
              {status.lastSync
                ? `Last sync: ${new Date(status.lastSync).toLocaleString()}`
                : "Never synced"}
            </div>
            <button className="btn-secondary btn-sm" onClick={handleSync} disabled={loading}>
              {loading ? "Syncing…" : "Sync Now"}
            </button>
          </div>

          {syncMsg && (
            <div className={`vault-${syncMsg.includes("fail") ? "error" : "success"}`}>
              {syncMsg}
            </div>
          )}

          {status.tier === "free" && (
            <div className="upgrade-banner">
              <strong>Free plan</strong> — up to 10 connections, 2 devices.{" "}
              <a href="https://condui.app/upgrade" target="_blank" rel="noreferrer">
                Upgrade to Pro →
              </a>
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="btn-secondary" onClick={handleLogout} disabled={loading}>
            Log out
          </button>
          <button className="btn-primary" onClick={onClose}>Done</button>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="modal-header">
        <h2>Sign in to Condui Sync</h2>
        <p>Sync your connections across devices</p>
      </div>
      <div className="modal-body">
        <div className="account-tabs">
          <button
            className={`account-tab${tab === "login" ? " active" : ""}`}
            onClick={() => { setTab("login"); setError(""); }}
          >
            Log in
          </button>
          <button
            className={`account-tab${tab === "register" ? " active" : ""}`}
            onClick={() => { setTab("register"); setError(""); }}
          >
            Register
          </button>
        </div>

        <form onSubmit={tab === "login" ? handleLogin : handleRegister} className="vault-form">
          <input
            className="modern-input"
            type="email"
            placeholder="Email"
            value={email}
            onChange={e => setEmail(e.target.value)}
            autoFocus
          />
          <input
            className="modern-input"
            type="password"
            placeholder="Password"
            value={password}
            onChange={e => setPassword(e.target.value)}
          />

          {error && <div className="vault-error">{error}</div>}
          {syncMsg && <div className="vault-success">{syncMsg}</div>}

          <button className="btn-primary" type="submit" disabled={loading}>
            {loading
              ? (tab === "login" ? "Signing in…" : "Creating account…")
              : (tab === "login" ? "Sign in" : "Create account")}
          </button>
        </form>

      </div>
      <div className="modal-footer">
        <button className="btn-secondary" onClick={onClose}>Cancel</button>
      </div>
    </div>
  );
}
