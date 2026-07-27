<p align="center">
  <img src="ssh-gui/frontend/src/assets/images/condui-transparent.png" width="160">
</p>

<h1 align="center">
condui
</h1>

<p align="center">
A modern remote infrastructure workspace.
<br/>
One interface. All your servers.
</p>

<p align="center">

![License](https://img.shields.io/badge/license-AGPL--3.0-purple)
![Go](https://img.shields.io/badge/backend-Go-00ADD8)
![React](https://img.shields.io/badge/frontend-React-61DAFB)
![Wails](https://img.shields.io/badge/desktop-Wails%20v3-red)
![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-green)

</p>

---

# What is condui?

**condui** is a cross-platform desktop workspace for developers, DevOps engineers and infrastructure teams that work with remote machines every day.

It brings SSH terminals, encrypted connection management, SFTP, tunnels, Docker operations, database exploration, VirtualBox controls, account sync and secure sharing into one desktop application.

condui is designed as a modern alternative to:

- MobaXterm
- Termius
- SecureCRT
- traditional SSH managers
- disconnected combinations of terminal, SFTP, tunnel and Docker tools

Built with:

- Go
- React
- Wails v3
- SQLite
- SSH, SFTP and native database drivers

---

# Features

## SSH Workspace

A unified workspace for managing remote SSH sessions.

- Save and organize SSH connections.
- Group connections in folders with color metadata.
- Open multiple terminal tabs and switch between active sessions.
- Connect with password or private key authentication.
- Use jump hosts / bastion hosts for routed access.
- Test a connection (saved or unsaved, including through a jump host) before opening it.
- Open a local shell terminal tab alongside remote SSH sessions.
- Resize terminal PTYs from the UI.
- Detect disconnections and surface reconnect/disconnect actions.
- Verify SSH host keys with a TOFU known-hosts flow.
- Reject changed host keys to help prevent MITM attacks.
- Keep credentials redacted from the frontend.

## Encrypted Local Vault

condui stores sensitive connection data locally with a master-password vault.

- Create and unlock a local vault with a master password.
- Derive vault keys with Argon2id.
- Encrypt saved connection passwords before they are persisted.
- Keep the vault key only in memory while unlocked.
- Lock the vault without logging out of the account.
- Preserve existing passwords when editing a connection without re-entering them.

## Connection Management

- Create, edit and delete connections.
- Confirm before deleting a connection.
- Save host, port, username, auth type, private key path and jump host.
- Assign connections to folders.
- Move and synchronize grouped connections.
- Keep local connection state in SQLite.
- Trigger background sync after connection or folder changes when the account and vault are ready.

## SFTP Remote File Explorer

Integrated SFTP file operations for each active SSH session.

- Browse remote directories.
- Upload files with transfer progress.
- Download remote files through the native save dialog.
- Delete and rename remote files.
- Create remote directories.
- Read remote files from the UI.
- Edit and save remote text files.
- Preview common image files through base64 transfer.

## SSH Tunnels

Manage local port forwarding over an active SSH session.

- Create tunnels with local port, remote host and remote port.
- Start and stop tunnels on demand.
- Edit existing tunnel definitions.
- Delete tunnels and automatically stop running forwards.
- Use tunnels for databases, dashboards and internal services.

## Docker Management

Inspect and operate Docker on remote hosts through SSH.

- List containers.
- Start, stop and restart containers.
- Stream Docker logs from selected containers.
- Fetch one-shot container CPU and memory stats.
- Detect listening ports on the remote host.
- Search and sort containers, open ports, VMs and databases within the bottom panel.
- Discover database services exposed by Docker or the host.
- View host-level CPU, memory, disk, uptime, network and disk I/O stats.

## Database Explorer

Explore databases through SSH-tunneled native Go drivers.

- Discover PostgreSQL and MySQL services from remote environments.
- Connect to databases through the current SSH session.
- Save and reuse database credentials locally.
- List databases, schemas, tables and columns.
- Execute SQL queries and inspect result sets.
- Disconnect database sessions and clean up resources.
- Open database tooling through condui's local embedded HTTP API.

## VirtualBox Management

Control VirtualBox installations on remote hosts where `VBoxManage` is available.

- Detect whether VirtualBox is installed.
- List virtual machines.
- Start VMs in GUI or headless mode.
- Stop VMs with ACPI or force power-off.
- Pause, resume, reset and save VM state.

## Accounts, Pro Sync and Multi-Device Access

condui includes an optional sync server for account-backed workflows.

- Register, log in and log out from the desktop app.
- Refresh access tokens automatically while the app is running.
- Track account tier and last sync status.
- Free tier syncs a capped connection set.
- Pro tier synchronizes the same encrypted connection and folder set across devices.
- Merge remote and local changes during sync.
- Use lightweight remote metadata and checksums to avoid downloading unchanged data.
- Poll periodically in Pro mode so devices converge without manual refresh.
- Keep synced connection data encrypted end-to-end from the client side.

## Secure Connection Sharing

Share individual connections with other condui users by email.

- Look up a recipient public key by email.
- Share a connection with read-only metadata.
- Optionally include the connection password in the encrypted share payload.
- Encrypt each share with a random key and wrap it for the recipient using X25519.
- Show pending incoming invitations in a virtual "Pending invitations" folder.
- Accept or decline shared connection invitations.
- Re-encrypt accepted shared credentials into the recipient's own local vault.
- List sent invitations from the share modal.
- Show invitation status and read-only state.
- Cancel pending invitations or revoke existing shared access.

## Interface & Usability

- Switch the interface language between English and Spanish at runtime.
- Drag and drop files onto the remote file explorer to upload them.
- Consistent, sortable and searchable panel UI across Docker, ports, database and VirtualBox tabs.

## Sync Server

The account, sync and sharing backend lives in its own repository,
[condui-server](https://github.com/mgueregath/condui-server), and is consumed by the desktop app purely over
HTTP — there is no build-time dependency between the two. Point a build at your own instance (or the hosted
one) via the `server_url` field in `ssh-gui/build.config.yaml` (see [Build Configuration](#build-configuration)).

- Email/password account registration and login.
- JWT access tokens and refresh tokens.
- Tier-aware limits for free and Pro accounts.
- Encrypted blob storage for client-side sync payloads.
- Blob metadata endpoint with size, version, timestamps and checksum.
- Per-user identity blob for encrypted sharing keys.
- Share invitations with sender, recipient, blob metadata, encrypted keys and status.
- Access checks so recipients can fetch shared blobs they are allowed to read.
- SQLite-backed storage and migrations.
- Management CLI helpers for server operations.

---

# Security Model

condui is designed so plaintext connection secrets stay on the user's device whenever possible.

- Local connection passwords are encrypted before storage.
- The frontend receives redacted connection passwords.
- SSH passwords are decrypted only in the backend when opening a session.
- Sync payloads are encrypted before upload.
- Sharing uses recipient public keys so the server does not need plaintext shared data.
- Host key verification stores known fingerprints and rejects unexpected changes.

Important note: optional password sharing transfers the saved SSH password inside the encrypted share payload. It does not transfer private key file contents; private key authentication still depends on the recipient having access to the referenced key path or updating the accepted connection.

Security vulnerabilities should not be reported publicly. Please read [SECURITY.md](SECURITY.md).

---

# Screenshots

<p align="center">
  <img src="docs/screenshots/main.png">
</p>

---

# Project Structure

```text
condui/
├── ssh-gui/              # Wails desktop application
│   ├── backend/          # Go services, storage, sessions and integrations
│   ├── frontend/         # React UI
│   └── build/            # Wails build configuration and assets
├── docs/                 # Roadmap and supporting documentation
└── scripts/              # Project scripts
```

The account/sync/sharing backend ([condui-server](https://github.com/mgueregath/condui-server)) is a separate
repository, cloned independently.

---

# Installation

Download the latest release from GitHub Releases.

## Windows

```bash
Condui-windows-x64.exe
```

## macOS (Apple Silicon & Intel)

```bash
Condui-mac.dmg
```

A single universal binary (arm64 + amd64) in one installer.

## Linux

```bash
Condui-linux-x64.AppImage
# or
Condui-linux-x64.deb
# or
Condui-linux-x64.rpm
```

---

# Development

## Requirements

- Go >= 1.25 for the desktop app
- Node.js >= 22
- Wails v3 CLI

## Clone

```bash
git clone git@github.com:mgueregath/condui.git
cd condui
```

The account/sync/sharing backend is a separate repository — see
[Sync Server (local dev)](#sync-server-local-dev) below for how to run it if you want to develop against a
local instance instead of a hosted one.

## Desktop App

Install frontend dependencies:

```bash
cd ssh-gui/frontend
npm install
```

Run the Wails desktop app:

```bash
cd ..
wails3 dev -config ./build/config.yml -port 9245
```

Generate TypeScript bindings after changing exported Go methods:

```bash
wails3 generate bindings -ts
```

Build the desktop app:

```bash
wails3 build -config ./build/config.yml
```

## Build Configuration

`ssh-gui/backend/buildconfig/build.config.yaml` (gitignored; copy from `build.config.example.yaml`, or the
build task does it for you on first run) fixes a few values **at compile time only**:

```yaml
server_url: "https://sync.condui.app"   # default condui-server this build talks to
db_manager_enabled: false               # informational; see below
```

- `server_url` is `go:embed`-ded into the Go binary (`backend/buildconfig`) and inlined into the frontend
  bundle as `VITE_CONDUI_SERVER_URL` at the same time, so both sides agree on one value. Editing the file
  after a binary is built has no effect — it is never read from disk at runtime, only at build time.
- Whether the private database explorer submodule (`ssh-gui/backend/dbexplorer`, not present in the public
  repo) gets compiled in and its UI shown is controlled by the `dbmanager` Go build tag, not this file —
  `db_manager_enabled` here is purely documentation of intent.

## Sync Server (local dev)

The sync backend is a separate repository. Clone and run it independently:

```bash
git clone git@github.com:mgueregath/condui-server.git
cd condui-server
go run .
```

To point the desktop app at your local instance instead of the hosted default, set `server_url` in
`ssh-gui/build.config.yaml` before building (see [Build Configuration](#build-configuration)).

---

# Testing

Run desktop backend tests:

```bash
cd ssh-gui
go test ./...
```

Build the frontend:

```bash
cd ssh-gui/frontend
npm run build
```

Sync server tests live in its own repository:

```bash
cd condui-server  # git@github.com:mgueregath/condui-server.git
go test ./...
```

---

# Roadmap

The current codebase already includes SSH, SFTP, Docker, tunnels, encrypted vault, account sync, sharing, database exploration, VirtualBox controls, a local terminal and a bilingual (EN/ES) interface.

Planned or future areas include:

- Kubernetes workflows.
- Cloud provider integrations.
- Plugin system.
- Themes and deeper customization.
- Team environments and organization-level administration.

---

# License

condui Community Edition is licensed under:

**GNU Affero General Public License v3.0 (AGPL-3.0)**

Commercial licensing may be available separately.

---

# Trademark

The condui name, logo and visual identity are not included under the software license.

You may:

- Fork the source.
- Modify the code.
- Build your own versions.

You may not:

- Use condui branding.
- Redistribute modified versions as official releases.
- Use condui logos commercially.

without permission.

---

<p align="center">
© 2026 Mirko Gueregat
</p>
