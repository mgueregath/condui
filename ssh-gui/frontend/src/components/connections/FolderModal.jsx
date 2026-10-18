import { useState } from "react";
import { useTranslation } from "react-i18next";

import ResponseMessage from "../common/ResponseMessage";
import useResponseAction from "../../hooks/useResponseAction";

export default function FolderModal({ initialName = "", onSave, onBusyChange }) {
  const { t } = useTranslation();
  const action = useResponseAction(onBusyChange);
  const save = () => action.run(() => onSave(name), t("common.response.saving"), t("connection.response.folderFailed"));
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
            disabled={action.busy}
            value={name}
            onChange={e => { setName(e.target.value); action.setResponse(null); }}
            placeholder={t("connection.folderPlaceholder")}
          />
        </div>
        <ResponseMessage {...action.response} />
      </div>
      <div className="modal-footer">
        <button className="btn-primary" disabled={action.busy} onClick={save}>{t("connection.saveFolder")}</button>
      </div>
    </div>
  );
}
