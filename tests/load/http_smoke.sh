#!/usr/bin/env bash
# Lightweight HTTP load smoke for PERF notes (OPEN-09). Not a full 10k-torrent bench.
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
CLIENTS="${CLIENTS:-5}"
ROUNDS="${ROUNDS:-20}"
COOKIE_JAR="${COOKIE_JAR:-}"

curl_opts=(-fsS --max-time 5)
if [[ -n "${COOKIE_JAR}" ]]; then
  curl_opts+=(-b "${COOKIE_JAR}")
fi

echo "load smoke: ${CLIENTS} clients × ${ROUNDS} rounds → ${BASE_URL}"

fail=0
for ((r = 1; r <= ROUNDS; r++)); do
  pids=()
  for ((c = 1; c <= CLIENTS; c++)); do
    (
      curl "${curl_opts[@]}" "${BASE_URL}/api/v1/health" >/dev/null
      curl "${curl_opts[@]}" "${BASE_URL}/api/v1/torrents" >/dev/null || curl "${curl_opts[@]}" "${BASE_URL}/api/v1/auth/status" >/dev/null
    ) &
    pids+=($!)
  done
  for pid in "${pids[@]}"; do
    if ! wait "${pid}"; then
      fail=1
    fi
  done
  if [[ "${fail}" -ne 0 ]]; then
    echo "round ${r} failed" >&2
    exit 1
  fi
done

echo "load smoke OK (${CLIENTS}×${ROUNDS})"
