import { useState, useEffect } from "react";
import { IsVaultSetup, SetupMasterPassword, UnlockVault } from "../../../bindings/ssh-gui/app";
import conduiLogo from "../../assets/images/condui-transparent.png";
import { useTranslation } from "react-i18next";

export default function VaultUnlock({ onUnlocked }) {
  const { t } = useTranslation();
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
      setError(t("vault.passwordMinError"));
      return;
    }
    if (password !== confirm) {
      setError(t("vault.passwordMismatch"));
      return;
    }
    setLoading(true);
    try {
      await SetupMasterPassword(password);
      onUnlocked();
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || t("vault.setupFailed"));
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
      setError(t("vault.incorrectPassword"));
    } finally {
      setLoading(false);
    }
  };

  if (mode === "loading") {
    return (
      <div className="vault-screen">
        <div style={{ color: "var(--text-muted)", fontSize: 13 }}>{t("common.loading")}</div>
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
            <h2 className="vault-title">{t("vault.createTitle")}</h2>
            <p className="vault-subtitle">
              {t("vault.createDescription")}
              <strong>{t("vault.cannotRecover")}</strong>
            </p>
            <form onSubmit={handleSetup} className="vault-form">
              <input
                className="modern-input"
                type="password"
                placeholder={t("vault.masterPasswordMin")}
                value={password}
                onChange={e => setPassword(e.target.value)}
                autoFocus
              />
              <input
                className="modern-input"
                type="password"
                placeholder={t("vault.confirmPassword")}
                value={confirm}
                onChange={e => setConfirm(e.target.value)}
              />
              {error && <div className="vault-error">{error}</div>}
              <button className="btn-primary" type="submit" disabled={loading}>
                {loading ? t("vault.settingUp") : t("vault.create")}
              </button>
            </form>
          </>
        ) : (
          <>
            <h2 className="vault-title">{t("vault.unlockTitle")}</h2>
            <p className="vault-subtitle">{t("vault.unlockDescription")}</p>
            <form onSubmit={handleUnlock} className="vault-form">
              <input
                className="modern-input"
                type="password"
                placeholder={t("vault.masterPassword")}
                value={password}
                onChange={e => setPassword(e.target.value)}
                autoFocus
              />
              {error && <div className="vault-error">{error}</div>}
              <button className="btn-primary" type="submit" disabled={loading}>
                {loading ? t("vault.unlocking") : t("vault.unlock")}
              </button>
            </form>
          </>
        )}
      </div>
    </div>
  );
}
