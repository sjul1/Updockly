#!/bin/sh

# Check if certificates exist
if [ -f "/etc/updockly/certs/server.crt" ] && [ -f "/etc/updockly/certs/server.key" ]; then
    echo "Certificates found, enabling SSL..."
else
    echo "Certificates not found, disabling SSL..."
    # Remove the SSL block from the template
    sed -i '/# SSL_START/,/# SSL_END/d' /etc/nginx/templates/default.conf.template
fi

# Exec the original docker-entrypoint
exec /docker-entrypoint.sh "$@"
