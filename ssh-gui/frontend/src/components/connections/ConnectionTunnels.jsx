import { useState } from "react";
import { useTranslation } from "react-i18next";
import { AddTunnel, EditTunnel, DeleteTunnel } from "../../../bindings/ssh-gui/app";
import useTunnels from "../../hooks/useTunnels";
import useResponseAction from "../../hooks/useResponseAction";
import ResponseMessage from "../common/ResponseMessage";
import { Input } from "./ConnectionFields";

const empty = () => ({ localPort: "", remoteHost: "127.0.0.1", remotePort: "" });
export default function ConnectionTunnels({ connectionId, value, onChange, disabled, onBusyChange }) {
  const { t } = useTranslation();
  const { tunnels: saved, reload, error } = useTunnels(connectionId);
  const tunnels = connectionId ? saved : value || [];
  const [editing, setEditing] = useState(null);
  const [draft, setDraft] = useState(empty);
  const [deleting, setDeleting] = useState(null);
  const action = useResponseAction(onBusyChange);
  const busy = disabled || action.busy;
  const run = callback => action.run(callback, t("common.response.saving"), t("panel.tunnelError", { error: "" }));
  const submit = () => run(async () => {
    const tunnel = { ...draft, localPort: Number(draft.localPort), remotePort: Number(draft.remotePort), remoteHost: draft.remoteHost.trim() };
    if (!tunnel.remoteHost || ![tunnel.localPort, tunnel.remotePort].every(port => Number.isInteger(port) && port >= 1 && port <= 65535)) throw new Error(t("connection.tunnels.invalid"));
    if (tunnels.some(item => item.id !== editing && item.localPort === tunnel.localPort)) throw new Error(t("connection.tunnels.duplicate"));
    if (connectionId) {
      if (editing) await EditTunnel(editing, tunnel.localPort, tunnel.remoteHost, tunnel.remotePort);
      else await AddTunnel(connectionId, tunnel.localPort, tunnel.remoteHost, tunnel.remotePort);
      await reload();
    } else {
      onChange(editing ? tunnels.map(item => item.id === editing ? { ...item, ...tunnel } : item) : [...tunnels, { ...tunnel, id: crypto.randomUUID(), active: false }]);
    }
    setEditing(null); setDraft(empty());
  });
  const remove = () => run(async () => {
    if (connectionId) { await DeleteTunnel(deleting); await reload(); }
    else onChange(tunnels.filter(item => item.id !== deleting));
    if (editing === deleting) { setEditing(null); setDraft(empty()); }
    setDeleting(null);
  });
  return <section className="connection-tunnels" aria-label={t("panel.tunnels")}>
    <h3>{t("panel.tunnels")}</h3>
    <p className="form-hint">{t(connectionId ? "connection.tunnels.immediate" : "connection.tunnels.draft")}</p>
    <p className="form-hint">{t("connection.tunnels.ssh")}</p>
    {error && <ResponseMessage severity="error" message={String(error)} />}
    {tunnels.length === 0 ? <p className="form-hint">{t("panel.noTunnels")}</p> : <div className="connection-tunnel-list">
      {tunnels.map(tunnel => <div key={tunnel.id} className="connection-tunnel-row">
        <code>127.0.0.1:{tunnel.localPort} → {tunnel.remoteHost}:{tunnel.remotePort}</code>
        <span className={tunnel.active ? "tunnel-active" : ""}>{t(tunnel.active ? "panel.listening" : "panel.inactive")}</span>
        <button type="button" className="btn-secondary" disabled={busy} onClick={() => { setEditing(tunnel.id); setDraft(tunnel); action.setResponse(null); }}>{t("common.edit")}</button>
        <button type="button" className="btn-secondary" disabled={busy} onClick={() => setDeleting(tunnel.id)}>{t("common.delete")}</button>
      </div>)}
    </div>}
    {deleting && <div className="connection-tunnel-confirm" role="alert">
      <span>{t("panel.deleteTunnelConfirm")}</span>
      <button type="button" className="btn-secondary" disabled={busy} onClick={() => setDeleting(null)}>{t("common.cancel")}</button>
      <button type="button" className="btn-primary" disabled={busy} onClick={remove}>{t("common.confirm")}</button>
    </div>}
    <fieldset className="response-fields" disabled={busy}>
      <div className="connection-tunnel-fields">
        <Input label={t("panel.localListeningPort")} type="number" min="1" max="65535" value={draft.localPort} onChange={localPort => setDraft(previous => ({ ...previous, localPort }))} />
        <Input label={t("panel.remoteHost")} value={draft.remoteHost} onChange={remoteHost => setDraft(previous => ({ ...previous, remoteHost }))} />
        <Input label={t("panel.remotePort")} type="number" min="1" max="65535" value={draft.remotePort} onChange={remotePort => setDraft(previous => ({ ...previous, remotePort }))} />
      </div>
      <div className="connection-tunnel-actions">
        {editing && <button type="button" className="btn-secondary" onClick={() => { setEditing(null); setDraft(empty()); }}>{t("common.cancel")}</button>}
        <button type="button" className="btn-secondary" onClick={submit}>{t(editing ? "panel.saveChanges" : "panel.addTunnel")}</button>
      </div>
    </fieldset>
    <ResponseMessage {...action.response} />
  </section>;
}
