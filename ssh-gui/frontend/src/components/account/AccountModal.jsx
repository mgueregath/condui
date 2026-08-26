import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { FaArrowRight } from "react-icons/fa";
import { AccountLogin, AccountLoginWithPin, AccountLogout, AccountRegister, AccountRequestPin, CheckForUpdates, GetAccountStatus, GetAppVersion, GetTierLimits, SyncNow } from "../../../bindings/ssh-gui/app";
import LanguageSwitcher from "../common/LanguageSwitcher";
import SettingsSidebar from "./SettingsSidebar";

const SERVER_URL = import.meta.env.VITE_CONDUI_SERVER_URL || "";

export default function AccountModal({ onClose }) {
  const { t, i18n } = useTranslation();
  const [status, setStatus] = useState(null);
  const [section, setSection] = useState("account");
  const [tab, setTab] = useState("login");
  const [loginMode, setLoginMode] = useState("password");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [pin, setPin] = useState("");
  const [pinSent, setPinSent] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(false);
  const [pinLoading, setPinLoading] = useState(false);
  const [appVersion, setAppVersion] = useState("");
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [updateMsg, setUpdateMsg] = useState("");
  const [planLimits, setPlanLimits] = useState(null);

  const refreshStatus = async () => {
    try { setStatus(await GetAccountStatus()); } catch (_) {}
  };

  useEffect(() => {
    refreshStatus();
    GetAppVersion().then(setAppVersion).catch(() => {});
    GetTierLimits(SERVER_URL).then(setPlanLimits).catch(() => {});
  }, []);

  const formatLimits = (limits) => {
    if (!limits) return "";
    if (limits.connections === -1 && limits.devices === -1) return t("account.planLimitsUnlimited");
    return t("account.planLimitsCapped", limits);
  };

  const resetLoginMode = (mode) => {
    setLoginMode(mode); setError(""); setMessage(""); setPinSent(false); setPin("");
  };

  const runAccountAction = async (action, fallback) => {
    setError(""); setLoading(true);
    try { await action(); await refreshStatus(); }
    catch (err) { setError(typeof err === "string" ? err : err?.message || t(fallback)); }
    finally { setLoading(false); }
  };

  const handleLogin = (event) => { event.preventDefault(); runAccountAction(() => AccountLogin(SERVER_URL, email, password), "account.loginFailed"); };
  const handleRegister = async (event) => {
    event.preventDefault(); setError("");
    if (password.length < 8) return setError(t("vault.passwordMinError"));
    setLoading(true);
    try { await AccountRegister(SERVER_URL, email, password); setTab("login"); setMessage(t("account.accountCreated")); }
    catch (err) { setError(typeof err === "string" ? err : err?.message || t("account.registrationFailed")); }
    finally { setLoading(false); }
  };
  const handleRequestPin = async (event) => {
    event.preventDefault(); setError("");
    if (!email) return setError(t("account.emailRequired"));
    setPinLoading(true);
    try { await AccountRequestPin(SERVER_URL, email); setPinSent(true); setMessage(t("account.pinSent")); }
    catch (err) { setError(typeof err === "string" ? err : err?.message || t("account.pinRequestFailed")); }
    finally { setPinLoading(false); }
  };
  const handlePinLogin = async (event) => {
    event.preventDefault(); setError(""); setPinLoading(true);
    try { await AccountLoginWithPin(SERVER_URL, email, pin); await refreshStatus(); }
    catch (err) { setError(typeof err === "string" ? err : err?.message || t("account.pinLoginFailed")); }
    finally { setPinLoading(false); }
  };
  const handleSync = async () => {
    setMessage(""); setLoading(true);
    try { await SyncNow(); setMessage(t("account.synced")); await refreshStatus(); }
    catch (err) { setMessage(t("account.syncFailed", { error: typeof err === "string" ? err : err?.message })); }
    finally { setLoading(false); }
  };
  const handleUpdates = async () => {
    setUpdateMsg(""); setCheckingUpdate(true);
    try { await CheckForUpdates(); }
    catch (err) { setUpdateMsg(t("account.updateCheckFailed", { error: typeof err === "string" ? err : err?.message })); }
    finally { setCheckingUpdate(false); }
  };

  if (!status) return <div className="settings-loading">{t("common.loading")}</div>;

  const authForm = (
    <>
      <div className="account-tabs">
        <button className={`account-tab${tab === "login" ? " active" : ""}`} onClick={() => { setTab("login"); resetLoginMode("password"); }}>{t("account.login")}</button>
        <button className={`account-tab${tab === "register" ? " active" : ""}`} onClick={() => { setTab("register"); resetLoginMode("password"); }}>{t("account.register")}</button>
      </div>
      {tab === "register" && planLimits && <div className="plan-limits-box">{["free", "pro"].filter((tier) => planLimits[tier]).map((tier) => <div className="plan-limits-row" key={tier}><span className={`tier-badge tier-${tier}`}>{tier === "pro" ? t("common.pro") : t("common.free")}</span><span>{formatLimits(planLimits[tier])}</span></div>)}</div>}
      {tab === "login" && loginMode === "pin" ? (
        <form onSubmit={pinSent ? handlePinLogin : handleRequestPin} className="vault-form settings-auth-form">
          <input className="modern-input" type="email" placeholder={t("account.email")} value={email} onChange={(e) => setEmail(e.target.value)} disabled={pinSent} autoFocus />
          {pinSent && <input className="modern-input" inputMode="numeric" placeholder={t("account.enterPin")} value={pin} onChange={(e) => setPin(e.target.value)} autoFocus />}
          {error && <div className="vault-error">{error}</div>}{message && <div className="vault-success">{message}</div>}
          <button className="btn-primary" type="submit" disabled={pinLoading}>{pinSent ? (pinLoading ? t("account.signingIn") : t("app.signIn")) : (pinLoading ? t("account.sendingPin") : t("account.sendPin"))}</button>
          <button type="button" className="account-link-btn" onClick={() => resetLoginMode("password")}>{t("account.signInWithPassword")}</button>
        </form>
      ) : (
        <form onSubmit={tab === "login" ? handleLogin : handleRegister} className="vault-form settings-auth-form">
          <input className="modern-input" type="email" placeholder={t("account.email")} value={email} onChange={(e) => setEmail(e.target.value)} autoFocus />
          <input className="modern-input" type="password" placeholder={t("account.password")} value={password} onChange={(e) => setPassword(e.target.value)} />
          {error && <div className="vault-error">{error}</div>}{message && <div className="vault-success">{message}</div>}
          <button className="btn-primary" type="submit" disabled={loading}>{loading ? (tab === "login" ? t("account.signingIn") : t("account.creatingAccount")) : (tab === "login" ? t("app.signIn") : t("account.createAccount"))}</button>
          {tab === "login" && <button type="button" className="account-link-btn" onClick={() => resetLoginMode("pin")}>{t("account.signInWithPin")}</button>}
        </form>
      )}
    </>
  );

  const pages = {
    account: <><Header title={status.loggedIn ? t("account.account") : t("account.signInTitle")} text={status.loggedIn ? t("settings.accountDescription") : t("account.signInDescription")} />{status.loggedIn ? <><div className="account-info-card settings-account-card"><div className="account-avatar">{status.email?.[0]?.toUpperCase() || "?"}</div><div className="account-info-details"><div className="account-email">{status.email}</div><span className={`tier-badge tier-${status.tier}`}>{status.tier === "pro" ? t("common.pro") : t("common.free")}</span></div></div><div className="settings-group"><Row title={t("settings.plan")} text={status.limits ? formatLimits(status.limits) : t("account.freePlanDescription")}>{status.tier === "free" && <a className="settings-inline-link" href="https://condui.app/upgrade" target="_blank" rel="noreferrer">{t("account.upgrade")} <FaArrowRight /></a>}</Row></div></> : authForm}</>,
    sync: <><Header title={t("settings.sections.sync")} text={t("settings.syncDescription")} /><div className="settings-group"><Row title={t("settings.syncStatus")} text={status.loggedIn ? (status.lastSync ? t("account.lastSync", { date: new Date(status.lastSync).toLocaleString(i18n.language) }) : t("account.neverSynced")) : t("settings.signInRequired")}><button className="btn-secondary btn-sm" onClick={handleSync} disabled={loading || !status.loggedIn}>{loading ? t("account.syncing") : t("account.syncNow")}</button></Row></div>{message && <div className="vault-success">{message}</div>}</>,
    appearance: <><Header title={t("settings.sections.appearance")} text={t("settings.appearanceDescription")} /><div className="settings-group"><Row title={t("settings.language")} text={t("settings.languageDescription")}><LanguageSwitcher /></Row></div></>,
    security: <><Header title={t("settings.sections.security")} text={t("settings.securityDescription")} /><div className="settings-empty-state"><strong>{t("settings.securityManaged")}</strong><span>{t("settings.securityManagedDescription")}</span></div></>,
    updates: <><Header title={t("settings.sections.updates")} text={t("settings.updatesDescription")} /><div className="settings-group"><Row title={t("settings.appVersion")} text={appVersion ? t("account.version", { version: appVersion }) : t("common.loading")}><button className="btn-secondary btn-sm" onClick={handleUpdates} disabled={checkingUpdate}>{checkingUpdate ? t("account.checkingForUpdates") : t("account.checkForUpdates")}</button></Row></div>{updateMsg && <div className="vault-error">{updateMsg}</div>}</>,
  };

  return <div className="settings-modal"><SettingsSidebar activeSection={section} onSelect={setSection} t={t} /><section className="settings-content"><div className="settings-content-body">{pages[section]}</div><footer className="settings-footer">{status.loggedIn && section === "account" && <button className="btn-secondary settings-logout" onClick={() => runAccountAction(AccountLogout, "account.logoutFailed")} disabled={loading}>{t("account.logout")}</button>}<button className="btn-primary" onClick={onClose}>{t("common.done")}</button></footer></section></div>;
}

function Header({ title, text }) { return <header className="settings-section-header"><h1>{title}</h1><p>{text}</p></header>; }
function Row({ title, text, children }) { return <div className="settings-row"><div><strong>{title}</strong><span>{text}</span></div>{children}</div>; }
