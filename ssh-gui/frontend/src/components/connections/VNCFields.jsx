import { useTranslation } from "react-i18next";
import { Input, Select } from "./ConnectionFields";

export default function VNCFields({ form, set, stored }) {
  const { t } = useTranslation();
  const vnc = form.vnc;
  const canReuseUser = form.sshEnabled;
  const canReusePass = canReuseUser && form.authType === "password";
  const source = key => vnc.reuse[key] ? "ssh" : "own";
  const changeSource = (key, value) => set("reuse", { ...vnc.reuse, [key]: value === "ssh" });
  return <>
    <Input label={t("connection.port")} type="number" min="1" max="65535" value={vnc.port} onChange={v => set("port", Number(v))} />
    <div className="connection-source">
      <Input label={t("connection.username")} value={vnc.reuse.user ? form.username : vnc.username}
        disabled={vnc.reuse.user} onChange={v => set("username", v)} />
      <Select label={t("connection.access.source")} value={source("user")} onChange={v => changeSource("user", v)}>
        <option value="own">{t("connection.access.own")}</option>
        <option value="ssh" disabled={!canReuseUser}>{t("connection.access.fromSSH")}</option>
      </Select>
    </div>
    <div className="connection-source">
      <Input label={t("connection.password")} type="password" autoComplete="new-password" value={vnc.reuse.pass ? "" : vnc.password}
        disabled={vnc.reuse.pass} onChange={v => set("password", v)}
        placeholder={vnc.reuse.pass ? t("connection.access.inherited") : stored && vnc.password == null ? t("connection.access.saved") : ""} />
      <Select label={t("connection.access.source")} value={source("pass")} onChange={v => changeSource("pass", v)}>
        <option value="own">{t("connection.access.own")}</option>
        <option value="ssh" disabled={!canReusePass}>{t("connection.access.fromSSH")}</option>
      </Select>
    </div>
    {!canReusePass && <p className="form-hint">{t("connection.access.noPass")}</p>}
    <p className="form-hint">{t("connection.vnc.user")}</p>
    <p className="form-hint">{t("connection.vnc.description")}</p>
  </>;
}
