export const protocols = ["ssh", "vnc", "rdp"];
export const hasSSH = connection => connection.sshEnabled ?? (connection.protocol || "ssh") === "ssh";
export const hasVNC = connection => !!connection.vnc?.enabled;
export const hasRDP = connection => !!connection.rdp?.enabled;
export const hasAccess = (connection, protocol) => protocol === "ssh" ? hasSSH(connection) : !!connection[protocol]?.enabled;
export const defaultProtocol = connection => protocols.find(protocol => hasAccess(connection, protocol)) || "ssh";
export const isDesktop = protocol => protocol === "vnc" || protocol === "rdp";
export const connectionKey = (id, protocol) => `${id}:${protocol}`;

function desktopConfig(initial, protocol, port) {
  return { enabled: false, port, username: "", password: initial ? undefined : "", ...initial?.[protocol],
    reuse: { user: false, pass: false, ...initial?.[protocol]?.reuse } };
}
export function connectionForm(initial) {
  const form = {
    name: "", host: "", port: 22, username: "", authType: "password",
    password: initial ? undefined : "", privateKeyPath: "", passphrase: initial ? undefined : "", folderId: "", color: "#6b6490", jumpHostId: "",
    ...initial, sshEnabled: initial ? hasSSH(initial) : true,
    vnc: desktopConfig(initial, "vnc", 5900),
    rdp: { resolution: "1920x1080", allowUntrusted: false, disableTLS: false, domain: "", certificateFingerprint: "", ...desktopConfig(initial, "rdp", 3389) },
  };
  for (const protocol of ["vnc", "rdp"]) {
    if (!form.sshEnabled) form[protocol].reuse = { user: false, pass: false };
    else if (form.authType !== "password") form[protocol].reuse.pass = false;
  }
  return form;
}
