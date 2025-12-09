FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application from the new cmd path
RUN CGO_ENABLED=0 GOOS=linux go build -o updockly-backend ./cmd/updockly-api

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache \
    ca-certificates \
    bash \
    tzdata \
    su-exec

RUN addgroup -S updockly && adduser -S updockly -G updockly

COPY --from=builder /app/updockly-backend .

COPY entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 5000

ENTRYPOINT ["entrypoint.sh"]
CMD ["./updockly-backend"]
