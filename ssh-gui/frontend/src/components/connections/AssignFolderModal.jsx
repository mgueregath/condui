import { useState } from "react";
import { useTranslation } from "react-i18next";

export default function AssignFolderModal({ connection, folders, onSave }) {
  const { t } = useTranslation();
  const [folderId, setFolderId] = useState(connection?.folderId || "");

  return (
    <div>
      <div className="modal-header">
        <h2>{t("connection.assignTitle")}</h2>
        <p>{t("connection.moveToFolder", { name: connection?.name })}</p>
      </div>
      <div className="modal-body">
        <div className="form-group">
          <label className="form-label">{t("connection.folder")}</label>
          <select
            className="modern-input"
            value={folderId}
            onChange={e => setFolderId(e.target.value)}
          >
            <option value="">{t("connection.noFolder")}</option>
            {folders.map(f => (
              <option key={f.id} value={f.id}>{f.name}</option>
            ))}
          </select>
        </div>
      </div>
      <div className="modal-footer">
        <button className="btn-primary" onClick={() => onSave({ ...connection, folderId })}>
          {t("common.save")}
        </button>
      </div>
    </div>
  );
}
