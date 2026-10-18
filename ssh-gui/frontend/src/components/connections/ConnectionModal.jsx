import "./ConnectionModal.css";
import { useState } from "react";
import { FaLock, FaCheck, FaTimes } from "react-icons/fa";
import { LuPlugZap } from "react-icons/lu";
import { TestConnectionParams } from "../../../bindings/ssh-gui/app";
import { useTranslation } from "react-i18next";

import ResponseMessage from "../common/ResponseMessage";
import useResponseAction from "../../hooks/useResponseAction";

function Field({ label, children }) {
  return (
    <div className="form-group">
      <label className="form-label">{label}</label>
      {children}
    </div>
  );
}

function Input({ label, value, onChange, type = "text", placeholder }) {
  return (
    <Field label={label}>
      <input
        className="modern-input"
        type={type}
        value={value ?? ""}
        placeholder={placeholder}
        onChange={e => onChange(e.target.value)}
      />
    </Field>
  );
}

export default function ConnectionModal({ initialValue, folders, connections = [], accountStatus, onSave, onCancel, onBusyChange }) {
  const { t } = useTranslation();
  const isPro = accountStatus?.tier === "pro";

  const [form, setForm] = useState(initialValue || {
    name: "", host: "", port: 22, username: "",
    password: "", authType: "password",
    privateKeyPath: "", passphrase: "", folderId: "", color: "#6b6490",
    jumpHostId: "",
  });

  const action = useResponseAction(onBusyChange);
  const [testState, setTestState] = useState(null); // null | 'testing' | { ok: boolean }

  const set = (k, v) => {
    setForm(p => ({ ...p, [k]: v }));
    setTestState(null);
    action.setResponse(null);
  };

  const handleTest = () => action.run(async () => {
    setTestState("testing");
    try {
      await TestConnectionParams(
        form.host, Number(form.port), form.username, form.authType,
        form.authType === "password" ? form.password ?? (initialValue?.id ? `__stored_connection_password__:${initialValue.id}` : "") : "",
        form.privateKeyPath || "",
        form.authType === "private_key" ? form.passphrase ?? (initialValue?.id ? `__stored_connection_password__:${initialValue.id}` : "") : "",
        form.jumpHostId || "",
      );
      setTestState({ ok: true });
    } catch (error) {
      setTestState({ ok: false });
      throw error;
    }
  }, t("connection.testing"), t("app.connectionFailed"), [form.password, form.passphrase]);

  const handleSave = () => action.run(() => { setTestState(null); return onSave(form); }, t("common.response.saving"), t("connection.response.saveFailed"), [form.password, form.passphrase]);

  return (
    <div>
      <div className="modal-header">
        <h2>{initialValue ? t("connection.editTitle") : t("connection.newTitle")}</h2>
        <p>{t("connection.description")}</p>
      </div>

      <div className="modal-body">
        <fieldset className="response-fields" disabled={action.busy}>
          <Input label={t("connection.name")} value={form.name} onChange={v => set("name", v)} placeholder={t("connection.namePlaceholder")} />

          <div className="form-grid-2">
            <Input label={t("connection.host")} value={form.host} onChange={v => set("host", v)} placeholder="192.168.1.100" />
            <Input label={t("connection.port")} value={form.port} onChange={v => set("port", Number(v))} placeholder="22" />
          </div>

          <Input label={t("connection.username")} value={form.username} onChange={v => set("username", v)} placeholder="root" />

          <div className="form-grid-2">
            <Field label={t("connection.folder")}>
              <select className="modern-input" value={form.folderId} onChange={e => set("folderId", e.target.value)}>
                <option value="">{t("connection.noFolder")}</option>
                {folders.map(f => <option key={f.id} value={f.id}>{f.name}</option>)}
              </select>
            </Field>
            <Field label={t("connection.authentication")}>
              <select className="modern-input" value={form.authType} onChange={e => set("authType", e.target.value)}>
                <option value="password">{t("connection.password")}</option>
                <option value="private_key">{t("connection.privateKey")}</option>
              </select>
            </Field>
          </div>

          {form.authType === "password" && (
            <Input label={t("connection.password")} type="password" value={form.password} onChange={v => set("password", v)} placeholder="••••••••" />
          )}
          {form.authType === "private_key" && (
            <>
              <Input label={t("connection.privateKeyPath")} value={form.privateKeyPath} onChange={v => set("privateKeyPath", v)} placeholder="~/.ssh/id_rsa" />
              <Input label={t("connection.passphrase")} type="password" value={form.passphrase || ""} onChange={v => set("passphrase", v)} placeholder={t("connection.passphraseOptional")} />
            </>
          )}

          <Field label={
            <span style={{ display: "flex", alignItems: "center", gap: 5 }}>
              {t("connection.jumpHost")}
              {!isPro && <span style={{ fontSize: 10, color: "var(--text-muted)" }}><FaLock /> Pro</span>}
            </span>
          }>
            {isPro ? (
              <select
                className="modern-input"
                value={form.jumpHostId || ""}
                onChange={e => set("jumpHostId", e.target.value || null)}
              >
                <option value="">{t("connection.direct")}</option>
                {connections
                  .filter(c => c.id !== initialValue?.id)
                  .map(c => (
                    <option key={c.id} value={c.id}>
                      {c.name} ({c.username}@{c.host}:{c.port})
                    </option>
                  ))}
              </select>
            ) : (
              <div
                style={{
                  padding: "7px 10px", borderRadius: 6,
                  background: "var(--bg-elevated)", border: "1px solid var(--border)",
                  fontSize: 12, color: "var(--text-muted)",
                  display: "flex", alignItems: "center", gap: 6,
                }}
              >
                <FaLock style={{ fontSize: 10 }} />
                {t("connection.jumpHostPro")}
              </div>
            )}
          </Field>

          <Field label={t("connection.color")}>
            <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
              <input
                className="modern-input connection-color"
                type="color" value={form.color}
                onChange={e => set("color", e.target.value)}
              />
              <span style={{ fontSize: "12px", color: "var(--text-muted)" }}>{form.color}</span>
            </div>
          </Field>
        </fieldset>
        <ResponseMessage {...(action.response || (testState?.ok ? { severity: "success", message: t("connection.connected") } : {}))} />
      </div>

      <div className="modal-footer">
        
        <button
            className={`btn-secondary test-conn-btn${testState === "testing" ? " testing" : testState?.ok === true ? " success" : testState?.ok === false ? " failure" : ""}`}
            onClick={handleTest}
            disabled={action.busy}
          >
            {testState === "testing" ? (
              <><span className="conn-loader" />{t("connection.testing")}</>
            ) : testState?.ok === true ? (
              <><FaCheck />{t("connection.connected")}</>
            ) : testState?.ok === false ? (
              <><FaTimes />{t("connection.failed")}</>
            ) : (
              <><LuPlugZap />{t("connection.test")}</>
            )}
          </button>
        <button className="btn-secondary" disabled={action.busy} onClick={() => onCancel()}>{t("common.cancel")}</button>
        <button className="btn-primary" disabled={action.busy} onClick={handleSave}>{t("connection.save")}</button>
      </div>

    </div>
  );
}
