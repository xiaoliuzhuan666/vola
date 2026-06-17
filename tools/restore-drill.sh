#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_ROOT=""
KEEP_TMP=0
DUMP_PATH=""
ZIP_PATH=""
OBJECT_NAME=""
COS_CREDENTIALS="${HOME}/Desktop/vola-cos-backup-credentials.md"
VAULT_MASTER_KEY_VALUE=""
VAULT_MASTER_KEY_FILE=""
VAULT_MASTER_KEY_EXPLICIT=0
VAULT_PROBE_SCOPE=""
RESTORE_MODE="skip"
PORT="0"
PG_PORT="0"
POSTGRES_IMAGE="${POSTGRES_IMAGE:-postgres:16-alpine}"
POSTGRES_PLATFORM="${POSTGRES_PLATFORM:-}"
PG_CONTAINER=""
SERVER_PID=""
TOKEN_ID=""
OWNER_TOKEN=""
API_BASE=""

usage() {
  cat <<'EOF'
Usage:
  tools/restore-drill.sh --zip /path/to/vola-export.zip [options]
  tools/restore-drill.sh --object vola/neudrive-export-YYYY.zip [--credentials /path/to/credentials.md] [options]

Options:
  --dump PATH              Optional Postgres custom dump to restore before app startup.
  --vault-master-key KEY   Optional original VAULT_MASTER_KEY for secret decrypt verification.
  --vault-master-key-file PATH
                           Read original VAULT_MASTER_KEY from a local file.
                           If neither option is set, VAULT_MASTER_KEY env is used when present.
  --vault-probe-scope SCOPE
                           Optional encrypted Vault scope to decrypt from the restored database.
  --mode skip|overwrite    Restore apply mode. Default: skip.
  --port PORT              Local Vola port. Default: first free port.
  --pg-port PORT           Local Postgres port. Default: random high port.
  --postgres-image IMAGE   Postgres image. Default: postgres:16-alpine.
  --postgres-platform OS/ARCH
                           Optional Docker platform, for example linux/arm64.
  --keep-tmp               Keep temporary directory and container after run.
  -h, --help               Show help.

Safety:
  - Never writes to production.
  - Starts an isolated temporary Postgres container.
  - Starts Vola on 127.0.0.1 only.
  - Does not print secret values.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dump)
      DUMP_PATH="$2"
      shift 2
      ;;
    --zip)
      ZIP_PATH="$2"
      shift 2
      ;;
    --object)
      OBJECT_NAME="$2"
      shift 2
      ;;
    --credentials)
      COS_CREDENTIALS="$2"
      shift 2
      ;;
    --vault-master-key)
      VAULT_MASTER_KEY_VALUE="$2"
      VAULT_MASTER_KEY_EXPLICIT=1
      shift 2
      ;;
    --vault-master-key-file)
      VAULT_MASTER_KEY_FILE="$2"
      shift 2
      ;;
    --vault-probe-scope)
      VAULT_PROBE_SCOPE="$2"
      shift 2
      ;;
    --mode)
      RESTORE_MODE="$2"
      shift 2
      ;;
    --port)
      PORT="$2"
      shift 2
      ;;
    --pg-port)
      PG_PORT="$2"
      shift 2
      ;;
    --postgres-image)
      POSTGRES_IMAGE="$2"
      shift 2
      ;;
    --postgres-platform)
      POSTGRES_PLATFORM="$2"
      shift 2
      ;;
    --keep-tmp)
      KEEP_TMP=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

pick_port() {
  python3 - <<'PY'
import socket
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
}

cleanup() {
  set +e
  if [[ -n "$TOKEN_ID" && -n "$OWNER_TOKEN" && -n "$API_BASE" ]]; then
    curl -fsS -X DELETE "$API_BASE/api/tokens/$TOKEN_ID" -H "Authorization: Bearer $OWNER_TOKEN" >/dev/null 2>&1 || true
  fi
  if [[ -n "$SERVER_PID" ]]; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  if [[ "$KEEP_TMP" -eq 0 && -n "$PG_CONTAINER" ]]; then
    docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
  fi
  if [[ "$KEEP_TMP" -eq 0 && -n "$TMP_ROOT" ]]; then
    rm -rf "$TMP_ROOT"
  elif [[ -n "$TMP_ROOT" ]]; then
    echo "Temporary directory preserved at: $TMP_ROOT"
    [[ -n "$PG_CONTAINER" ]] && echo "Postgres container preserved: $PG_CONTAINER"
  fi
}
trap cleanup EXIT

