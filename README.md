# Updockly

![GPLv3 License](https://img.shields.io/badge/License-GPLv3-blue.svg) ![Go Badge](https://img.shields.io/badge/Go-1.22+-00ADD8.svg?logo=go&logoColor=white) ![Vue Badge](https://img.shields.io/badge/Vue-3-42b883.svg?logo=vuedotjs&logoColor=white) ![Docker Badge](https://img.shields.io/badge/Docker-Supported-2496ED.svg?logo=docker&logoColor=white)

A robust, self-hosted Docker container management platform featuring a **Go** backend and a **Vue 3** frontend.
**Updockly** provides a unified dashboard to monitor, manage, and auto-update containers across multiple hosts via lightweight agents.

<p align="center">
  <img src="docs/screenshots/dashboard.png" alt="Updockly Dashboard" width="80%">
</p>

This marks my first open-source release on GitHub. I have always learned by myself through private projects, but I am fully committed to refining and perfecting this codebase. I welcome all feedback and recommendations - constructive criticism and advice are highly appreciated!

---

## ✨ Key Features

- 🛡️ **Secure by Design** -- Login with JWT, optional 2FA (TOTP), and OIDC SSO.
- 🐳 **Multi-Host Management** -- Control containers on the main server and remote hosts via agents.
- 🔄 **Automatic Updates & Rollbacks** -- Scheduled image pulls, safe recreation, and one-click rollback.
- 📈 **Live Monitoring** -- Real-time container status, logs, and action history.
- ⚙️ **Configurable & Self-Hosted** -- TLS support, runtime configuration, and `.env`-based settings.

<p align="center">
  <img src="docs/screenshots/containers.png" alt="Containers Screenshot" width="45%">
  <img src="docs/screenshots/agents.png" alt="Agents Screenshot" width="45%">
</p>

---

## 🛡️ Security & Authentication

- **Secure Access**: Built-in authentication using JWT-based sessions.
- **2FA (TOTP)**: Google Authenticator, Authy, Aegis, and more.
- **SSO (OIDC)**: Login with enterprise identity providers.
- **HTTPS/TLS**: Automatic self-signed certificate generation with SAN/IP support.
- **Non-Root Containers**: Both backend and frontend run as restricted users.

<p align="center">
  <img src="docs/screenshots/login.png" alt="Login Screenshot" width="45%">
</p>

---

## 🐳 Container Management

- **Multi-Host Support**: Control local and remote Docker hosts.
- **Remote Agents**: Lightweight Go agent with encrypted TLS communication.
- **Live Monitoring**: Real-time CPU, memory, and status indicators.
- **Container Actions**: Start, stop, restart, logs, history.

---

## 🔄 Auto-Updates & Rollbacks

- **Automatic Image Updates**: Scheduled pull + recreate.
- **Rollback Support**: Restore previous image versions if an update fails.
- **Webhooks**: Notify Discord or custom endpoints.

<p align="center">
  <img src="docs/screenshots/history.png" alt="History Screenshot" width="65%">
</p>

---

## ⚙️ Configuration

- **Web UI Settings**: Database, timezone, certificates, and more.
- **`.env` Config**: Full environment variable control.

<p align="center">
  <img src="docs/screenshots/settings.png" alt="Settings Screenshot" width="65%">
</p>

---

## 🚀 Quick Start

1.  **Clone the repository**:

    ```bash
    git clone https://github.com/your-username/updockly.git
    cd updockly
    ```

2.  **Copy environment file**:

    ```bash
    cp .env.example .env
    ```

3.  **Start the full stack**:

    ```bash
    docker compose up -d --build
    ```

4.  **Access the UI**:

    - **HTTP**: http://localhost:5174\

5.  **Follow the setup wizard** to create the admin account and configure the database.

<p align="center">
  <img src="docs/screenshots/admin-creation.png" alt="Admin Creation Screenshot" width="45%">
</p>

---

## 🛰️ Agents Setup

1.  Go to **Agents** in the dashboard.
2.  Click **Create Agent** → copy install script.
3.  Run script on the remote host.
4.  (TLS Mode) Download `ca.crt`, place it next to the agent binary, restart it.

---

## 🗂 Project Structure

    backend/          → Go backend (Gin, GORM)
    frontend/         → Vue 3 SPA (TypeScript, Tailwind)
    updockly-agent/   → Lightweight Go agent
    docker-compose.yml → Full stack orchestration

---

## 🧩 Troubleshooting

### Permission Denied for Certificates

**Fix**: Restart the backend:

```bash
docker compose restart backend
```

---

### Agent TLS Verification Failed

Replace agent `ca.crt` with the one downloaded from the UI.

---

### Certificate SAN Mismatch

Set:

    SERVER_SAN_IPS=<HOST_IP>

Delete certs volume → restart backend.

---

### Frontend 502 Bad Gateway

Backend may still be booting. Check logs:

```bash
docker compose logs -f backend
```

---

## 📅 Upcoming Tasks

- [ ] Publish Docker image
- [ ] Implement user roles and permissions for granular access control.
- [ ] Write Wiki documentation
- [ ] Add support for more container orchestration platforms (e.g., Kubernetes).
- [ ] Develop a more comprehensive notification system with customizable alerts.
- [ ] Integrate with cloud providers for easier agent deployment and management.

---

## 📜 License

This project **Updockly** is licensed under the **GNU General Public License v3.0**.
See the [`LICENSE`](./LICENSE) file for full details.
