import { useTranslation } from "react-i18next";
import { Input, Select } from "./ConnectionFields";

export default function DesktopFields({ form, set, stored, protocol }) {
  const { t } = useTranslation();
  const desktop = form[protocol];
  const canReuseUser = form.sshEnabled;
  const canReusePass = canReuseUser && form.authType === "password";
  const source = key => desktop.reuse[key] ? "ssh" : "own";
  const changeSource = (key, value) => set("reuse", { ...desktop.reuse, [key]: value === "ssh" });
  return <>
    <Input label={t("connection.port")} type="number" min="1" max="65535" value={desktop.port} onChange={v => set("port", Number(v))} />
    <div className="connection-source">
      <Input label={t("connection.username")} value={desktop.reuse.user ? form.username : desktop.username}
        disabled={desktop.reuse.user} onChange={v => set("username", v)} />
      <Select label={t("connection.access.source")} value={source("user")} onChange={v => changeSource("user", v)}>
        <option value="own">{t("connection.access.own")}</option>
        <option value="ssh" disabled={!canReuseUser}>{t("connection.access.fromSSH")}</option>
      </Select>
    </div>
    <div className="connection-source">
      <Input label={t("connection.password")} type="password" autoComplete="new-password" value={desktop.reuse.pass ? "" : desktop.password}
        disabled={desktop.reuse.pass} onChange={v => set("password", v)}
        placeholder={desktop.reuse.pass ? t("connection.access.inherited") : stored && desktop.password == null ? t("connection.access.saved") : ""} />
      <Select label={t("connection.access.source")} value={source("pass")} onChange={v => changeSource("pass", v)}>
        <option value="own">{t("connection.access.own")}</option>
        <option value="ssh" disabled={!canReusePass}>{t("connection.access.fromSSH")}</option>
      </Select>
    </div>
    {!canReusePass && <p className="form-hint">{t("connection.access.noPass", { protocol: protocol.toUpperCase() })}</p>}
    {protocol === "rdp" && <>
      <Select label={t("connection.rdp.resolution")} value={desktop.resolution || "1920x1080"} onChange={v => set("resolution", v)}>
        {["1280x720", "1920x1080", "2560x1440"].map(value => <option key={value} value={value}>{value.replace("x", " x ")}</option>)}
      </Select>
      <p className="form-hint">{t("connection.rdp.resolutionHelp")}</p>
      <Input label={t("connection.rdp.domain")} value={desktop.domain} onChange={v => set("domain", v)} />
      <Select label={t("connection.rdp.security")} value={desktop.disableTLS ? "standard" : "tls"} onChange={v => set("disableTLS", v === "standard")} >
        <option value="tls">{t("connection.rdp.tls")}</option>
        <option value="standard">{t("connection.rdp.standard")}</option>
      </Select>
      {!desktop.disableTLS && <>
        <Select label={t("connection.rdp.verification")} value={desktop.allowUntrusted ? "skip" : "verify"} onChange={v => set("allowUntrusted", v === "skip")}>
          <option value="verify">{t("connection.rdp.verify")}</option>
          <option value="skip">{t("connection.rdp.skip")}</option>
        </Select>
        {desktop.allowUntrusted ? <p className="form-hint">{t("connection.rdp.untrusted")}</p> : <>
          <Input label={t("connection.rdp.fingerprint")} value={desktop.certificateFingerprint} onChange={v => set("certificateFingerprint", v)} autoComplete="off" />
          <p className="form-hint">{t("connection.rdp.trust")}</p>
        </>}
      </>}
      {desktop.disableTLS && <p className="form-hint">{t("connection.rdp.noTLS")}</p>}
    </>}
    <p className="form-hint">{t(`connection.${protocol}.user`)}</p>
    <p className="form-hint">{t(`connection.${protocol}.description`)}</p>
  </>;
}
