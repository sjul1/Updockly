# Build Stage
FROM node:alpine AS builder

WORKDIR /app

# Install dependencies
COPY package.json package-lock.json ./
RUN npm ci

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Runtime Stage
FROM nginx:alpine

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy Nginx configuration
COPY nginx.conf /etc/nginx/templates/default.conf.template

# Use non-root user
# We need to change ownership of directories nginx needs to write to
# /var/cache/nginx, /var/log/nginx, /etc/nginx/conf.d
RUN chown -R nginx:nginx /var/cache/nginx /var/log/nginx /etc/nginx/conf.d /usr/share/nginx/html && \
    touch /var/run/nginx.pid && \
    chown nginx:nginx /var/run/nginx.pid

USER nginx

# Expose port
EXPOSE 8080 8443

ENV SERVER_ADDR=:5000
