# FAQ

## Does Fleet replace my torrent client?

No. Fleet is a multi-client web UI and control plane. Your qBittorrent, Deluge, Transmission, and/or rTorrent instances remain the BitTorrent engines.

## Which clients are supported?

qBittorrent, Deluge, Transmission, and rTorrent. See [Clients](clients.md).

## Where is my data?

- Config: `~/.config/fleet/config.yaml`
- SQLite / extensions / backups: `~/.local/share/fleet/`
- Binary prefix: `/opt/<user>/fleet/` (default)

Torrent payloads stay where each client already stores them.

## Why can’t I erase data / use Files / share links?

Those path-guarded features use `rtorrent.download_roots` (absolute directories). Paths outside those roots are rejected. See [Clients](clients.md).

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
