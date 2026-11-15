#!/usr/bin/env bash
# Black-box smoke against a running Fleet (M6). Does not start the server.
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
COOKIE_JAR="${COOKIE_JAR:-}"

curl_opts=(-fsS)
if [[ -n "${COOKIE_JAR}" ]]; then
  curl_opts+=(-b "${COOKIE_JAR}" -c "${COOKIE_JAR}")
fi

echo "== health =="
curl "${curl_opts[@]}" "${BASE_URL}/api/v1/health" | grep -q '"status"'

echo "== version =="
curl "${curl_opts[@]}" "${BASE_URL}/api/v1/version" | grep -q '"version"'

echo "== auth status =="
curl "${curl_opts[@]}" "${BASE_URL}/api/v1/auth/status" | grep -q '"enabled"'

echo "== capabilities =="
code="$(curl "${curl_opts[@]}" -o /tmp/fleet-caps.json -w '%{http_code}' "${BASE_URL}/api/v1/engine/capabilities" || true)"
if [[ "${code}" != "200" && "${code}" != "401" && "${code}" != "403" ]]; then
  echo "unexpected capabilities status ${code}" >&2
  exit 1
fi

if [[ "${code}" == "200" ]]; then
  echo "== torrents snapshot =="
  curl "${curl_opts[@]}" "${BASE_URL}/api/v1/torrents" | grep -q '"torrents"'
  echo "== updates status =="
  curl "${curl_opts[@]}" "${BASE_URL}/api/v1/updates" | grep -q '"autoUpdate"'
fi

echo "e2e smoke OK against ${BASE_URL}"
