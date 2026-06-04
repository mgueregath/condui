import { useState } from "react";

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

export default function ConnectionModal({ initialValue, folders, onSave, onCancel }) {
  const [form, setForm] = useState(initialValue || {
    name: "", host: "", port: 22, username: "",
    password: "", authType: "password",
    privateKeyPath: "", folderId: "", color: "#eeecf9",
  });

  const set = (k, v) => setForm(p => ({ ...p, [k]: v }));

  return (
    <div>
      <div className="modal-header">
        <h2>{initialValue ? "Edit Connection" : "New Connection"}</h2>
        <p>Configure your SSH connection details</p>
      </div>

      <div className="modal-body">
        <Input label="Connection Name" value={form.name} onChange={v => set("name", v)} placeholder="My Server" />

        <div className="form-grid-2">
          <Input label="Host" value={form.host} onChange={v => set("host", v)} placeholder="192.168.1.100" />
          <Input label="Port" value={form.port} onChange={v => set("port", Number(v))} placeholder="22" />
        </div>

        <Input label="Username" value={form.username} onChange={v => set("username", v)} placeholder="root" />

        <div className="form-grid-2">
          <Field label="Folder">
            <select className="modern-input" value={form.folderId} onChange={e => set("folderId", e.target.value)}>
              <option value="">No Folder</option>
              {folders.map(f => <option key={f.id} value={f.id}>{f.name}</option>)}
            </select>
          </Field>
          <Field label="Authentication">
            <select className="modern-input" value={form.authType} onChange={e => set("authType", e.target.value)}>
              <option value="password">Password</option>
              <option value="private_key">Private Key</option>
            </select>
          </Field>
        </div>

        {form.authType === "password" && (
          <Input label="Password" type="password" value={form.password} onChange={v => set("password", v)} placeholder="••••••••" />
        )}
        {form.authType === "private_key" && (
          <Input label="Private Key Path" value={form.privateKeyPath} onChange={v => set("privateKeyPath", v)} placeholder="~/.ssh/id_rsa" />
        )}

        <Field label="Color">
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
        <button className="btn-secondary" onClick={() => onCancel()}>Cancel</button>
        <button className="btn-primary" onClick={() => onSave(form)}>Save Connection</button>
      </div>
    </div>
  );
}