case "$RESTORE_MODE" in
  skip|overwrite) ;;
  *)
    echo "--mode must be skip or overwrite" >&2
    exit 2
    ;;
esac

need_cmd docker
need_cmd curl
need_cmd python3
need_cmd go
need_cmd psql
need_cmd pg_restore

TMP_ROOT="$(mktemp -d /private/tmp/vola-restore-drill-XXXXXX)"
if [[ "$PORT" == "0" ]]; then
  PORT="$(pick_port)"
fi
if [[ "$PG_PORT" == "0" ]]; then
  PG_PORT="$(pick_port)"
fi
PG_CONTAINER="vola-restore-drill-${PG_PORT}"
API_BASE="http://127.0.0.1:${PORT}"

if [[ -n "$OBJECT_NAME" ]]; then
  ZIP_PATH="$TMP_ROOT/cos-export.zip"
  python3 "$ROOT_DIR/tools/download-s3-backup.py" \
    --credentials "$COS_CREDENTIALS" \
    --object "$OBJECT_NAME" \
    --output "$ZIP_PATH"
fi

if [[ -z "$ZIP_PATH" || ! -f "$ZIP_PATH" ]]; then
  echo "restore zip is required; pass --zip or --object" >&2
  exit 2
fi

echo "Starting temporary Postgres on 127.0.0.1:${PG_PORT} using image ${POSTGRES_IMAGE}"
DOCKER_RUN_ARGS=(run -d --name "$PG_CONTAINER")
if [[ -n "$POSTGRES_PLATFORM" ]]; then
  DOCKER_RUN_ARGS+=(--platform "$POSTGRES_PLATFORM")
fi
docker "${DOCKER_RUN_ARGS[@]}" \
  -e POSTGRES_USER=vola \
  -e POSTGRES_PASSWORD=vola_restore \
  -e POSTGRES_DB=vola_restore \
  -p "127.0.0.1:${PG_PORT}:5432" \
  "$POSTGRES_IMAGE" >/dev/null

for _ in $(seq 1 60); do
  if docker exec "$PG_CONTAINER" pg_isready -U vola -d vola_restore >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$PG_CONTAINER" pg_isready -U vola -d vola_restore >/dev/null

DATABASE_URL="postgres://vola:vola_restore@127.0.0.1:${PG_PORT}/vola_restore?sslmode=disable"
if [[ -n "$DUMP_PATH" ]]; then
  if [[ ! -f "$DUMP_PATH" ]]; then
    echo "dump file not found: $DUMP_PATH" >&2
    exit 2
  fi
  echo "Restoring Postgres dump into temporary database"
  pg_restore --clean --if-exists --no-owner --file - "$DUMP_PATH" \
    | sed '/^SET transaction_timeout = /d' \
    | psql "$DATABASE_URL" >/dev/null
fi

JWT_SECRET="restore-drill-jwt-$(date +%s)-${PG_PORT}"
if [[ -z "$VAULT_MASTER_KEY_VALUE" && -n "$VAULT_MASTER_KEY_FILE" ]]; then
  if [[ ! -f "$VAULT_MASTER_KEY_FILE" ]]; then
    echo "vault master key file not found: $VAULT_MASTER_KEY_FILE" >&2
    exit 2
  fi
  VAULT_MASTER_KEY_VALUE="$(tr -d '\r\n' <"$VAULT_MASTER_KEY_FILE")"
  VAULT_MASTER_KEY_EXPLICIT=1
fi
if [[ -z "$VAULT_MASTER_KEY_VALUE" && -n "${VAULT_MASTER_KEY:-}" ]]; then
  VAULT_MASTER_KEY_VALUE="$VAULT_MASTER_KEY"
  VAULT_MASTER_KEY_EXPLICIT=1
fi
if [[ -z "$VAULT_MASTER_KEY_VALUE" ]]; then
  VAULT_MASTER_KEY_VALUE="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
fi

echo "Building local Vola binary"
GOCACHE="${GOCACHE:-/private/tmp/vola-go-cache}" go build -o "$TMP_ROOT/vola" ./cmd/vola

echo "Starting temporary Vola on ${API_BASE}"
VAULT_MASTER_KEY="$VAULT_MASTER_KEY_VALUE" "$TMP_ROOT/vola" server \
  --listen "127.0.0.1:${PORT}" \
  --storage postgres \
  --database-url "$DATABASE_URL" \
  --jwt-secret "$JWT_SECRET" \
  --public-base-url "$API_BASE" \
  --local-mode >"$TMP_ROOT/server.log" 2>&1 &
