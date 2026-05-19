#!/usr/bin/env bash

set -euo pipefail

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

PLATFORM="${PLATFORM:-linux/amd64}"
BASE_IMAGE_REGISTRY="${BASE_IMAGE_REGISTRY:-$ACR_REGISTRY}"
BASE_IMAGE_NAMESPACE="${BASE_IMAGE_NAMESPACE:-$ACR_NAMESPACE}"
BASE_IMAGE_REPOSITORY="${BASE_IMAGE_REPOSITORY:-$ACR_REPOSITORY}"

if [[ -n "${DOCKERHUB_USERNAME:-}" && -n "${DOCKERHUB_TOKEN:-}" ]]; then
  echo "$DOCKERHUB_TOKEN" | docker login docker.io --username "$DOCKERHUB_USERNAME" --password-stdin
fi

echo "$ACR_PASSWORD" | docker login "$ACR_REGISTRY" --username "$ACR_USERNAME" --password-stdin

mirror_image() {
  local source="$1"
  local target_tag="$2"
  local target="${BASE_IMAGE_REGISTRY}/${BASE_IMAGE_NAMESPACE}/${BASE_IMAGE_REPOSITORY}:${target_tag}"

  printf 'Mirroring %s (%s) -> %s\n' "$source" "$PLATFORM" "$target"
  docker pull --platform "$PLATFORM" "$source"
  docker tag "$source" "$target"
  docker push "$target"
}

mirror_image "node:20-alpine" "base-node-20-alpine"
mirror_image "golang:1.25-alpine" "base-golang-1.25-alpine"
mirror_image "alpine:3.19" "base-alpine-3.19"

printf 'Base images are ready in %s/%s/%s\n' \
  "$BASE_IMAGE_REGISTRY" \
  "$BASE_IMAGE_NAMESPACE" \
  "$BASE_IMAGE_REPOSITORY"
