# syntax=docker/dockerfile:1.6
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o updockly-agent main.go

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/updockly-agent .

ENV UPDOCKLY_INTERVAL=30s

ENTRYPOINT ["./updockly-agent"]
