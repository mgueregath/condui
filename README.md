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
![Wails](https://img.shields.io/badge/desktop-Wails-red)
![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-green)

</p>


---

# What is condui?

**condui** is a cross-platform desktop workspace designed to simplify the way developers, DevOps engineers and infrastructure teams interact with remote environments.

Instead of switching between terminals, file transfer clients, Docker dashboards and tunnel managers, **condui** brings everything together into a single unified experience.


condui is designed as a modern alternative to:

- MobaXterm
- Termius
- SecureCRT
- traditional SSH managers


Built with:

- Go
- React
- Wails


---

# Features


## ⚡ SSH Workspace

A modern environment for managing remote SSH sessions.

Features:

- Persistent terminal sessions
- Multiple servers
- Fast switching
- Secure authentication
- Clean desktop experience


---

## 📁 Remote File Explorer

Integrated SFTP file management.

Manage remote systems without external tools.

Features:

- Browse remote directories
- Upload files
- Download files
- Remote file editing


---

## 🐳 Docker Management

Control Docker environments directly from condui.

Features:

- List containers
- Inspect containers
- Start / stop services
- Monitor status


---

## 🔀 SSH Tunnels

Create secure tunnels easily.

Useful for:

- Databases
- Internal dashboards
- Private services
- Development environments


---

# Screenshots


<p align="center">

<img src="docs/screenshots/main.png">

</p>


---

# Why condui?


Modern infrastructure is everywhere:

- Cloud servers
- Private servers
- Edge devices
- Containers
- Internal networks
- VPN environments


Developers usually need multiple tools:

- terminal application
- FTP/SFTP client
- Docker UI
- tunnel manager


condui replaces this fragmented workflow with a single workspace.


---

# Installation


Download the latest release from GitHub Releases.


## Windows

```bash
Condui-windows-x64.exe
```


## macOS Apple Silicon

```bash
Condui-mac-arm64.dmg
```


## macOS Intel

```bash
Condui-mac-intel.dmg
```


## Linux

```bash
Condui-linux-x64.AppImage
```


---

# Development


Requirements:


- Go >= 1.23
- Node.js >= 22
- Wails v2


Clone:

```bash
git clone git@github.com:mgueregath/condui.git

cd condui
```


Install dependencies:

```bash
cd ssh-gui/frontend

npm install
```


Run development mode:


```bash
wails dev
```


Build:


```bash
wails build
```


---

# Architecture


```
condui/

├── ssh-gui/
│
├── backend/
│   └── Go services
│
├── frontend/
│   └── React interface
│
├── docs/
│
└── scripts/
```


---

# Roadmap


Current:

- SSH client
- SFTP explorer
- Docker integration
- SSH tunnels


Planned:

- 🔐 Encrypted credential vault
- ☸️ Kubernetes support
- ☁️ Cloud provider integrations
- 🧩 Plugin system
- 🎨 Themes
- 👥 Team environments


---

# Security


condui manages access to remote infrastructure.

Security vulnerabilities should **not** be reported publicly.

Please read:

SECURITY.md


---

# License


condui Community Edition is licensed under:

**GNU Affero General Public License v3.0 (AGPL-3.0)**


Commercial licensing may be available separately.


---

# Trademark


The condui name, logo and visual identity are not included under the software license.

You may:

- Fork the source
- Modify the code
- Build your own versions


You may not:

- Use condui branding
- Redistribute modified versions as official releases
- Use condui logos commercially

without permission.


---

<p align="center">

© 2026 Mirko Gueregat

</p>