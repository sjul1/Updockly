#!/usr/bin/env bash
set -euo pipefail

# Build and push latest Docker images for backend, frontend, and agent.
# Usage: ./scripts/build-and-push.sh [tag]
# Defaults to tag "latest". Requires `docker login` for user "sjul" beforehand.

TAG="${1:-latest}"
USER="sjul"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "Building backend image: ${USER}/updockly-api:${TAG}"
docker build \
  -t "${USER}/updockly-api:${TAG}" \
  -f "${ROOT_DIR}/backend/Dockerfile" \
  "${ROOT_DIR}/backend"

echo "Building frontend image: ${USER}/updockly:${TAG}"
docker build \
  -t "${USER}/updockly:${TAG}" \
  -f "${ROOT_DIR}/frontend/Dockerfile" \
  "${ROOT_DIR}/frontend"

echo "Building agent image: ${USER}/updockly-agent:${TAG}"
docker build \
  -t "${USER}/updockly-agent:${TAG}" \
  -f "${ROOT_DIR}/updockly-agent/Dockerfile" \
  "${ROOT_DIR}/updockly-agent"

echo "Pushing images to Docker Hub..."
docker push "${USER}/updockly-api:${TAG}"
docker push "${USER}/updockly:${TAG}"
docker push "${USER}/updockly-agent:${TAG}"

echo "Done: pushed tag ${TAG} for backend, frontend, and agent."
