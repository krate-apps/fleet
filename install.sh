#!/usr/bin/env bash
# Fleet installer (INS-001…009 / SEC-007).
# Installs a release tree under /opt/<user>/fleet without touching rTorrent.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: install.sh [--prefix DIR] [--user NAME] [--version VER] [--bin PATH] [--non-interactive]

Environment (non-interactive / Krate integration — INS-002):
  FLEET_PREFIX            Install prefix (default: /opt/<user>/fleet)
  FLEET_USER              Unix account that will own the instance (default: current user)
  FLEET_VERSION           Release version directory name (default: 0.0.0-dev)
  FLEET_BIN               Path to the fleet binary to install (default: ./bin/fleet)
  FLEET_UNIT              Install systemd unit when root (default: 1). Set 0 to skip.
  FLEET_OFFICIAL_BUCKET   Optional path to an Official bucket tree to stage into the prefix

This installer does not install or reconfigure rTorrent (INS-003).
It never runs chmod 777; world-writable paths are rejected (SEC-007 / INS-008).
EOF
}

FLEET_USER="${FLEET_USER:-$(id -un)}"
FLEET_VERSION="${FLEET_VERSION:-0.0.0-dev}"
FLEET_BIN="${FLEET_BIN:-./bin/fleet}"
FLEET_UNIT="${FLEET_UNIT:-1}"
PREFIX_SET=0
NON_INTERACTIVE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) FLEET_PREFIX="$2"; PREFIX_SET=1; shift 2 ;;
    --user) FLEET_USER="$2"; shift 2 ;;
    --version) FLEET_VERSION="$2"; shift 2 ;;
    --bin) FLEET_BIN="$2"; shift 2 ;;
    --non-interactive) NON_INTERACTIVE=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ "${PREFIX_SET}" -eq 0 ]]; then
  FLEET_PREFIX="${FLEET_PREFIX:-/opt/${FLEET_USER}/fleet}"
fi

if [[ "${FLEET_VERSION}" == *"/"* || "${FLEET_VERSION}" == *".."* ]]; then
  echo "invalid version: ${FLEET_VERSION}" >&2
  exit 2
fi

if [[ ! -f "${FLEET_BIN}" ]]; then
  echo "fleet binary not found: ${FLEET_BIN}" >&2
  echo "Provide --bin PATH (or build a binary first)." >&2
  exit 1
fi

refuse_world_writable() {
  local path="$1"
  [[ -e "${path}" ]] || return 0
  local mode
  mode="$(stat -c '%a' "${path}" 2>/dev/null || stat -f '%OLp' "${path}")"
  local other="${mode: -1}"
  case "${other}" in
    2|3|6|7)
      echo "refusing world-writable path: ${path} (mode ${mode}); fix permissions manually (no chmod 777)" >&2
      exit 1
      ;;
  esac
}

# FLEET_HOME overrides the Unix home for config/data/cache (tests / custom layouts).
if [[ -n "${FLEET_HOME:-}" ]]; then
  HOME_DIR="${FLEET_HOME}"
else
  HOME_DIR="$(getent passwd "${FLEET_USER}" | cut -d: -f6 || true)"
fi
if [[ -z "${HOME_DIR}" ]]; then
  echo "unknown user: ${FLEET_USER}" >&2
  exit 1
fi

RELEASE_DIR="${FLEET_PREFIX}/releases/${FLEET_VERSION}"
CONFIG_DIR="${HOME_DIR}/.config/fleet"
DATA_DIR="${HOME_DIR}/.local/share/fleet"
CACHE_DIR="${HOME_DIR}/.cache/fleet"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Docs/examples when running from a Fleet source checkout; release tarballs may ship copies under PREFIX/packaging.
DOCS_EXAMPLES="${SCRIPT_DIR}/docs/user/examples"

echo "Installing Fleet ${FLEET_VERSION}"
echo "  user:    ${FLEET_USER}"
echo "  prefix:  ${FLEET_PREFIX}"
echo "  binary:  ${FLEET_BIN}"

install -d -m 0755 "${FLEET_PREFIX}"
install -d -m 0755 "${FLEET_PREFIX}/releases"
install -d -m 0755 "${RELEASE_DIR}"
install -d -m 0755 "${RELEASE_DIR}/web"
install -d -m 0755 "${RELEASE_DIR}/tools"
install -d -m 0700 "${CONFIG_DIR}"
install -d -m 0700 "${DATA_DIR}"
install -d -m 0700 "${CACHE_DIR}"

