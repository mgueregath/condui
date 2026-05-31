import { useState } from "react";

export default function ConnectionModal({
  initialValue,
  folders,
  onSave,
}) {

  const [form,
    setForm] =
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
        color: "#6366f1",
      }
    );

  return (

    <div>

      <div
        style={{
          padding:
            "24px 28px",

          borderBottom:
            "1px solid #e5e7eb",
        }}
      >

        <div
          style={{
            fontSize: "24px",
            fontWeight: 700,
          }}
        >
          {
            initialValue
              ? "Edit Connection"
              : "New Connection"
          }
        </div>

        <div
          style={{
            color: "#6b7280",
            marginTop: "6px",
          }}
        >
          Configure your SSH connection
        </div>

      </div>

      <div
        style={{
          padding: "28px",

          display: "grid",

          gap: "18px",
        }}
      >

        <Input
          label="Connection Name"
          value={form.name}
          onChange={(value) =>
            setForm({
              ...form,
              name: value,
            })
          }
        />

        <div
          style={{
            display: "grid",
            gridTemplateColumns:
              "1fr 120px",
            gap: "12px",
          }}
        >

          <Input
            label="Host"
            value={form.host}
            onChange={(value) =>
              setForm({
                ...form,
                host: value,
              })
            }
          />

          <Input
            label="Port"
            value={form.port}
            onChange={(value) =>
              setForm({
                ...form,
                port:
                  Number(value),
              })
            }
          />

        </div>

        <Input
          label="Username"
          value={form.username}
          onChange={(value) =>
            setForm({
              ...form,
              username: value,
            })
          }
        />

        <div>

          <label
            style={{
              display: "block",
              marginBottom: "8px",
              fontWeight: 600,
            }}
          >
            Folder
          </label>

          <select
            value={form.folderId}
            onChange={(e) =>
              setForm({
                ...form,
                folderId:
                  e.target.value,
              })
            }
            className="modern-input"
          >

            <option value="">
              No Folder
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

        </div>

        <div>

          <label
            style={{
              display: "block",
              marginBottom: "8px",
              fontWeight: 600,
            }}
          >
            Authentication
          </label>

          <select
            value={form.authType}
            onChange={(e) =>
              setForm({
                ...form,
                authType:
                  e.target.value,
              })
            }
            className="modern-input"
          >

            <option value="password">
              Password
            </option>

            <option value="private_key">
              Private Key
            </option>

          </select>

        </div>

        {form.authType ===
          "password" && (

          <Input
            label="Password"
            type="password"
            value={form.password}
            onChange={(value) =>
              setForm({
                ...form,
                password:
                  value,
              })
            }
          />

        )}

        {form.authType ===
          "private_key" && (

          <Input
            label="Private Key Path"
            value={
              form.privateKeyPath
            }
            onChange={(value) =>
              setForm({
                ...form,
                privateKeyPath:
                  value,
              })
            }
          />

        )}

        <div>

          <label
            style={{
              display: "block",
              marginBottom: "8px",
              fontWeight: 600,
            }}
          >
            Color
          </label>

          <input
            type="color"
            value={form.color}
            onChange={(e) =>
              setForm({
                ...form,
                color:
                  e.target.value,
              })
            }
          />

        </div>

      </div>

      <div
        style={{
          display: "flex",
          justifyContent:
            "flex-end",

          gap: "12px",

          padding: "24px",

          borderTop:
            "1px solid #e5e7eb",
        }}
      >

        <button
          className="btn-secondary"
        >
          Cancel
        </button>

        <button
          className="btn-primary"
          onClick={() =>
            onSave(form)
          }
        >
          Save Connection
        </button>

      </div>

    </div>

  );

}

function Input({
  label,
  value,
  onChange,
  type = "text",
}) {

  return (

    <div>

      <label
        style={{
          display: "block",
          marginBottom: "8px",
          fontWeight: 600,
        }}
      >
        {label}
      </label>

      <input
        type={type}
        value={value}
        onChange={(e) =>
          onChange(
            e.target.value
          )
        }
        className="modern-input"
      />

    </div>

  );

}