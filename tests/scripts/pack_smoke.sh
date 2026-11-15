#!/usr/bin/env bash
# Smoke: templates/extension make pack produces archive + SHA256SUMS.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EXT="${ROOT}/../templates/extension"
if [[ ! -f "${EXT}/scripts/pack.sh" ]]; then
  echo "skip pack_smoke: templates/extension not beside app/" >&2
  exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT
# Pack into an isolated out dir to avoid dirtying the template tree permanently.
(
  cd "${EXT}"
  bash scripts/pack.sh "${TMP}/dist"
)
test -f "${TMP}/dist/"*-linux-amd64.tar.gz
test -f "${TMP}/dist/SHA256SUMS"
( cd "${TMP}/dist" && sha256sum -c SHA256SUMS --quiet )
echo "pack_smoke OK"
