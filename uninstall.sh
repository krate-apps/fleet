#!/usr/bin/env bash
# Fleet uninstaller (INS-009). Never deletes instance data unless --purge-data is set.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: uninstall.sh [--prefix DIR] [--user NAME] [--purge-data] [--non-interactive]

Removes the install prefix release tree and optional systemd unit.
Keeps ~/.config/fleet, ~/.local/share/fleet, and ~/.cache/fleet unless --purge-data.

Environment:
  FLEET_PREFIX
  FLEET_USER
EOF
}

FLEET_USER="${FLEET_USER:-$(id -un)}"
PREFIX_SET=0
PURGE_DATA=0
NON_INTERACTIVE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) FLEET_PREFIX="$2"; PREFIX_SET=1; shift 2 ;;
    --user) FLEET_USER="$2"; shift 2 ;;
    --purge-data) PURGE_DATA=1; shift ;;
    --non-interactive) NON_INTERACTIVE=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ "${PREFIX_SET}" -eq 0 ]]; then
  FLEET_PREFIX="${FLEET_PREFIX:-/opt/${FLEET_USER}/fleet}"
fi

if [[ -n "${FLEET_HOME:-}" ]]; then
  HOME_DIR="${FLEET_HOME}"
else
  HOME_DIR="$(getent passwd "${FLEET_USER}" | cut -d: -f6 || true)"
fi
if [[ -z "${HOME_DIR}" ]]; then
  echo "unknown user: ${FLEET_USER}" >&2
  exit 1
fi

CONFIG_DIR="${HOME_DIR}/.config/fleet"
DATA_DIR="${HOME_DIR}/.local/share/fleet"
CACHE_DIR="${HOME_DIR}/.cache/fleet"

if [[ "${NON_INTERACTIVE}" -eq 0 && -t 0 ]]; then
  echo "This will remove ${FLEET_PREFIX}"
  if [[ "${PURGE_DATA}" -eq 1 ]]; then
    echo "AND purge ${CONFIG_DIR}, ${DATA_DIR}, ${CACHE_DIR}"
  else
    echo "Data directories will be kept."
  fi
  read -r -p "Continue? [y/N] " ans
  case "${ans}" in
    y|Y|yes|YES) ;;
    *) echo "Aborted."; exit 1 ;;
  esac
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl disable --now "fleet@${FLEET_USER}" 2>/dev/null || true
fi

if [[ -d "${FLEET_PREFIX}" ]]; then
  rm -rf "${FLEET_PREFIX}"
  echo "Removed ${FLEET_PREFIX}"
else
  echo "Prefix not found: ${FLEET_PREFIX}"
fi

if [[ "${PURGE_DATA}" -eq 1 ]]; then
  rm -rf "${CONFIG_DIR}" "${DATA_DIR}" "${CACHE_DIR}"
  echo "Purged config/data/cache for ${FLEET_USER}"
else
  echo "Kept data:"
  echo "  ${CONFIG_DIR}"
  echo "  ${DATA_DIR}"
  echo "  ${CACHE_DIR}"
fi

echo "Done."
