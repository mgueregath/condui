import { useTranslation } from "react-i18next";
import ResponseMessage from "./ResponseMessage";
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
          <ResponseMessage severity={variant} message={message} />
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
