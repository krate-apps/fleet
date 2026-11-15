#!/usr/bin/env bash
# Seed N tiny stopped torrents into a running Fleet+rTorrent lab (OPEN-09 fixtures).
# Does not touch existing torrents outside the name prefix.
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
COUNT="${COUNT:-1000}"
START_INDEX="${START_INDEX:-1}"
PREFIX="${PREFIX:-fleet-perf}"
WORKDIR="${WORKDIR:-}"
MKTORRENT="${MKTORRENT:-fleet-mktorrent}"
KEEP="${KEEP:-0}"

usage() {
  cat <<'EOF'
Usage: seed_rtorrent_fixtures.sh

Env:
  BASE_URL      Fleet API base (default http://127.0.0.1:8080)
  COUNT         Number of torrents to add (default 1000)
  START_INDEX   First numeric suffix (default 1); use to append batches
  PREFIX        Name prefix for fixture files (default fleet-perf)
  WORKDIR       Scratch dir (default mktemp under /tmp)
  MKTORRENT     JSON mktorrent CLI (default fleet-mktorrent)
  KEEP          If 1, keep workdir torrents on disk after load
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if ! command -v curl >/dev/null || ! command -v python3 >/dev/null; then
  echo "curl and python3 required" >&2
  exit 2
fi
if ! command -v "${MKTORRENT}" >/dev/null && [[ ! -x "${MKTORRENT}" ]]; then
  echo "mktorrent CLI not found: ${MKTORRENT}" >&2
  exit 2
fi

if [[ -z "${WORKDIR}" ]]; then
  WORKDIR="$(mktemp -d /tmp/fleet-perf-seed-XXXX)"
else
  mkdir -p "${WORKDIR}"
fi
end_index=$((START_INDEX + COUNT - 1))
echo "seeding ${COUNT} torrents (${PREFIX}-${START_INDEX}..${end_index}) into ${BASE_URL} (workdir ${WORKDIR})"

before="$(curl -fsS "${BASE_URL}/api/v1/torrents" | python3 -c 'import sys,json;print(len(json.load(sys.stdin).get("torrents",[])))')"
echo "torrents before: ${before}"

batch=50
loaded=0
for ((i = START_INDEX; i <= end_index; i++)); do
  name=$(printf '%s-%05d.txt' "${PREFIX}" "${i}")
  path="${WORKDIR}/${name}"
  # Unique payload per index (and batch id) so re-runs do not collide on infohash.
  printf 'fleet perf fixture %05d seed=%s\n' "${i}" "${START_INDEX}-${COUNT}" >"${path}"
  tor="${WORKDIR}/${name}.torrent"
  "${MKTORRENT}" --json -a http://127.0.0.1:9/announce -o "${tor}" -l 14 "${path}" >/dev/null
  b64="$(base64 -w0 "${tor}")"
  curl -fsS -X POST "${BASE_URL}/api/v1/torrents" \
    -H 'Content-Type: application/json' \
    -d "{\"torrentBase64\":\"${b64}\",\"start\":false}" >/dev/null
  loaded=$((loaded + 1))
  if ((loaded % batch == 0)) || ((loaded == COUNT)); then
    echo "loaded ${loaded}/${COUNT} (index ${i})"
  fi
done

# Allow sync to catch up (large sessions can take a while).
for _ in $(seq 1 120); do
  after="$(curl -fsS "${BASE_URL}/api/v1/torrents" | python3 -c 'import sys,json;print(len(json.load(sys.stdin).get("torrents",[])))')"
  if ((after >= before + COUNT)); then
    break
  fi
  sleep 0.5
done

echo "torrents after: ${after} (delta $((after - before)))"
curl -fsS -o /dev/null -w "GET /api/v1/torrents: %{time_total}s http=%{http_code}\n" "${BASE_URL}/api/v1/torrents"

if [[ "${KEEP}" != "1" ]]; then
  rm -rf "${WORKDIR}"
fi
echo "seed fixtures OK"
