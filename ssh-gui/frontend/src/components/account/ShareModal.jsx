import { useState } from "react";
import { ShareConnection } from "../../../bindings/ssh-gui/app";

export default function ShareModal({ connection, onClose }) {
  const [email, setEmail] = useState("");
  const [readOnly, setReadOnly] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  const handleShare = async (e) => {
    e.preventDefault();
    setError("");
    if (!email.includes("@")) {
      setError("Enter a valid email address");
      return;
    }
    setLoading(true);
    try {
      await ShareConnection(connection.id, email, readOnly);
      setSuccess(true);
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || "Share failed");
    } finally {
      setLoading(false);
    }
  };

  if (success) {
    return (
      <div>
        <div className="modal-header">
          <h2>Invitation sent</h2>
          <p>Share successful</p>
        </div>
        <div className="modal-body">
          <div className="vault-success">
            Invitation sent to <strong>{email}</strong>. They will see it in Condui when they log in.
          </div>
        </div>
        <div className="modal-footer">
          <button className="btn-primary" onClick={onClose}>Done</button>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="modal-header">
        <h2>Share connection</h2>
        <p>{connection?.name}</p>
      </div>
      <div className="modal-body">
        <form onSubmit={handleShare} className="vault-form">
          <label className="form-label">Recipient email</label>
          <input
            className="modern-input"
            type="email"
            placeholder="colleague@example.com"
            value={email}
            onChange={e => setEmail(e.target.value)}
            autoFocus
          />

          <label className="form-label" style={{ marginTop: 12 }}>Permissions</label>
          <div className="permission-toggle">
            <label className={`perm-option${readOnly ? " active" : ""}`}>
              <input
                type="radio" name="perm" checked={readOnly}
                onChange={() => setReadOnly(true)} style={{ display: "none" }}
              />
              Read-only
            </label>
            <label className={`perm-option${!readOnly ? " active" : ""}`}>
              <input
                type="radio" name="perm" checked={!readOnly}
                onChange={() => setReadOnly(false)} style={{ display: "none" }}
              />
              Read & write
            </label>
          </div>

          <p style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 8 }}>
            Note: credentials are not shared. The recipient will need to add their own password.
          </p>

          {error && <div className="vault-error">{error}</div>}

          <button className="btn-primary" type="submit" disabled={loading}>
            {loading ? "Sending…" : "Send invitation"}
          </button>
        </form>
      </div>
      <div className="modal-footer">
        <button className="btn-secondary" onClick={onClose}>Cancel</button>
      </div>
    </div>
  );
}
