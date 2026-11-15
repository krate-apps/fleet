#!/usr/bin/env bash
# Lightweight reconnect soak helper for PERF-005 notes (not a full 24h cert by itself).
# Restarts rtorrent@USER periodically and checks Fleet health/capabilities.
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
UNIT="${UNIT:-rtorrent@trevor}"
ROUNDS="${ROUNDS:-24}"
SLEEP_SEC="${SLEEP_SEC:-60}"
# Large sessions (≈10k torrents) may need >60s for rTorrent socket + first sync.
WAIT_SEC="${WAIT_SEC:-180}"
# Avoid systemd StartLimitBurst when restarting rapidly.
COOLDOWN_SEC="${COOLDOWN_SEC:-12}"

echo "soak: ${ROUNDS} rounds, sleep ${SLEEP_SEC}s, wait ${WAIT_SEC}s, cooldown ${COOLDOWN_SEC}s, unit ${UNIT}, fleet ${BASE_URL}"
for ((i = 1; i <= ROUNDS; i++)); do
  echo "== round ${i}/${ROUNDS} $(date -u +%Y-%m-%dT%H:%M:%SZ) =="
  curl -fsS "${BASE_URL}/api/v1/health" >/dev/null
  systemctl reset-failed "${UNIT}" >/dev/null 2>&1 || true
  systemctl stop "${UNIT}" || true
  sleep 2
  if ! systemctl start "${UNIT}"; then
    systemctl reset-failed "${UNIT}" || true
    sleep "${COOLDOWN_SEC}"
    systemctl start "${UNIT}"
  fi
  ok=0
  for s in $(seq 1 "${WAIT_SEC}"); do
    if curl -fsS "${BASE_URL}/api/v1/engine/capabilities" | grep -q '"connected":true'; then
      echo "reconnected after ${s}s"
      ok=1
      break
    fi
    sleep 1
  done
  if [[ "${ok}" -ne 1 ]]; then
    echo "reconnect failed on round ${i}" >&2
    systemctl status "${UNIT}" --no-pager >&2 || true
    exit 1
  fi
  count="$(curl -fsS "${BASE_URL}/api/v1/torrents" | python3 -c 'import sys,json;print(len(json.load(sys.stdin).get("torrents",[])))')"
  echo "connected torrents=${count}"
  if [[ "${i}" -lt "${ROUNDS}" ]]; then
    sleep "${SLEEP_SEC}"
  fi
done
echo "soak reconnect OK (${ROUNDS} rounds)"
