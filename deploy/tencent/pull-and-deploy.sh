#!/usr/bin/env bash

set -euo pipefail

APP_DIR="${APP_DIR:-/opt/neudrive}"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-neudrive}"
COMPOSE_FILE="${COMPOSE_FILE:-$APP_DIR/deploy/tencent/docker-compose.yml}"
ENV_FILE="${NEUDRIVE_ENV_FILE:-$APP_DIR/config/neudrive.env}"
HOST_PORT="${NEUDRIVE_HOST_PORT:-18080}"
HEALTHCHECK_URL="${HEALTHCHECK_URL:-http://127.0.0.1:${HOST_PORT}/api/health}"

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "compose file not found: $COMPOSE_FILE" >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "env file not found: $ENV_FILE" >&2
  exit 1
fi

docker compose -p "$COMPOSE_PROJECT" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" pull db server
docker compose -p "$COMPOSE_PROJECT" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d db server
docker compose -p "$COMPOSE_PROJECT" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps
curl -fsS "$HEALTHCHECK_URL"
echo
