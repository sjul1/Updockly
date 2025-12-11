# Updockly

![GPLv3 License](https://img.shields.io/badge/License-GPLv3-blue.svg) ![Go Badge](https://img.shields.io/badge/Go-1.25+-00ADD8.svg?logo=go&logoColor=white) ![Vue Badge](https://img.shields.io/badge/Vue-3-42b883.svg?logo=vuedotjs&logoColor=white) ![Docker Badge](https://img.shields.io/badge/Docker-Supported-2496ED.svg?logo=docker&logoColor=white)

A robust, self-hosted Docker container management platform featuring a **Go** backend and a **Vue 3** frontend.
**Updockly** provides a unified dashboard to monitor, manage, and auto-update containers across multiple hosts via lightweight agents.

<p align="center">
  <img src="docs/screenshots/dashboard.png" alt="Updockly Dashboard" width="80%">
</p>

This marks my first open-source release on GitHub. I have always learned by myself through private projects, but I am fully committed to refining and perfecting this codebase. I welcome all feedback and recommendations - constructive criticism and advice are highly appreciated!

---

## ✨ Key Features

- 🛡️ **Secure by Design**: Login with JWT, optional 2FA (TOTP), and OIDC SSO.
- 🐳 **Multi-Host Management**: Control containers on the main server and remote hosts via agents.
- 🔄 **Automatic Updates & Rollbacks**: Scheduled image pulls, safe recreation, and one-click rollback.
- 📈 **Live Monitoring**: Real-time container status.
- ⚙️ **Configurable & Self-Hosted**: TLS support, runtime configuration, and `.env`-based settings.

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
- **Live Monitoring**: Real-time status indicators.
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

Updockly is configured using environment variables. You can set these in your `.env` file or directly in your Docker configuration.

- **Web UI Settings**: Database, timezone, certificates, and more.
- **`.env` Config**: Full environment variable control.

**Env-only keys**: `DATABASE_URL`, `JWT_SECRET`, `VAULT_KEY`, `CLIENT_ORIGIN`, `SERVER_ADDR`.
These must be provided via environment/.env and are not editable in the UI.

**Runtime settings**: Everything else (timezone, notifications, SMTP, SSO, UI toggles, agent/runtime flags) is stored in the database.
The UI loads defaults from env on first boot and persists changes to the DB so they survive container recreations.

<p align="center">
  <img src="docs/screenshots/settings.png" alt="Settings Screenshot" width="65%">
</p>

## Core Settings

| Variable                   | Description                                                                                | Default Value                                           |
| :------------------------- | :----------------------------------------------------------------------------------------- | :------------------------------------------------------ |
| `DATABASE_URL`             | Connection string for the database (PostgreSQL or SQLite).                                 | `postgres://updockly:updockly@localhost:5432/updocklydb |
| `JWT_SECRET`               | **Important.** Primary JWT signing key (generated on first boot if missing).               | (auto-generated)                                        |
| `VAULT_KEY`                | **Important.** Vault encryption key for 2FA secrets (generated on first boot if missing).  | (auto-generated)                                        |
| `CLIENT_ORIGIN`            | The URL where the frontend is accessible. Used for CORS and redirects.                     | `http://localhost:8080`                                 |
| `SERVER_ADDR`              | The address and port the backend server listens on.                                        | `:5000`                                                 |
| `TIMEZONE`                 | Timezone used for scheduling and logging (e.g., `Europe/Paris`).                           | `UTC`                                                   |
| `SERVER_SAN_IPS`           | Comma-separated list of IP addresses to add to the self-signed certificate.                | `127.0.0.1,0.0.0.0`                                     |
| `SERVER_SAN_DOMAINS`       | Comma-separated list of domains to add to the self-signed certificate.                     | `localhost,backend,updockly-backend`                    |
| `HIDE_SUPPORT_BUTTON`      | Hide the “Support the project” sidebar CTA in the UI (`true`/`false`).                     | `false`                                                 |
| `AGENT_REQUIRE_IP_BINDING` | Require agent tokens to bind to the first IP seen; rejects tokens without IP on first use. | `false`                                                 |

## Single Sign-On (SSO)

| Variable            | Description                                                                                       | Default Value |
| :------------------ | :------------------------------------------------------------------------------------------------ | :------------ |
| `SSO_ENABLED`       | Enable or disable SSO (`true`/`false`).                                                           | `false`       |
| `SSO_PROVIDER`      | The OIDC provider type (e.g., `authentik`, `keycloak`).                                           | (Empty)       |
| `SSO_ISSUER_URL`    | The OIDC Issuer URL (e.g., `https://auth.example.com/application/o/updockly/`).                   | (Empty)       |
| `SSO_CLIENT_ID`     | The Client ID provided by your IdP.                                                               | (Empty)       |
| `SSO_CLIENT_SECRET` | The Client Secret provided by your IdP.                                                           | (Empty)       |
| `SSO_REDIRECT_URL`  | The callback URL registered in your IdP. Should match `CLIENT_ORIGIN` + `/api/auth/sso/callback`. | (Empty)       |

## Notifications

| Variable                       | Description                                              | Default Value |
| :----------------------------- | :------------------------------------------------------- | :------------ |
| `NOTIFICATION_WEBHOOK_URL`     | Generic webhook URL for notifications.                   | (Empty)       |
| `NOTIFICATION_DISCORD_TOKEN`   | Discord Bot Token.                                       | (Empty)       |
| `NOTIFICATION_DISCORD_CHANNEL` | Discord Channel ID.                                      | (Empty)       |
| `NOTIFICATION_ON_SUCCESS`      | Send notification on successful update (`true`/`false`). | (Empty)       |
| `NOTIFICATION_ON_FAILURE`      | Send notification on failed update (`true`/`false`).     | (Empty)       |
| `NOTIFICATION_RECAP_TIME`      | Time for daily recap (HH:MM).                            | (Empty)       |
| `NOTIFICATION_CRON`            | Cron expression for recap schedule.                      | (Empty)       |

## SMTP

| Variable        | Description                                      | Default Value |
| :-------------- | :----------------------------------------------- | :------------ |
| `SMTP_ENABLED`  | Enable or disable SMTP (`true`/`false`).         | `false`       |
| `SMTP_HOST`     | The SMTP server hostname.                        | (Empty)       |
| `SMTP_PORT`     | The SMTP server port.                            | (Empty)       |
| `SMTP_USER`     | The username for SMTP authentication.            | (Empty)       |
| `SMTP_PASSWORD` | The password for SMTP authentication.            | (Empty)       |
| `SMTP_FROM`     | The sender email address.                        | (Empty)       |
| `SMTP_TLS`      | Enable or disable TLS for SMTP (`true`/`false`). | (Empty)       |

## File Secrets (Docker Secrets)

Most variables support appending `_FILE` to the name to read the value from a file (e.g., `SECRET_KEY_FILE=/run/secrets/my_secret_key`). This is useful for Docker Swarm or Kubernetes secrets.

---

## 🚀 Quick Start

1.  **Follow the setup guide for a quick start**:
    👉 [Setup Guide](https://github.com/sjul1/Updockly/wiki/1.-Setup)

2.  **Access the UI**:

    - **HTTP**: http://localhost:5174\

3.  **Follow the setup wizard** to create the admin account and configure the database.

<p align="center">
  <img src="docs/screenshots/admin-creation.png" alt="Admin Creation Screenshot" width="45%">
</p>

---

## 🛰️ Agents Setup

👉 [Agent Deployment Guide](https://github.com/sjul1/Updockly/wiki/2.-Agent-Deployment)

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
docker compose restart updockly-backend
```

### Agent TLS Verification Failed

Replace agent `ca.crt` with the one downloaded from the UI.

### Certificate SAN Mismatch

Set:

    SERVER_SAN_IPS=<HOST_IP>

Delete certs volume → restart backend.

### Frontend 502 Bad Gateway

Backend may still be booting. Check logs:

```bash
docker compose logs -f updockly-backend
```

---

## 📅 Upcoming Tasks

- [x] Publish Docker image
- [ ] Implement user roles and permissions for granular access control.
- [x] Write Wiki documentation
- [ ] Add support for more container orchestration platforms (e.g., Kubernetes).
- [ ] Develop a more comprehensive notification system with customizable alerts.
- [ ] Integrate with cloud providers for easier agent deployment and management.

---

Your support is huge for me, thank you!

<a href="https://www.buymeacoffee.com/joul" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 60px !important;width: 217px !important;" ></a>

---

## 📜 License

This project **Updockly** is licensed under the **GNU General Public License v3.0**.
See the [`LICENSE`](./LICENSE) file for full details.
