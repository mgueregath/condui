import { useState } from "react";

export default function ConnectionModal({
  initialValue,
  folders,
  onSave,
}) {

  const [form, setForm] =
    useState(
      initialValue || {
        name: "",
        host: "",
        port: 22,
        username: "",
        password: "",
        authType: "password",
        privateKeyPath: "",
        folderId: "",
        color: "#3b82f6",
      }
    );

  return (
    <div
      style={{
        width: "500px",
        display: "flex",
        flexDirection: "column",
        gap: "12px",
      }}
    >

      <h3>
        {
          initialValue
            ? "Editar conexión"
            : "Nueva conexión"
        }
      </h3>

      <input
        placeholder="Nombre"
        value={form.name}
        onChange={(e) =>
          setForm({
            ...form,
            name: e.target.value,
          })
        }
      />

      <select
        value={form.folderId}
        onChange={(e) =>
          setForm({
            ...form,
            folderId: e.target.value,
          })
        }
      >
        <option value="">
          Sin carpeta
        </option>

        {folders.map(folder => (
          <option
            key={folder.id}
            value={folder.id}
          >
            {folder.name}
          </option>
        ))}
      </select>

      <input
        placeholder="Host"
        value={form.host}
        onChange={(e) =>
          setForm({
            ...form,
            host: e.target.value,
          })
        }
      />

      <input
        type="number"
        placeholder="Puerto"
        value={form.port}
        onChange={(e) =>
          setForm({
            ...form,
            port: Number(e.target.value),
          })
        }
      />

      <input
        placeholder="Usuario"
        value={form.username}
        onChange={(e) =>
          setForm({
            ...form,
            username: e.target.value,
          })
        }
      />

      <select
        value={form.authType}
        onChange={(e) =>
          setForm({
            ...form,
            authType: e.target.value,
          })
        }
      >
        <option value="password">
          Password
        </option>

        <option value="private_key">
          Private Key
        </option>
      </select>

      {form.authType === "password" && (
        <input
          type="password"
          placeholder="Password"
          value={form.password}
          onChange={(e) =>
            setForm({
              ...form,
              password: e.target.value,
            })
          }
        />
      )}

      {form.authType === "private_key" && (
        <input
          placeholder="Ruta llave privada"
          value={form.privateKeyPath}
          onChange={(e) =>
            setForm({
              ...form,
              privateKeyPath:
                e.target.value,
            })
          }
        />
      )}

      <div>

        <label>
          Color
        </label>

        <input
          type="color"
          value={form.color}
          onChange={(e) =>
            setForm({
              ...form,
              color: e.target.value,
            })
          }
        />

      </div>

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