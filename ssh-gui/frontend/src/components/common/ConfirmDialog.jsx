import { useTranslation } from "react-i18next";

export default function ConfirmDialog({
  open,
  title,
  onConfirm,
  onCancel,
}) {
  const { t } = useTranslation();

  if (!open) {
    return null;
  }

  return (
    <div className="modal-overlay">

      <div className="modal">

        <h3>{title}</h3>

        <div
          style={{
            display:"flex",
            gap:"10px",
            marginTop:"20px",
          }}
        >

          <button
            onClick={onConfirm}
          >
            {t("common.confirm")}
          </button>

          <button
            onClick={onCancel}
          >
            {t("common.cancel")}
          </button>

        </div>

      </div>

    </div>
  );

}
