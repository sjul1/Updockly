<p align="center">
  <img src="docs/assets/logo.png" alt="Updockly logo" width="120">
</p>

<h1 align="center">Updockly</h1>

<p align="center">
  Self-hosted Docker container management with a modern UI, multi-host agents, and scheduled auto-updates.
</p>

<p align="center">
  <a href="./LICENSE"><img alt="License: GPLv3" src="https://img.shields.io/badge/License-GPLv3-blue.svg"></a>
  <a href="https://github.com/sjul1/Updockly/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/sjul1/Updockly/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="Go 1.25+" src="https://img.shields.io/badge/Go-1.25+-00ADD8.svg?logo=go&logoColor=white">
  <img alt="Vue 3" src="https://img.shields.io/badge/Vue-3-42b883.svg?logo=vuedotjs&logoColor=white">
  <img alt="Docker supported" src="https://img.shields.io/badge/Docker-Supported-2496ED.svg?logo=docker&logoColor=white">
  <a href="https://hub.docker.com/r/sjul/updockly"><img alt="Docker pulls (frontend)" src="https://img.shields.io/docker/pulls/sjul/updockly"></a>
</p>

<p align="center">
  <a href="https://github.com/sjul1/Updockly/wiki/1.-Setup">Quick Start</a> •
  <a href="https://github.com/sjul1/Updockly/wiki">Documentation</a> •
  <a href="https://github.com/sjul1/Updockly/wiki/2.-Agent-Deployment">Agent Deployment</a> •
  <a href="https://github.com/sjul1/Updockly/issues">Issues</a>
</p>

<p align="center">
  <img src="docs/screenshots/dashboard.png" alt="Updockly Dashboard" width="80%">
</p>

---

## ✨ Key Features

- 🛡️ **Security & Auth**: JWT sessions, optional 2FA (TOTP), and OIDC SSO.
- 🐳 **Multi-host**: Manage local Docker + remote Docker hosts via agents.
- 🔄 **Auto-updates**: Scheduled image pulls, safe recreation, and rollback.
- 📈 **Monitoring**: Real-time container status, logs, and history.
- ⚙️ **Self-hosted**: `.env` configuration, runtime settings in DB, and optional TLS for agents.

<p align="center">
  <img src="docs/screenshots/containers.png" alt="Containers Screenshot" width="45%">
  <img src="docs/screenshots/agents.png" alt="Agents Screenshot" width="45%">
</p>

---

## 🚀 Quick Start (Docker Compose)

**Prerequisites**: Docker + Docker Compose.

1. Copy the example env file:

   ```bash
   cp .env.example .env
   ```

2. Start Updockly:

   ```bash
   docker compose up -d
   ```

3. Open the UI:

   - `http://localhost:5174` (HTTP)
   - `https://localhost:5175` (HTTPS, self-signed)

4. Complete the setup wizard to create the admin account.

👉 Full guide: <a href="https://github.com/sjul1/Updockly/wiki/1.-Setup">Quick Start</a>

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
- **Remote Agents**: Lightweight Go agent with TLS communication.
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

Example env file: `.env.example`

<details>
<summary><strong>Environment variables</strong></summary>

### Core Settings

| Variable                   | Description                                                                                | Default Value                                           |
| :------------------------- | :----------------------------------------------------------------------------------------- | :------------------------------------------------------ |
| `DATABASE_URL`             | Connection string for the database (PostgreSQL or SQLite).                                 | `postgres://updockly:updockly@postgres:5432/updocklydb` |
| `JWT_SECRET`               | **Important.** Primary JWT signing key (generated on first boot if missing).               | (auto-generated)                                        |
| `VAULT_KEY`                | **Important.** Vault encryption key for 2FA secrets (generated on first boot if missing).  | (auto-generated)                                        |
| `CLIENT_ORIGIN`            | The URL where the frontend is accessible. Used for CORS and redirects.                     | `http://localhost:5174`                                 |
| `SERVER_ADDR`              | The address and port the backend server listens on.                                        | `:5000`                                                 |
| `TIMEZONE`                 | Timezone used for scheduling and logging (e.g., `Europe/Paris`).                           | `UTC`                                                   |
| `AUTO_PRUNE_IMAGES`        | Automatically clean unused Docker images after updates (`true`/`false`).                   | `false`                                                 |
| `SERVER_SAN_IPS`           | Comma-separated list of IP addresses to add to the self-signed certificate.                | `127.0.0.1,0.0.0.0`                                     |
| `SERVER_SAN_DOMAINS`       | Comma-separated list of domains to add to the self-signed certificate.                     | `localhost,backend,updockly-backend`                    |
| `HIDE_SUPPORT_BUTTON`      | Hide the “Support the project” sidebar CTA in the UI (`true`/`false`).                     | `false`                                                 |
| `AGENT_REQUIRE_IP_BINDING` | Require agent tokens to bind to the first IP seen; rejects tokens without IP on first use. | `false`                                                 |

### Single Sign-On (SSO)

