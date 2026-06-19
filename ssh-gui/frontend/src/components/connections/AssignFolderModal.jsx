import { useState } from "react";

export default function AssignFolderModal({ connection, folders, onSave }) {
  const [folderId, setFolderId] = useState(connection?.folderId || "");

  return (
    <div>
      <div className="modal-header">
        <h2>Assign to folder</h2>
        <p>Move <strong>{connection?.name}</strong> to a folder</p>
      </div>
      <div className="modal-body">
        <div className="form-group">
          <label className="form-label">Folder</label>
          <select
            className="modern-input"
            value={folderId}
            onChange={e => setFolderId(e.target.value)}
          >
            <option value="">— No folder —</option>
            {folders.map(f => (
              <option key={f.id} value={f.id}>{f.name}</option>
            ))}
          </select>
        </div>
      </div>
      <div className="modal-footer">
        <button className="btn-primary" onClick={() => onSave({ ...connection, folderId })}>
          Save
        </button>
      </div>
    </div>
  );
}