refuse_world_writable "${FLEET_PREFIX}"
refuse_world_writable "${RELEASE_DIR}"
refuse_world_writable "${CONFIG_DIR}"
refuse_world_writable "${DATA_DIR}"

install -m 0755 "${FLEET_BIN}" "${RELEASE_DIR}/fleet"

if [[ -d "./web/dist" ]]; then
  cp -a ./web/dist/. "${RELEASE_DIR}/web/"
elif [[ -d "./cmd/fleet/web" ]] && compgen -G "./cmd/fleet/web/*" >/dev/null; then
  cp -a ./cmd/fleet/web/. "${RELEASE_DIR}/web/" || true
fi

# Stage optional assets into the prefix (unit, seed config, Official bucket).
PREFIX_PACKAGING="${FLEET_PREFIX}/packaging"
install -d -m 0755 "${PREFIX_PACKAGING}"

# Official bucket: FLEET_OFFICIAL_BUCKET, else ./official-bucket next to this script (release layout).
OFFICIAL_SRC=""
if [[ -n "${FLEET_OFFICIAL_BUCKET:-}" && -f "${FLEET_OFFICIAL_BUCKET}/bucket.json" ]]; then
  OFFICIAL_SRC="${FLEET_OFFICIAL_BUCKET}"
elif [[ -f "${SCRIPT_DIR}/official-bucket/bucket.json" ]]; then
  OFFICIAL_SRC="${SCRIPT_DIR}/official-bucket"
fi
if [[ -n "${OFFICIAL_SRC}" ]]; then
  rm -rf "${PREFIX_PACKAGING}/official-bucket"
  mkdir -p "${PREFIX_PACKAGING}/official-bucket"
  if [[ -d "${OFFICIAL_SRC}/packages" ]]; then
    rsync -a --exclude '.git/' "${OFFICIAL_SRC}/" "${PREFIX_PACKAGING}/official-bucket/"
  else
    rsync -a --exclude '.git/' --exclude 'packages/' "${OFFICIAL_SRC}/" "${PREFIX_PACKAGING}/official-bucket/"
  fi
else
  echo "NOTE: no Official bucket staged (set FLEET_OFFICIAL_BUCKET or ship ./official-bucket)."
fi

if [[ -f "${DOCS_EXAMPLES}/systemd/fleet@.service" ]]; then
  install -d -m 0755 "${PREFIX_PACKAGING}/systemd"
  install -m 0644 "${DOCS_EXAMPLES}/systemd/fleet@.service" "${PREFIX_PACKAGING}/systemd/fleet@.service"
fi
if [[ -f "${DOCS_EXAMPLES}/config.example.yaml" ]]; then
  install -m 0644 "${DOCS_EXAMPLES}/config.example.yaml" "${PREFIX_PACKAGING}/config.example.yaml"
fi

BIN_DIR="$(cd "$(dirname "${FLEET_BIN}")" && pwd)"
for helper in fleet-mktorrent fleet-dumptorrent rtprobe; do
  if [[ -x "${BIN_DIR}/${helper}" ]]; then
    install -m 0755 "${BIN_DIR}/${helper}" "${RELEASE_DIR}/tools/${helper}"
  fi
done

TMP="${FLEET_PREFIX}/current.new.$$"
ln -sfn "releases/${FLEET_VERSION}" "${TMP}"
mv -Tf "${TMP}" "${FLEET_PREFIX}/current"

EXAMPLE_CONFIG="${DOCS_EXAMPLES}/config.example.yaml"
if [[ ! -f "${EXAMPLE_CONFIG}" ]]; then
  EXAMPLE_CONFIG="${PREFIX_PACKAGING}/config.example.yaml"
fi
TARGET_CONFIG="${CONFIG_DIR}/config.yaml"
OFFICIAL_BUCKET_PATH="${PREFIX_PACKAGING}/official-bucket"
if [[ ! -f "${TARGET_CONFIG}" ]]; then
  if [[ -f "${EXAMPLE_CONFIG}" ]]; then
    install -m 0600 "${EXAMPLE_CONFIG}" "${TARGET_CONFIG}"
  else
    cat > "${TARGET_CONFIG}" <<EOF
listen:
  host: 127.0.0.1
  port: 8080
