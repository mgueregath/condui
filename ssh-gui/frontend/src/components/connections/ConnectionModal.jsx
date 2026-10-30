import "./ConnectionModal.css";
import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { TestConnectionParams } from "../../../bindings/ssh-gui/app";
import { testRDP } from "../../services/rdp";
import { testVNC } from "../../services/vnc";
import useResponseAction from "../../hooks/useResponseAction";
import ResponseMessage from "../common/ResponseMessage";
import ProtocolIcon from "./ProtocolIcon";
import { Input, Select } from "./ConnectionFields";
import SSHFields from "./SSHFields";
import DesktopFields from "./DesktopFields";
import ConnectionTest from "./ConnectionTest";
import { connectionForm, protocols, defaultProtocol, hasAccess } from "./connectionForm";

export default function ConnectionModal({ initialValue, folders, connections = [], accountStatus, onSave, onCancel, onBusyChange }) {
  const { t } = useTranslation();
  const id = useId();
  const [form, setForm] = useState(() => connectionForm(initialValue));
  const [context, setContext] = useState(() => initialValue ? defaultProtocol(initialValue) : "ssh");
  const [success, setSuccess] = useState({});
  const saveAction = useResponseAction();
  const sshAction = useResponseAction();
  const vncAction = useResponseAction();
  const rdpAction = useResponseAction();
  const actions = { ssh: sshAction, vnc: vncAction, rdp: rdpAction };
  const busy = saveAction.busy || Object.values(actions).some(action => action.busy);
  useEffect(() => { onBusyChange?.(busy); }, [busy, onBusyChange]);
  useEffect(() => () => onBusyChange?.(false), [onBusyChange]);
  const invalidate = (protocol) => {
    setSuccess(previous => ({ ...previous, [protocol]: false }));
    actions[protocol].setResponse(null);
  };
  const set = (key, value) => {
    setForm(previous => {
      const next = { ...previous, [key]: value };
      for (const protocol of ["vnc", "rdp"]) {
        if (key === "sshEnabled" && !value) next[protocol] = { ...previous[protocol], reuse: { user: false, pass: false } };
        if (key === "authType" && value === "private_key") next[protocol] = { ...previous[protocol], reuse: { ...previous[protocol].reuse, pass: false } };
      }
      return next;
    });
    saveAction.setResponse(null);
    if (["host", "port", "username", "authType", "password", "privateKeyPath", "passphrase", "jumpHostId", "sshEnabled"].includes(key)) invalidate("ssh");
    for (const protocol of ["vnc", "rdp"]) if (key === "host" || key === "sshEnabled" || key === "authType" || (key === "username" && form[protocol].reuse.user) || (key === "password" && form[protocol].reuse.pass)) invalidate(protocol);
  };
  const setDesktop = (protocol, key, value) => {
    setForm(previous => ({ ...previous, [protocol]: { ...previous[protocol], [key]: value } }));
    invalidate(protocol); saveAction.setResponse(null);
  };
  const validate = (protocol) => {
    const port = protocol === "ssh" ? form.port : form[protocol].port;
    if (!form.host.trim() || !Number.isInteger(Number(port)) || Number(port) < 1 || Number(port) > 65535) throw new Error(t("connection.access.invalid"));
    if (protocol === "rdp" && !(form.rdp.reuse.user ? form.username : form.rdp.username).trim()) throw new Error(t("connection.rdp.required"));
    if (protocol === "rdp" && !form.rdp.disableTLS && !form.rdp.allowUntrusted && form.rdp.certificateFingerprint && !/^[a-f0-9]{64}$/i.test(form.rdp.certificateFingerprint.replaceAll(":", "").trim())) throw new Error(t("connection.rdp.invalidFingerprint"));
    if (protocol === "ssh" && (!form.username.trim() || (form.authType === "private_key" && !form.privateKeyPath?.trim()))) throw new Error(t("connection.access.sshRequired"));
  };
  const test = protocol => {
    const action = actions[protocol];
    return action.run(async () => {
      validate(protocol);
      setSuccess(previous => ({ ...previous, [protocol]: false }));
      if (protocol === "rdp") await testRDP(form);
      else if (protocol === "vnc") await testVNC(form);
      else await TestConnectionParams(form.host, Number(form.port), form.username, form.authType,
        form.authType === "password" ? form.password ?? (form.id ? `__stored_connection_password__:${form.id}` : "") : "",
        form.privateKeyPath || "", form.authType === "private_key" ? form.passphrase ?? (form.id ? `__stored_connection_password__:${form.id}` : "") : "", form.jumpHostId || "");
      setSuccess(previous => ({ ...previous, [protocol]: true }));
    }, t("connection.testing"), t("connection.access.failed", { protocol: protocol.toUpperCase() }), [form.password, form.passphrase, form.vnc.password, form.rdp.password]);
  };
  const save = () => saveAction.run(async () => {
    if (!form.name.trim() || !protocols.some(protocol => hasAccess(form, protocol))) throw new Error(t("connection.access.required"));
    for (const protocol of protocols) if (hasAccess(form, protocol)) validate(protocol);
    await onSave({ ...form, protocol: defaultProtocol(form) });
  }, t("common.response.saving"), t("connection.response.saveFailed"), [form.password, form.passphrase, form.vnc.password, form.rdp.password]);
  const chooseTab = event => {
    if (!["ArrowUp", "ArrowDown", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    const direction = event.key === "ArrowUp" ? -1 : 1;
    const next = event.key === "Home" ? protocols[0] : event.key === "End" ? protocols.at(-1) : protocols[(protocols.indexOf(context) + direction + protocols.length) % protocols.length];
    setContext(next); document.getElementById(`${id}-${next}-tab`)?.focus();
  };
  return <div className="connection-editor">
    <div className="modal-header"><h2>{t(initialValue ? "connection.editTitle" : "connection.newTitle")}</h2></div>
    <div className="connection-shared">
      <fieldset className="response-fields" disabled={busy}>
        <div className="connection-identity">
          <Input label={t("connection.name")} value={form.name} onChange={v => set("name", v)} placeholder={t("connection.namePlaceholder")} />
          <Select label={t("connection.folder")} value={form.folderId} onChange={v => set("folderId", v || null)}>
            <option value="">{t("connection.noFolder")}</option>{folders.map(f => <option key={f.id} value={f.id}>{f.name}</option>)}
          </Select>
          <Input label={t("connection.color")} type="color" value={form.color || "#6b6490"} onChange={v => set("color", v)} />
        </div>
        <Input label={t("connection.access.host")} value={form.host} onChange={v => set("host", v)} placeholder="192.0.2.14" />
      </fieldset>
    </div>
    <div className="connection-layout">
      <div className="connection-tabs" role="tablist" aria-label={t("connection.access.label")} aria-orientation="vertical">
        {protocols.map(protocol => <button key={protocol} type="button" role="tab" id={`${id}-${protocol}-tab`}
          aria-controls={`${id}-${protocol}-panel`} aria-selected={context === protocol} tabIndex={context === protocol ? 0 : -1}
          onClick={() => setContext(protocol)} onKeyDown={chooseTab}>
          <ProtocolIcon protocol={protocol} /><span>{t(`connection.type.${protocol}`)}</span>
          <small>{t(hasAccess(form, protocol) ? "connection.access.enabled" : "connection.access.disabled")}</small>
        </button>)}
      </div>
      <div className="connection-panels">
        {protocols.map(protocol => {
          const enabled = hasAccess(form, protocol);
          const action = actions[protocol];
          return <section key={protocol} role="tabpanel" id={`${id}-${protocol}-panel`} aria-labelledby={`${id}-${protocol}-tab`} hidden={context !== protocol}>
            <div className="connection-enable"><h3>{t(`connection.access.${protocol}`)}</h3>
              <label><input type="checkbox" checked={enabled} disabled={busy}
                onChange={e => protocol === "ssh" ? set("sshEnabled", e.target.checked) : setDesktop(protocol, "enabled", e.target.checked)} />
                {t("connection.access.enable", { protocol: protocol.toUpperCase() })}</label>
            </div>
            <fieldset className="response-fields" disabled={!enabled || busy}>
              {protocol === "ssh" ? <SSHFields form={form} set={set} connections={connections} isPro={accountStatus?.tier === "pro"} stored={!!initialValue} /> :
                <DesktopFields form={form} protocol={protocol} set={(key, value) => setDesktop(protocol, key, value)} stored={!!initialValue?.[protocol]} />}
            </fieldset>
            <ConnectionTest protocol={protocol} action={action} success={success[protocol]} disabled={!enabled || saveAction.busy} onTest={() => test(protocol)} />
          </section>;
        })}
      </div>
    </div>
    <ResponseMessage {...saveAction.response} />
    <div className="modal-footer">
      <button type="button" className="btn-secondary" disabled={busy} onClick={onCancel}>{t("common.cancel")}</button>
      <button type="button" className="btn-primary" disabled={busy} onClick={save}>{t("connection.save")}</button>
    </div>
  </div>;
}
