#!/usr/bin/env bash
# Smoke: install.sh + uninstall.sh against a fake binary (no root / no systemd).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

BIN="${TMP}/fleet-bin"
PREFIX="${TMP}/prefix"
HOME_FAKE="${TMP}/home"
mkdir -p "${HOME_FAKE}"
printf '#!/bin/true\n' >"${BIN}"
chmod +x "${BIN}"

FLEET_UNIT=0 FLEET_HOME="${HOME_FAKE}" "${ROOT}/install.sh" \
  --non-interactive \
  --user "$(id -un)" \
  --prefix "${PREFIX}" \
  --version "0.0.0-test" \
  --bin "${BIN}"

test -x "${PREFIX}/releases/0.0.0-test/fleet"
test -L "${PREFIX}/current"
target="$(readlink "${PREFIX}/current")"
[[ "${target}" == "releases/0.0.0-test" ]]
test -f "${HOME_FAKE}/.config/fleet/config.yaml"

FLEET_HOME="${HOME_FAKE}" "${ROOT}/uninstall.sh" \
  --non-interactive \
  --user "$(id -un)" \
  --prefix "${PREFIX}" \
  --purge-data

test ! -e "${PREFIX}"
test ! -e "${HOME_FAKE}/.config/fleet"

echo "install_uninstall_smoke OK"
