import { useTranslation } from "react-i18next";
import Modal from "./Modal";

export default function AlertModal({ open, title, message, variant = "info", onClose }) {
  const { t } = useTranslation();

  return (
    <Modal open={open} onClose={onClose}>
      <div>
        <div className="modal-header">
          <h2>{title}</h2>
        </div>
        <div className="modal-body">
          <div
            className="ssh-error-box"
            style={variant === "error" ? { borderColor: "var(--red)" } : undefined}
          >
            <p style={{ margin: 0 }}>{message}</p>
          </div>
        </div>
        <div className="modal-footer">
          <button className="btn-primary" onClick={onClose}>
            {t("common.ok")}
          </button>
        </div>
      </div>
    </Modal>
  );
}
