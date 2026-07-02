import { useEffect, useState } from "react";
import { CancelShare, GetSentShares, ShareConnection } from "../../../bindings/ssh-gui/app";
import { useTranslation } from "react-i18next";

function statusLabel(status, t) {
  if (status === "accepted") return t("share.accepted");
  if (status === "revoked") return t("share.revoked");
  return t("share.pending");
}

export default function ShareModal({ connection, onClose }) {
  const { t } = useTranslation();
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
      setError(t("share.invalidEmail"));
      return;
    }
    setLoading(true);
    try {
      await ShareConnection(connection.id, targetEmail, readOnly, includePassword);
      setEmail("");
      await loadSentShares();
    } catch (err) {
      setError(typeof err === "string" ? err : err?.message || t("share.failed"));
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
      setError(typeof err === "string" ? err : err?.message || t("share.cancelFailed"));
    } finally {
      setCancellingId("");
    }
  };

  return (
    <div>
      <div className="modal-header">
        <h2>{t("share.title")}</h2>
        <p>{connection?.name}</p>
      </div>
      <div className="modal-body">
        <form onSubmit={handleShare} className="vault-form">
          <label className="form-label">{t("share.recipientEmail")}</label>
          <input
            className="modern-input"
            type="email"
            placeholder="colleague@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoFocus
          />

          <label className="form-label" style={{ marginTop: 12 }}>{t("share.permissions")}</label>
          <div className="permission-toggle">
            <label className={`perm-option${readOnly ? " active" : ""}`}>
              <input
                type="radio"
                name="perm"
                checked={readOnly}
                onChange={() => setReadOnly(true)}
                style={{ display: "none" }}
              />
              {t("app.readOnly")}
            </label>
            <label className={`perm-option${!readOnly ? " active" : ""}`}>
              <input
                type="radio"
                name="perm"
                checked={!readOnly}
                onChange={() => setReadOnly(false)}
                style={{ display: "none" }}
              />
              {t("app.readWrite")}
            </label>
          </div>

          <p style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 8 }}>
            {t("share.encryptedHelp")}
          </p>

          <label className="share-secret-option">
            <input
              type="checkbox"
              checked={includePassword}
              onChange={(e) => setIncludePassword(e.target.checked)}
            />
            <span>{t("share.includePassword")}</span>
          </label>

          {error && <div className="vault-error">{error}</div>}

          <button className="btn-primary" type="submit" disabled={loading}>
            {loading ? t("share.sending") : t("share.sendInvitation")}
          </button>
        </form>

        <div className="share-list">
          <div className="share-list-header">
            <span>{t("share.invitedUsers")}</span>
            <button className="link-btn" onClick={loadSentShares} disabled={sentLoading}>
              {sentLoading ? t("share.refreshing") : t("common.refresh")}
            </button>
          </div>

          {sentShares.length === 0 ? (
            <div className="share-empty">{t("share.empty")}</div>
          ) : (
            sentShares.map((share) => (
              <div className="share-row" key={share.id}>
                <div className="share-row-main">
                  <strong>{share.recipientEmail || t("share.unknownRecipient")}</strong>
                  <span>{share.permissions === "write" ? t("app.readWrite") : t("app.readOnly")}</span>
                </div>
                <span className={`share-status share-status-${share.status || "pending"}`}>
                  {statusLabel(share.status, t)}
                </span>
                <button
                  className="btn-secondary btn-sm"
                  disabled={cancellingId === share.id}
                  onClick={() => {
                    if (confirm(t("share.cancelShareConfirm"))) handleCancel(share);
                  }}
                >
                  {cancellingId === share.id ? t("share.cancelling") : t("common.cancel")}
                </button>
              </div>
            ))
          )}
        </div>
      </div>
      <div className="modal-footer">
        <button className="btn-secondary" onClick={onClose}>{t("common.close")}</button>
      </div>
    </div>
  );
}
