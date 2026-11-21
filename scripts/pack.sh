#!/usr/bin/env bash
# Build a linux/amd64 Fleet host release archive + SHA256SUMS.
# Usage: VERSION=1.0.0 bash scripts/pack.sh [out-dir]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

OUT_DIR="${1:-dist}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)}"
VERSION="${VERSION#v}"
ASSET="fleet-v${VERSION}-linux-amd64.tgz"

mkdir -p "${OUT_DIR}" bin

echo "Building Fleet ${VERSION}…"
make embed build VERSION="v${VERSION}"
go build -trimpath -ldflags "-s -w -X github.com/krate-apps/fleet/internal/version.Version=v${VERSION}" -o bin/rtprobe ./cmd/rtprobe

STAGE="$(mktemp -d)"
trap 'rm -rf "${STAGE}"' EXIT
DEST="${STAGE}/fleet"
mkdir -p "${DEST}/bin"

cp -a bin/fleet bin/mktorrent bin/dumptorrent bin/rtprobe "${DEST}/bin/"
chmod +x "${DEST}/bin/"*
cp -a install.sh uninstall.sh "${DEST}/"
cp -a LICENSE README.md THIRD_PARTY_NOTICES.md "${DEST}/"
if [[ -d docs/user/examples ]]; then
  mkdir -p "${DEST}/docs/user"
  cp -a docs/user/examples "${DEST}/docs/user/"
fi

tar -C "${STAGE}" -czf "${OUT_DIR}/${ASSET}" fleet
(
  cd "${OUT_DIR}"
  sha256sum "${ASSET}" | tee SHA256SUMS
)
echo "Wrote ${OUT_DIR}/${ASSET}"
