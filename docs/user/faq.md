# FAQ

## Does Fleet replace rTorrent?

No. Fleet is a web UI and control plane. rTorrent remains the BitTorrent engine.

## Where is my data?

- Config: `~/.config/fleet/config.yaml`
- SQLite / extensions / backups: `~/.local/share/fleet/`
- Binary prefix: `/opt/<user>/fleet/` (default)

Torrent data stays where rTorrent already stores it; Fleet only touches paths under `rtorrent.download_roots`.

## Why can’t I erase data / use Files / share links?

Set `rtorrent.download_roots` to absolute directories that contain the payloads. Paths outside those roots are rejected.

## How do I install an official extension?

Enable the **Fleet Official** bucket, open **Extensions**, install by id (for example `org.fleet.rssfeeds`). Listing files use the current slugs (`notifications.yaml`, `autodownload.yaml`, `servarrbridge.yaml`, `traffic.yaml`, …) — not older names like `notify.yaml` / `autodl.yaml`.

## Are third-party extensions sandboxed?

No. They run as processes under your Unix account. Fleet asks for explicit consent before installing non-official packages.

## Can I run multiple Fleet versions?

Yes. Each Unix account owns an isolated instance (prefix, config, SQLite, extensions). Alice and Bob can run different versions without interference.

## Path-mode reverse proxy assets 404?

Rebuild with `FLEET_UI_BASE=/your/prefix/` so the SPA and API links include the strip prefix.

## Where are developer docs?

See the [documentation hub](../README.md) under **Developer guides**.
