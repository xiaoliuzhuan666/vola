#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${REPO_ROOT:-$(cd "$SCRIPT_DIR/../.." && pwd)}"

require_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    echo "required environment variable is missing: $key" >&2
    exit 1
  fi
}

require_env ACR_REGISTRY
require_env ACR_NAMESPACE
require_env ACR_REPOSITORY
require_env ACR_USERNAME
require_env ACR_PASSWORD

IMAGE_TAG="${IMAGE_TAG:-}"
if [[ -z "$IMAGE_TAG" ]]; then
  IMAGE_TAG="$(git -C "$REPO_ROOT" rev-parse --short=12 HEAD)"
fi

PLATFORM="${PLATFORM:-linux/amd64}"
IMAGE="${ACR_REGISTRY}/${ACR_NAMESPACE}/${ACR_REPOSITORY}:${IMAGE_TAG}"
LATEST_IMAGE="${ACR_REGISTRY}/${ACR_NAMESPACE}/${ACR_REPOSITORY}:latest"

BASE_IMAGE_REGISTRY="${BASE_IMAGE_REGISTRY:-$ACR_REGISTRY}"
BASE_IMAGE_NAMESPACE="${BASE_IMAGE_NAMESPACE:-$ACR_NAMESPACE}"
BASE_IMAGE_REPOSITORY="${BASE_IMAGE_REPOSITORY:-$ACR_REPOSITORY}"
NODE_BASE_IMAGE="${NODE_BASE_IMAGE:-${BASE_IMAGE_REGISTRY}/${BASE_IMAGE_NAMESPACE}/${BASE_IMAGE_REPOSITORY}:base-node-20-alpine}"
GO_BASE_IMAGE="${GO_BASE_IMAGE:-${BASE_IMAGE_REGISTRY}/${BASE_IMAGE_NAMESPACE}/${BASE_IMAGE_REPOSITORY}:base-golang-1.25-alpine}"
RUNTIME_BASE_IMAGE="${RUNTIME_BASE_IMAGE:-${BASE_IMAGE_REGISTRY}/${BASE_IMAGE_NAMESPACE}/${BASE_IMAGE_REPOSITORY}:base-alpine-3.19}"

echo "$ACR_PASSWORD" | docker login "$ACR_REGISTRY" --username "$ACR_USERNAME" --password-stdin

printf 'Using base images:\n'
printf '  NODE_BASE_IMAGE=%s\n' "$NODE_BASE_IMAGE"
printf '  GO_BASE_IMAGE=%s\n' "$GO_BASE_IMAGE"
printf '  RUNTIME_BASE_IMAGE=%s\n' "$RUNTIME_BASE_IMAGE"

if docker buildx version >/dev/null 2>&1; then
  docker buildx build \
    --platform "$PLATFORM" \
    --provenance=false \
    --build-arg "NODE_BASE_IMAGE=$NODE_BASE_IMAGE" \
    --build-arg "GO_BASE_IMAGE=$GO_BASE_IMAGE" \
    --build-arg "RUNTIME_BASE_IMAGE=$RUNTIME_BASE_IMAGE" \
    --tag "$IMAGE" \
    --tag "$LATEST_IMAGE" \
    --push \
    "$REPO_ROOT"
else
  if [[ "$PLATFORM" != "linux/amd64" ]]; then
    echo "docker buildx is required for PLATFORM=$PLATFORM" >&2
    exit 1
  fi
  docker build \
    --build-arg "NODE_BASE_IMAGE=$NODE_BASE_IMAGE" \
    --build-arg "GO_BASE_IMAGE=$GO_BASE_IMAGE" \
    --build-arg "RUNTIME_BASE_IMAGE=$RUNTIME_BASE_IMAGE" \
    --tag "$IMAGE" \
    --tag "$LATEST_IMAGE" \
    "$REPO_ROOT"
  docker push "$IMAGE"
  docker push "$LATEST_IMAGE"
fi

printf 'NEUDRIVE_IMAGE=%s\n' "$IMAGE"
