import { useState } from "react";
import { FaLock, FaCheck, FaTimes } from "react-icons/fa";
import { LuPlugZap } from "react-icons/lu";
import { TestConnection, TestConnectionParams } from "../../../bindings/ssh-gui/app";
import { useTranslation } from "react-i18next";

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
        value={value}
        placeholder={placeholder}
        onChange={e => onChange(e.target.value)}
      />
    </Field>
  );
}

export default function ConnectionModal({ initialValue, folders, connections = [], accountStatus, onSave, onCancel }) {
  const { t } = useTranslation();
  const isPro = accountStatus?.tier === "pro";

  const [form, setForm] = useState(initialValue || {
    name: "", host: "", port: 22, username: "",
    password: "", authType: "password",
    privateKeyPath: "", folderId: "", color: "#6b6490",
    jumpHostId: "",
  });

  const [testState, setTestState] = useState(null); // null | 'testing' | { ok: true } | { ok: false, error: string }

  const set = (k, v) => {
    setForm(p => ({ ...p, [k]: v }));
    setTestState(null);
  };

  const handleTest = async () => {
    setTestState("testing");
    try {
      if (initialValue?.id) {
        // Editing an existing connection: use the stored (vault-encrypted) credentials
        await TestConnection(initialValue.id);
      } else {
        // New connection: use raw form data (password is available in plaintext)
        await TestConnectionParams(
          form.host,
          Number(form.port),
          form.username,
          form.authType,
          form.password || "",
          form.privateKeyPath || "",
          form.jumpHostId || "",
        );
      }
      setTestState({ ok: true });
    } catch (err) {
      setTestState({ ok: false, error: typeof err === "string" ? err : err?.message || t("app.connectionFailed") });
    }
  };

  return (
    <div>
      <div className="modal-header">
        <h2>{initialValue ? t("connection.editTitle") : t("connection.newTitle")}</h2>
        <p>{t("connection.description")}</p>
      </div>

      <div className="modal-body">
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
          <Input label={t("connection.privateKeyPath")} value={form.privateKeyPath} onChange={v => set("privateKeyPath", v)} placeholder="~/.ssh/id_rsa" />
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
              type="color" value={form.color}
              onChange={e => set("color", e.target.value)}
              style={{ width: "40px", height: "32px", border: "none", borderRadius: "6px", cursor: "pointer", background: "transparent" }}
            />
            <span style={{ fontSize: "12px", color: "var(--text-muted)" }}>{form.color}</span>
          </div>
        </Field>
      </div>

      <div className="modal-footer">
        <div className="test-conn-wrap">
          <button
            className={`btn-secondary test-conn-btn${testState === "testing" ? " testing" : testState?.ok === true ? " success" : testState?.ok === false ? " failure" : ""}`}
            onClick={handleTest}
            disabled={testState === "testing"}
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
          {testState?.ok === false && testState.error && (
            <div className="test-conn-error">{testState.error}</div>
          )}
        </div>
        <button className="btn-secondary" onClick={() => onCancel()}>{t("common.cancel")}</button>
        <button className="btn-primary" onClick={() => onSave(form)}>{t("connection.save")}</button>
      </div>
    </div>
  );
}
