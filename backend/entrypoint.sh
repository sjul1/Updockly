#!/bin/sh
set -e

# Ensure the app directory is writable by the user (for .env generation)
chown -R updockly:updockly /app

# Ensure certs directory is writable by updockly user
if [ -d "/etc/updockly/certs" ]; then
  chown -R updockly:updockly /etc/updockly/certs
  if [ -f "/etc/updockly/certs/server.key" ]; then
    chmod 644 /etc/updockly/certs/server.key
  fi
fi

# Grant the updockly user access to the Docker socket when it is mounted
SOCK_PATH="/var/run/docker.sock"
if [ -S "$SOCK_PATH" ]; then
  SOCK_GID="$(stat -c '%g' "$SOCK_PATH")"
  GROUP_NAME="$(getent group "$SOCK_GID" | cut -d: -f1)"

  if [ -z "$GROUP_NAME" ]; then
    GROUP_NAME="dockerhost"
    addgroup -g "$SOCK_GID" "$GROUP_NAME" >/dev/null 2>&1 || true
    GROUP_NAME="$(getent group "$SOCK_GID" | cut -d: -f1)"
  fi

  if [ -n "$GROUP_NAME" ]; then
    addgroup updockly "$GROUP_NAME" >/dev/null 2>&1 || true
  fi
fi

TARGET_GROUP="updockly"
if [ -n "$GROUP_NAME" ]; then
  TARGET_GROUP="$GROUP_NAME"
fi

# Execute the main command as the 'updockly' user
# Respect TIMEZONE inside the container
if [ -n "$TIMEZONE" ] && [ -f "/usr/share/zoneinfo/$TIMEZONE" ]; then
  echo "$TIMEZONE" > /etc/timezone
  ln -sf "/usr/share/zoneinfo/$TIMEZONE" /etc/localtime
fi

exec su-exec updockly:"$TARGET_GROUP" "$@"
