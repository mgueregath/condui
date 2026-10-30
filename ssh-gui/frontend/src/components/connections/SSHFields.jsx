import { useTranslation } from "react-i18next";
import { FaLock } from "react-icons/fa";
import { Input, Select } from "./ConnectionFields";
import { hasSSH } from "./connectionForm";

export default function SSHFields({ form, set, connections, isPro, stored }) {
  const { t } = useTranslation();
  return <>
    <div className="form-grid-2">
      <Input label={t("connection.port")} type="number" min="1" max="65535" value={form.port} onChange={v => set("port", Number(v))} />
      <Input label={t("connection.username")} value={form.username} onChange={v => set("username", v)} />
    </div>
    <Select label={t("connection.authentication")} value={form.authType} onChange={v => set("authType", v)}>
      <option value="password">{t("connection.password")}</option>
      <option value="private_key">{t("connection.privateKey")}</option>
    </Select>
    {form.authType === "password" ?
      <Input label={t("connection.password")} type="password" autoComplete="new-password" value={form.password}
        placeholder={stored && form.password == null ? t("connection.access.saved") : ""} onChange={v => set("password", v)} /> : <>
      <Input label={t("connection.privateKeyPath")} value={form.privateKeyPath} onChange={v => set("privateKeyPath", v)} placeholder="~/.ssh/id_ed25519" />
      <Input label={t("connection.passphrase")} type="password" autoComplete="new-password" value={form.passphrase}
        onChange={v => set("passphrase", v)} placeholder={stored && form.passphrase == null ? t("connection.access.saved") : t("connection.passphraseOptional")} />
    </>}
    {isPro ? <Select label={t("connection.jumpHost")} value={form.jumpHostId} onChange={v => set("jumpHostId", v || null)}>
      <option value="">{t("connection.direct")}</option>
      {connections.filter(c => c.id !== form.id && hasSSH(c)).map(c =>
        <option key={c.id} value={c.id}>{c.name} ({c.username}@{c.host}:{c.port})</option>)}
    </Select> : <div className="connection-plan"><FaLock aria-hidden="true" />{t("connection.jumpHostPro")}</div>}
  </>;
}
