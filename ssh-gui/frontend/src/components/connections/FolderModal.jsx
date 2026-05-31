import { useState } from "react";

export default function FolderModal({
  initialName = "",
  onSave,
}) {

  const [name,setName] =
    useState(initialName)

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
