#!/bin/bash

set -e

echo "========================================"
echo "Incremento 5.2 -> 5.7"
echo "========================================"

cd ssh-gui

mkdir -p frontend/src/components/common
mkdir -p frontend/src/components/connections
mkdir -p frontend/src/styles

####################################################
# COMMON
####################################################

cat > frontend/src/components/common/Modal.jsx <<'EOF'
export default function Modal({
  open,
  children,
  onClose,
}) {

  if (!open) {
    return null;
  }

  return (
    <div className="modal-overlay">

      <div className="modal">

        {children}

      </div>

    </div>
  );

}
EOF

cat > frontend/src/components/common/ConfirmDialog.jsx <<'EOF'
export default function ConfirmDialog({
  open,
  title,
  onConfirm,
  onCancel,
}) {

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
            Confirmar
          </button>

          <button
            onClick={onCancel}
          >
            Cancelar
          </button>

        </div>

      </div>

    </div>
  );

}
EOF

####################################################
# CONNECTIONS
####################################################

cat > frontend/src/components/connections/FolderModal.jsx <<'EOF'
import { useState } from "react";

export default function FolderModal({
  onSave,
}) {

  const [name,setName] =
    useState("");

  return (
    <div>

      <h3>Nueva carpeta</h3>

      <input
        value={name}
        onChange={e =>
          setName(e.target.value)
        }
      />

      <button
        onClick={() =>
          onSave(name)
        }
      >
        Guardar
      </button>

    </div>
  );

}
EOF

cat > frontend/src/components/connections/ConnectionModal.jsx <<'EOF'
import { useState } from "react";

export default function ConnectionModal({
  folders,
  onSave,
}) {

  const [form,setForm] =
    useState({
      name:"",
      host:"",
      port:22,
      username:"",
      password:"",
      folderId:"",
    });

  return (
    <div>

      <h3>Nueva conexión</h3>

      <input
        placeholder="Nombre"
        onChange={e =>
          setForm({
            ...form,
            name:e.target.value,
          })
        }
      />

      <input
        placeholder="Host"
        onChange={e =>
          setForm({
            ...form,
            host:e.target.value,
          })
        }
      />

      <button
        onClick={() =>
          onSave(form)
        }
      >
        Guardar
      </button>

    </div>
  );

}
EOF

cat > frontend/src/components/connections/FolderNode.jsx <<'EOF'
export default function FolderNode({
  folder,
  expanded,
  onToggle,
  children,
}) {

  return (
    <div>

      <div
        onClick={() =>
          onToggle(folder.id)
        }
      >
        {expanded ? "▼" : "▶"} {folder.name}
      </div>

      {expanded && children}

    </div>
  );

}
EOF

cat > frontend/src/components/connections/ConnectionNode.jsx <<'EOF'
export default function ConnectionNode({
  connection,
  onOpen,
  onEdit,
  onDelete,
}) {

  return (

    <div
      onDoubleClick={() =>
        onOpen(connection)
      }
    >

      🖥 {connection.name}

      <button
        onClick={() =>
          onEdit(connection)
        }
      >
        ✎
      </button>

      <button
        onClick={() =>
          onDelete(connection)
        }
      >
        🗑
      </button>

    </div>

  );

}
EOF

cat > frontend/src/components/connections/ConnectionDrawer.jsx <<'EOF'
export default function ConnectionDrawer() {
  return null;
}
EOF

####################################################
# CSS
####################################################

cat > frontend/src/styles/modal.css <<'EOF'
.modal-overlay {

  position: fixed;

  inset: 0;

  background: rgba(0,0,0,.4);

  display:flex;

  justify-content:center;

  align-items:center;

}

.modal {

  width:500px;

  background:#25262b;

  border:1px solid #333;

  padding:20px;

}
EOF

cat > frontend/src/styles/connections.css <<'EOF'
.connection-item {

  padding:6px 12px;

}

.folder-item {

  padding:8px 12px;

  font-weight:bold;

}
EOF

echo ""
echo "Archivos creados correctamente"
echo ""