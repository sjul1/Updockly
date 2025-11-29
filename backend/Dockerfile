# Build Stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o updockly-backend main.go

# Runtime Stage
FROM alpine:latest

WORKDIR /app

# Install backup tools and docker cli
# Note: postgresql-client provides pg_dump
# mariadb-client provides mysqldump
# mongodb-tools provides mongodump
# redis provides redis-cli
# docker-cli is needed for "docker" mode connections
RUN apk add --no-cache \
    ca-certificates \
    docker-cli \
    bash \
    tzdata \
    su-exec

# Create a non-root user
RUN addgroup -S updockly && adduser -S updockly -G updockly

# Copy binary from builder
COPY --from=builder /app/updockly-backend .

# Copy entrypoint script
COPY entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh

# Expose port
EXPOSE 5000

# Use entrypoint to handle permissions dropping
ENTRYPOINT ["entrypoint.sh"]

# Run the application
CMD ["./updockly-backend"]
