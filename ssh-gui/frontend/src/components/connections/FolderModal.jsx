import { useState } from "react";
import { useTranslation } from "react-i18next";

export default function FolderModal({ initialName = "", onSave }) {
  const { t } = useTranslation();
  const [name, setName] = useState(initialName);

  return (
    <div>
      <div className="modal-header">
        <h2>{initialName ? t("connection.folderEdit") : t("connection.folderNew")}</h2>
        <p>{t("connection.folderDescription")}</p>
      </div>
      <div className="modal-body">
        <div className="form-group">
          <label className="form-label">{t("connection.folderName")}</label>
          <input
            className="modern-input"
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder={t("connection.folderPlaceholder")}
          />
        </div>
      </div>
      <div className="modal-footer">
        <button className="btn-primary" onClick={() => onSave(name)}>{t("connection.saveFolder")}</button>
      </div>
    </div>
  );
}
