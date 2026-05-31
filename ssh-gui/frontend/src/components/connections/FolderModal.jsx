import { useState } from "react";

export default function FolderModal({ initialName = "", onSave }) {
  const [name, setName] = useState(initialName);

  return (
    <div>
      <div className="modal-header">
        <h2>{initialName ? "Edit Folder" : "New Folder"}</h2>
        <p>Organize your connections into folders</p>
      </div>
      <div className="modal-body">
        <div className="form-group">
          <label className="form-label">Folder Name</label>
          <input
            className="modern-input"
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="e.g. Production"
          />
        </div>
      </div>
      <div className="modal-footer">
        <button className="btn-primary" onClick={() => onSave(name)}>Save Folder</button>
      </div>
    </div>
  );
}