rtorrent:
  address: ""
install_prefix: ${FLEET_PREFIX}
data_dir: ${DATA_DIR}
log_level: info
EOF
    chmod 0600 "${TARGET_CONFIG}"
  fi
  if ! grep -q '^install_prefix:' "${TARGET_CONFIG}" 2>/dev/null; then
    printf '\ninstall_prefix: %s\n' "${FLEET_PREFIX}" >> "${TARGET_CONFIG}"
  fi
  echo "Wrote ${TARGET_CONFIG} (no default password — complete setup in the UI)"
else
  echo "Kept existing ${TARGET_CONFIG}"
fi

if [[ -d "${OFFICIAL_BUCKET_PATH}" ]]; then
  python3 - "${TARGET_CONFIG}" "${OFFICIAL_BUCKET_PATH}" <<'PY' || true
import pathlib, re, sys
cfg, path = pathlib.Path(sys.argv[1]), sys.argv[2]
text = cfg.read_text()
line = f"  official_path: {path}"
if re.search(r"(?m)^[ \t]*official_path:", text):
    text = re.sub(r"(?m)^[ \t]*official_path:.*$", line, text, count=1)
elif re.search(r"(?m)^buckets:\s*$", text):
    text = re.sub(r"(?m)^buckets:\s*$", "buckets:\n" + line, text, count=1)
else:
    text = text.rstrip() + "\n\nbuckets:\n" + line + "\n"
cfg.write_text(text)
PY
  echo "Official bucket path: ${OFFICIAL_BUCKET_PATH}"
fi

ADDR="$(awk '/^[[:space:]]*address:/{print $2; exit}' "${TARGET_CONFIG}" 2>/dev/null | tr -d "\"'" || true)"
if [[ "${ADDR}" == unix://* ]]; then
  SOCK="${ADDR#unix://}"
  if [[ -e "${SOCK}" ]]; then
    refuse_world_writable "${SOCK}"
    if [[ -r "${SOCK}" && -w "${SOCK}" ]]; then
      echo "rtorrent socket OK: ${SOCK}"
    else
      echo "WARN: rtorrent socket not read/write for current user: ${SOCK}" >&2
    fi
  else
    echo "WARN: rtorrent socket path not found yet: ${SOCK}" >&2
  fi
elif [[ -z "${ADDR}" || "${ADDR}" == '""' ]]; then
  echo "NOTE: rtorrent.address empty — configure an existing engine socket (INS-003)."
fi

UNIT_SRC="${PREFIX_PACKAGING}/systemd/fleet@.service"
if [[ ! -f "${UNIT_SRC}" ]]; then
  UNIT_SRC="${DOCS_EXAMPLES}/systemd/fleet@.service"
fi
if [[ -f "${UNIT_SRC}" && "$(id -u)" -eq 0 && "${FLEET_UNIT}" -eq 1 ]]; then
  install -m 0644 "${UNIT_SRC}" /etc/systemd/system/fleet@.service
  systemctl daemon-reload || true
  echo "Installed systemd unit fleet@.service (enable with: systemctl enable --now fleet@${FLEET_USER})"
elif [[ -f "${UNIT_SRC}" ]]; then
  echo "Unit template available at ${UNIT_SRC}"
  if [[ "$(id -u)" -ne 0 ]]; then
    echo "Run install as root (or copy the unit) to register fleet@.service; FLEET_UNIT=${FLEET_UNIT}"
  elif [[ "${FLEET_UNIT}" -ne 1 ]]; then
    echo "Skipped systemd unit install (FLEET_UNIT=${FLEET_UNIT})."
  fi
else
  echo "WARN: systemd unit missing (docs/user/examples/systemd/fleet@.service); skipped unit install." >&2
fi

if [[ "$(id -u)" -eq 0 && "${FLEET_USER}" != "root" ]]; then
  chown -R "${FLEET_USER}:${FLEET_USER}" "${FLEET_PREFIX}" "${CONFIG_DIR}" "${DATA_DIR}" "${CACHE_DIR}"
fi

echo "Done. Start manually:"
echo "  ${FLEET_PREFIX}/current/fleet -config ${TARGET_CONFIG}"
echo "Uninstall (keeps data by default):"
echo "  ${SCRIPT_DIR}/uninstall.sh --prefix ${FLEET_PREFIX} --user ${FLEET_USER} --non-interactive"
