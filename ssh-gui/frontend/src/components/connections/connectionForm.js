export const hasSSH = connection => connection.sshEnabled ?? connection.protocol !== "vnc";
export const hasVNC = connection => !!connection.vnc?.enabled;
export const defaultProtocol = connection => hasSSH(connection) ? "ssh" : "vnc";
export const connectionKey = (id, protocol) => `${id}:${protocol}`;

export function connectionForm(initial) {
  const form = {
    name: "", host: "", port: 22, username: "", authType: "password",
    password: initial ? undefined : "", privateKeyPath: "", passphrase: initial ? undefined : "", folderId: "", color: "#6b6490", jumpHostId: "",
    ...initial,
    sshEnabled: initial ? hasSSH(initial) : true,
    vnc: { enabled: false, port: 5900, username: "", password: initial ? undefined : "", ...initial?.vnc,
      reuse: { user: false, pass: false, ...initial?.vnc?.reuse } },
  };
  if (!form.sshEnabled) form.vnc.reuse = { user: false, pass: false };
  else if (form.authType !== "password") form.vnc.reuse.pass = false;
  return form;
}
