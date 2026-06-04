#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCROOT="$ROOT_DIR/web/public"
PORT="${1:-8787}"
PAGE_PATH="${PAGE_PATH:-index.html}"
STAMP="$(date +%s)"
URL="http://127.0.0.1:${PORT}/${PAGE_PATH}?v=${STAMP}"

echo "[vola] docroot: $DOCROOT"
echo "[vola] port:    $PORT"

# Kill any existing server on the port
if command -v lsof >/dev/null 2>&1; then
  lsof -tiTCP:"$PORT" -sTCP:LISTEN 2>/dev/null | xargs -r kill -9 || true
fi

# Start python http.server in background
(
  cd "$DOCROOT"
  nohup python3 -m http.server "$PORT" > /tmp/vola_http_${PORT}.log 2>&1 & echo $! > /tmp/vola_http_${PORT}.pid
) || true

# Wait until reachable
echo "[vola] waiting for http://127.0.0.1:${PORT}/ ..."
for i in {1..60}; do
  if curl -fsS "http://127.0.0.1:${PORT}/" >/dev/null 2>&1; then
    break
  fi
  sleep 0.25
done

echo "[vola] opening: $URL"
if command -v open >/dev/null 2>&1; then
  open "$URL"
elif command -v xdg-open >/dev/null 2>&1; then
  xdg-open "$URL" >/dev/null 2>&1 || true
else
  echo "Open this URL in your browser: $URL"
fi

echo "[vola] tail server log: tail -f /tmp/vola_http_${PORT}.log"
