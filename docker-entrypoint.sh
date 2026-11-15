#!/bin/sh
# Fix /data ownership when started as root, then drop to fleet (uid 1000).
# /config is often mounted read-only — never chown it.
set -eu
if [ "$(id -u)" = "0" ]; then
  mkdir -p /data
  chown -R fleet:fleet /data
  exec su-exec fleet:fleet /usr/local/bin/fleet "$@"
fi
exec /usr/local/bin/fleet "$@"