SERVER_PID="$!"

for _ in $(seq 1 80); do
  if curl -fsS "$API_BASE/api/health" >"$TMP_ROOT/health.json" 2>/dev/null; then
    break
  fi
  if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    echo "temporary Vola exited early; log follows" >&2
    sed -n '1,160p' "$TMP_ROOT/server.log" >&2
    exit 1
  fi
  sleep 1
done
curl -fsS "$API_BASE/api/health" >/dev/null

TOKEN_RESPONSE="$TMP_ROOT/owner-token.json"
curl -fsS -X POST "$API_BASE/api/local/owner-token" >"$TOKEN_RESPONSE"
OWNER_TOKEN="$(python3 - "$TOKEN_RESPONSE" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)
print(payload["data"]["token"])
PY
)"
TOKEN_ID="$(python3 - "$TOKEN_RESPONSE" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)
print(payload["data"]["scoped_token"]["id"])
PY
)"

echo "Previewing restore zip"
curl -fsS "$API_BASE/api/backup/restore/preview" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -F "file=@${ZIP_PATH}" >"$TMP_ROOT/preview.json"
python3 - "$TMP_ROOT/preview.json" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)["data"]
cats = ",".join(f'{c["id"]}:{c["files"]}' for c in payload.get("categories", []))
print(f'preview recognized={payload.get("recognized")} total_files={payload.get("total_files")} total_bytes={payload.get("total_bytes")} categories={cats}')
if not payload.get("recognized"):
    raise SystemExit("restore preview did not recognize Vola export")
PY

echo "Applying restore zip with mode=${RESTORE_MODE}"
curl -fsS "$API_BASE/api/backup/restore/apply" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -F "file=@${ZIP_PATH}" \
  -F "mode=${RESTORE_MODE}" >"$TMP_ROOT/apply.json"
python3 - "$TMP_ROOT/apply.json" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)["data"]
print(
    "apply "
    f'recognized={payload.get("recognized")} '
    f'mode={payload.get("mode")} '
    f'applied={payload.get("applied")} '
    f'skipped={payload.get("skipped")} '
    f'overwritten={payload.get("overwritten")} '
    f'errors={len(payload.get("errors") or [])}'
)
if not payload.get("recognized") or payload.get("errors"):
    raise SystemExit("restore apply failed")
PY

TREE_COUNT="$(docker exec "$PG_CONTAINER" psql -U vola -d vola_restore -Atc "SELECT COUNT(*) FROM file_tree WHERE deleted_at IS NULL;" | tr -d '\r')"
echo "temporary file_tree live rows=${TREE_COUNT}"

if [[ -n "$DUMP_PATH" && "$VAULT_MASTER_KEY_EXPLICIT" -eq 1 ]]; then
  echo "Vault metadata probe: /api/vault/scopes"
  curl -fsS "$API_BASE/api/vault/scopes" -H "Authorization: Bearer $OWNER_TOKEN" >"$TMP_ROOT/vault-scopes.json"
  python3 - "$TMP_ROOT/vault-scopes.json" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)["data"]
scopes = payload.get("scopes", [])
print(f'vault scopes count={len(scopes)}')
PY
  VAULT_PROBE_ARGS=(--database-url "$DATABASE_URL")
  if [[ -n "$VAULT_PROBE_SCOPE" ]]; then
    VAULT_PROBE_ARGS+=(--scope "$VAULT_PROBE_SCOPE")
  fi
  VAULT_MASTER_KEY="$VAULT_MASTER_KEY_VALUE" go run "$ROOT_DIR/tools/probe-vault-decrypt.go" "${VAULT_PROBE_ARGS[@]}" >"$TMP_ROOT/vault-probe.json"
  python3 - "$TMP_ROOT/vault-probe.json" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)
print(f'vault decrypt ok scope={payload.get("scope")} plaintext_bytes={payload.get("plaintext_bytes")}')
if not payload.get("plaintext_bytes"):
    raise SystemExit("vault decrypt returned empty plaintext")
PY
fi
if [[ -n "$DUMP_PATH" && "$VAULT_MASTER_KEY_EXPLICIT" -eq 0 ]]; then
  echo "vault decrypt skipped: original VAULT_MASTER_KEY was not provided"
fi

echo "Restore drill passed. api_base=${API_BASE} zip_bytes=$(wc -c <"$ZIP_PATH")"