| Variable            | Description                                                                                       | Default Value |
| :------------------ | :------------------------------------------------------------------------------------------------ | :------------ |
| `SSO_ENABLED`       | Enable or disable SSO (`true`/`false`).                                                           | `false`       |
| `SSO_PROVIDER`      | The OIDC provider type (e.g., `authentik`, `keycloak`).                                           | (Empty)       |
| `SSO_ISSUER_URL`    | The OIDC Issuer URL (e.g., `https://auth.example.com/application/o/updockly/`).                   | (Empty)       |
| `SSO_CLIENT_ID`     | The Client ID provided by your IdP.                                                               | (Empty)       |
| `SSO_CLIENT_SECRET` | The Client Secret provided by your IdP.                                                           | (Empty)       |
| `SSO_REDIRECT_URL`  | The callback URL registered in your IdP. Should match `CLIENT_ORIGIN` + `/api/auth/sso/callback`. | (Empty)       |

### Notifications

| Variable                       | Description                                              | Default Value |
| :----------------------------- | :------------------------------------------------------- | :------------ |
| `NOTIFICATION_WEBHOOK_URL`     | Generic webhook URL for notifications.                   | (Empty)       |
| `NOTIFICATION_DISCORD_TOKEN`   | Discord Bot Token.                                       | (Empty)       |
| `NOTIFICATION_DISCORD_CHANNEL` | Discord Channel ID.                                      | (Empty)       |
| `NOTIFICATION_ON_SUCCESS`      | Send notification on successful update (`true`/`false`). | `false`       |
| `NOTIFICATION_ON_FAILURE`      | Send notification on failed update (`true`/`false`).     | `false`       |
| `NOTIFICATION_RECAP_TIME`      | Time for daily recap (HH:MM).                            | (Empty)       |
| `NOTIFICATION_CRON`            | Cron expression for recap schedule.                      | (Empty)       |

### SMTP

| Variable        | Description                                      | Default Value |
| :-------------- | :----------------------------------------------- | :------------ |
| `SMTP_ENABLED`  | Enable or disable SMTP (`true`/`false`).         | `false`       |
| `SMTP_HOST`     | The SMTP server hostname.                        | (Empty)       |
| `SMTP_PORT`     | The SMTP server port.                            | `587`         |
| `SMTP_USER`     | The username for SMTP authentication.            | (Empty)       |
| `SMTP_PASSWORD` | The password for SMTP authentication.            | (Empty)       |
| `SMTP_FROM`     | The sender email address.                        | (Empty)       |
| `SMTP_TLS`      | Enable or disable TLS for SMTP (`true`/`false`). | `false`       |

### File Secrets (Docker Secrets)

Most variables support appending `_FILE` to the name to read the value from a file (e.g., `SECRET_KEY_FILE=/run/secrets/my_secret_key`). This is useful for Docker Swarm or Kubernetes secrets.

</details>

---

## 🛰️ Agents Setup

Agents let you manage containers on remote Docker hosts from a single Updockly UI.

👉 Guide: <a href="https://github.com/sjul1/Updockly/wiki/2.-Agent-Deployment">Agents Deployment</a>

---

## 🧑‍💻 Development

### Dev stack (Docker)

```bash
docker compose -f docker-compose-dev.yml up --build
```

Dev UI ports:

- `http://localhost:5554`
- `https://localhost:5555`

---

## 🗂 Project Structure

    backend/          → Go backend (Gin, GORM)
    frontend/         → Vue 3 SPA (TypeScript, Tailwind)
    updockly-agent/   → Lightweight Go agent
    docker-compose.yml → Full stack orchestration

### Tests

```bash
cd backend && go test ./...
cd ../frontend && npm ci && npm test
```

---

## 🧩 Troubleshooting

### Permission Denied for Certificates

**Fix**: Restart the updockly-backend:

```bash
docker compose restart updockly-backend
```

### Agent TLS Verification Failed

Replace agent `ca.crt` with the one downloaded from the UI.

### Certificate SAN Mismatch

Set:

    SERVER_SAN_IPS=<HOST_IP>

Delete certs volume → restart updockly-backend.

### Frontend 502 Bad Gateway

Backend may still be booting. Check logs:

```bash
docker compose logs -f updockly-backend
```

---

## 🗺️ Roadmap

- [x] Publish Docker image
- [ ] Implement user roles and permissions for granular access control.
- [x] Write Wiki documentation
- [ ] Add support for more container orchestration platforms (e.g., Kubernetes).
- [ ] Develop a more comprehensive notification system with customizable alerts.
- [ ] Integrate with cloud providers for easier agent deployment and management.

---

## 🤝 Contributing

- Issues and PRs are welcome. If you’re unsure where to start, open a discussion/issue with your use case and environment details.
- Please include logs (`docker compose logs`) and your deployment type (single host / agent / SSO) when reporting bugs.

---

## ☕ Support

If Updockly helps you, consider supporting development:

<a href="https://www.buymeacoffee.com/joul" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 60px !important;width: 217px !important;" ></a>

---

## 📜 License

This project **Updockly** is licensed under the **GNU General Public License v3.0**.
See the [`LICENSE`](./LICENSE) file for full details.
