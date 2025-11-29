# Updockly Agent

Lightweight companion service that runs on a remote Docker host and periodically reports basic engine metadata to your Updockly server. It also executes remote commands (Update, Start, Stop, Logs) issued from the Updockly dashboard.

## Features

- **Heartbeat**: Reports Docker version, platform, and running containers status every 30s.
- **Command Execution**: Receives commands from the server to restart, stop, or update containers.
- **Secure Communication**: Supports Token authentication and optional TLS verification with custom CA.

## Building

```bash
cd updockly-agent
go build -o bin/updockly-agent .
```

## Configuration

Configuration is done via environment variables or CLI flags.

| Environment Variable   | Flag        | Description                                                      |
| ---------------------- | ----------- | ---------------------------------------------------------------- |
| `UPDOCKLY_SERVER`      | `-server`   | Base URL of your Updockly server (e.g. `https://10.0.1.50:5175`) |
| `UPDOCKLY_AGENT_TOKEN` | `-token`    | **Required**. Token issued when creating the agent in the UI     |
| `UPDOCKLY_AGENT_NAME`  | `-name`     | Optional hostname override sent to the server                    |
| `UPDOCKLY_INTERVAL`    | `-interval` | Heartbeat interval (default `30s`)                               |
| `UPDOCKLY_CA_CERT`     | `-ca-cert`  | Path to a trusted Root CA certificate (for self-signed servers)  |
| `DOCKER_HOST`          | N/A         | Docker socket override (defaults to unix socket)                 |

## Running

### Standard Run

```bash
UPDOCKLY_SERVER="http://updockly.local:5174" \
UPDOCKLY_AGENT_TOKEN="<token>" \
./bin/updockly-agent
```

### With TLS (Self-Signed)

If your server uses a self-signed certificate:

1.  Download `ca.crt` from the Updockly Dashboard (Agents page).
2.  Place `ca.crt` in the agent directory.
3.  Run:

```bash
UPDOCKLY_SERVER="https://updockly.local:5175" \
UPDOCKLY_AGENT_TOKEN="<token>" \
UPDOCKLY_CA_CERT="ca.crt" \
./bin/updockly-agent
```

## Docker Usage

Build the image:

```bash
docker build -t updockly/agent:latest .
```

Run with Docker socket mounted (Required to manage containers):

```bash
docker run -d --name updockly-agent \
  -e UPDOCKLY_SERVER="http://updockly.local:5174" \
  -e UPDOCKLY_AGENT_TOKEN="<token>" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  updockly/agent:latest
```

### Docker Compose with TLS

```yaml
services:
  agent:
    image: updockly/agent:latest
    environment:
      - UPDOCKLY_SERVER=https://10.0.1.50:5175
      - UPDOCKLY_AGENT_TOKEN="<token>"
      - UPDOCKLY_CA_CERT=/app/ca.crt
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./agent-data/ca.crt:/app/ca.crt:ro # Mount the CA cert
    restart: unless-stopped
```
