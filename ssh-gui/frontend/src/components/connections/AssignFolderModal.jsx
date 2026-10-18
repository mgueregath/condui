import { useState } from "react";
import { useTranslation } from "react-i18next";

import ResponseMessage from "../common/ResponseMessage";
import useResponseAction from "../../hooks/useResponseAction";

export default function AssignFolderModal({ connection, folders, onSave, onBusyChange }) {
  const { t } = useTranslation();
  const action = useResponseAction(onBusyChange);
  const save = () => action.run(() => onSave({ ...connection, folderId }), t("common.response.saving"), t("connection.response.folderFailed"));
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
            disabled={action.busy}
            value={folderId}
            onChange={e => { setFolderId(e.target.value); action.setResponse(null); }}
          >
            <option value="">{t("connection.noFolder")}</option>
            {folders.map(f => (
              <option key={f.id} value={f.id}>{f.name}</option>
            ))}
          </select>
        </div>
        <ResponseMessage {...action.response} />
      </div>
      <div className="modal-footer">
        <button className="btn-primary" disabled={action.busy} onClick={save}>
          {t("common.save")}
        </button>
      </div>
    </div>
  );
}
