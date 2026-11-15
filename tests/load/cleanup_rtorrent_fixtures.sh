#!/usr/bin/env bash
# Erase torrents whose name starts with PREFIX from a running Fleet (no data delete).
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
PREFIX="${PREFIX:-fleet-perf}"

mapfile -t arr < <(PREFIX="${PREFIX}" curl -fsS "${BASE_URL}/api/v1/torrents" | python3 -c '
import json,sys,os
prefix=os.environ["PREFIX"]
d=json.load(sys.stdin)
for t in d.get("torrents",[]):
    if str(t.get("name","")).startswith(prefix):
        print(t["hash"])
')

if [[ "${#arr[@]}" -eq 0 ]]; then
  echo "no torrents matching prefix ${PREFIX}"
  exit 0
fi

echo "erasing ${#arr[@]} torrents with prefix ${PREFIX}"
chunk=100
for ((i = 0; i < ${#arr[@]}; i += chunk)); do
  slice=("${arr[@]:i:chunk}")
  payload="$(python3 -c 'import json,sys; print(json.dumps({"action":"erase","hashes":sys.argv[1:]}))' "${slice[@]}")"
  curl -fsS -X POST "${BASE_URL}/api/v1/torrents/actions" \
    -H 'Content-Type: application/json' \
    -d "${payload}" >/dev/null
  echo "erased through $((i + ${#slice[@]}))"
done

after="$(curl -fsS "${BASE_URL}/api/v1/torrents" | python3 -c 'import sys,json;print(len(json.load(sys.stdin).get("torrents",[])))')"
echo "torrents remaining: ${after}"
echo "cleanup fixtures OK"
