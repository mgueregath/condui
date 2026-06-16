import { useState, useEffect } from "react";
import { IsVaultSetup, SetupMasterPassword, UnlockVault } from "../../../bindings/ssh-gui/app";
import conduiLogo from "../../assets/images/condui-transparent.png";

export default function VaultUnlock({ onUnlocked }) {
  const [mode, setMode] = useState("loading"); // "loading" | "setup" | "unlock"
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    IsVaultSetup().then(isSetup => {
      setMode(isSetup ? "unlock" : "setup");
    }).catch(() => {
      setMode("setup"); // Si falla, asumir primer uso
    });
  }, []);

  const handleSetup = async (e) => {
    e.preventDefault();
    setError("");
    if (password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    if (password !== confirm) {
      setError("Passwords do not match");
      return;
    }
    setLoading(true);
    try {
      await SetupMasterPassword(password);
      onUnlocked();
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || "Setup failed");
    } finally {
      setLoading(false);
    }
  };

  const handleUnlock = async (e) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await UnlockVault(password);
      onUnlocked();
    } catch (err) {
      setError("Incorrect password");
    } finally {
      setLoading(false);
    }
  };

  if (mode === "loading") {
    return (
      <div className="vault-screen">
        <div style={{ color: "var(--text-muted)", fontSize: 13 }}>Loading…</div>
      </div>
    );
  }

  return (
    <div className="vault-screen">
      <div className="vault-card">
        <div className="vault-logo">
          <img src={conduiLogo} alt="Condui" style={{ height: 48 }} />
        </div>

        {mode === "setup" ? (
          <>
            <h2 className="vault-title">Create your vault</h2>
            <p className="vault-subtitle">
              Your master password encrypts all SSH credentials stored locally.
              <strong> It cannot be recovered if lost.</strong>
            </p>
            <form onSubmit={handleSetup} className="vault-form">
              <input
                className="modern-input"
                type="password"
                placeholder="Master password (min. 8 chars)"
                value={password}
                onChange={e => setPassword(e.target.value)}
                autoFocus
              />
              <input
                className="modern-input"
                type="password"
                placeholder="Confirm master password"
                value={confirm}
                onChange={e => setConfirm(e.target.value)}
              />
              {error && <div className="vault-error">{error}</div>}
              <button className="btn-primary" type="submit" disabled={loading}>
                {loading ? "Setting up…" : "Create vault"}
              </button>
            </form>
          </>
        ) : (
          <>
            <h2 className="vault-title">Unlock vault</h2>
            <p className="vault-subtitle">Enter your master password to access your connections.</p>
            <form onSubmit={handleUnlock} className="vault-form">
              <input
                className="modern-input"
                type="password"
                placeholder="Master password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                autoFocus
              />
              {error && <div className="vault-error">{error}</div>}
              <button className="btn-primary" type="submit" disabled={loading}>
                {loading ? "Unlocking…" : "Unlock"}
              </button>
            </form>
          </>
        )}
      </div>
    </div>
  );
}
