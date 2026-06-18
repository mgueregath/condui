import { useEffect, useState } from "react";
import { CancelShare, GetSentShares, ShareConnection } from "../../../bindings/ssh-gui/app";

function statusLabel(status) {
  if (status === "accepted") return "Accepted";
  if (status === "revoked") return "Revoked";
  return "Pending";
}

export default function ShareModal({ connection, onClose }) {
  const [email, setEmail] = useState("");
  const [readOnly, setReadOnly] = useState(true);
  const [includePassword, setIncludePassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [sentLoading, setSentLoading] = useState(false);
  const [cancellingId, setCancellingId] = useState("");
  const [error, setError] = useState("");
  const [sentShares, setSentShares] = useState([]);

  const loadSentShares = async () => {
    setSentLoading(true);
    try {
      const shares = await GetSentShares();
      setSentShares((shares || []).filter((share) =>
        share.status !== "revoked" && share.blobId?.includes(connection.id),
      ));
    } catch (err) {
      console.error("Failed to load sent shares", err);
    } finally {
      setSentLoading(false);
    }
  };

  useEffect(() => {
    loadSentShares();
  }, []);

  const handleShare = async (e) => {
    e.preventDefault();
    setError("");
    const targetEmail = email.trim().toLowerCase();
    if (!targetEmail.includes("@")) {
      setError("Enter a valid email address");
      return;
    }
    setLoading(true);
    try {
      await ShareConnection(connection.id, targetEmail, readOnly, includePassword);
      setEmail("");
      await loadSentShares();
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || "Share failed");
    } finally {
      setLoading(false);
    }
  };

  const handleCancel = async (share) => {
    setCancellingId(share.id);
    setError("");
    try {
      await CancelShare(share.id);
      await loadSentShares();
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || "Unable to cancel share");
    } finally {
      setCancellingId("");
    }
  };

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
            onChange={(e) => setEmail(e.target.value)}
            autoFocus
          />

          <label className="form-label" style={{ marginTop: 12 }}>Permissions</label>
          <div className="permission-toggle">
            <label className={`perm-option${readOnly ? " active" : ""}`}>
              <input
                type="radio"
                name="perm"
                checked={readOnly}
                onChange={() => setReadOnly(true)}
                style={{ display: "none" }}
              />
              Read-only
            </label>
            <label className={`perm-option${!readOnly ? " active" : ""}`}>
              <input
                type="radio"
                name="perm"
                checked={!readOnly}
                onChange={() => setReadOnly(false)}
                style={{ display: "none" }}
              />
              Read & write
            </label>
          </div>

          <p style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 8 }}>
            Shared passwords are encrypted end-to-end and saved in the recipient vault.
          </p>

          <label className="share-secret-option">
            <input
              type="checkbox"
              checked={includePassword}
              onChange={(e) => setIncludePassword(e.target.checked)}
            />
            <span>Include connection password</span>
          </label>

          {error && <div className="vault-error">{error}</div>}

          <button className="btn-primary" type="submit" disabled={loading}>
            {loading ? "Sending..." : "Send invitation"}
          </button>
        </form>

        <div className="share-list">
          <div className="share-list-header">
            <span>Invited users</span>
            <button className="link-btn" onClick={loadSentShares} disabled={sentLoading}>
              {sentLoading ? "Refreshing..." : "Refresh"}
            </button>
          </div>

          {sentShares.length === 0 ? (
            <div className="share-empty">No active invitations or shares yet.</div>
          ) : (
            sentShares.map((share) => (
              <div className="share-row" key={share.id}>
                <div className="share-row-main">
                  <strong>{share.recipientEmail || "Unknown recipient"}</strong>
                  <span>{share.permissions === "write" ? "Read & write" : "Read-only"}</span>
                </div>
                <span className={`share-status share-status-${share.status || "pending"}`}>
                  {statusLabel(share.status)}
                </span>
                <button
                  className="btn-secondary btn-sm"
                  disabled={cancellingId === share.id}
                  onClick={() => {
                    if (confirm("Cancel this invitation/share?")) handleCancel(share);
                  }}
                >
                  {cancellingId === share.id ? "Cancelling..." : "Cancel"}
                </button>
              </div>
            ))
          )}
        </div>
      </div>
      <div className="modal-footer">
        <button className="btn-secondary" onClick={onClose}>Close</button>
      </div>
    </div>
  );
}
