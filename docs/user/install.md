# Install

## Requirements

- Linux amd64 (Debian 13 reference)
- At least one torrent client Fleet can reach:
  - **rTorrent** with SCGI (Unix socket or TCP) — deepest integration (Files / `download_roots` / erase_data)
  - and/or **qBittorrent**, **Deluge**, **Transmission** registered under **Settings → Clients**
- Optional: Node.js 22+ only when building the UI from source

Fleet does **not** install or reconfigure your torrent clients.

Prefer containers? See [Docker](docker.md) (`ghcr.io/krate-apps/fleet`) — Fleet-only image; clients stay external. This page covers the binary / systemd install.

## Quick install (from a built binary)

```bash
make embed build
./install.sh --non-interactive --user "$USER" --bin ./bin/fleet
```

Typical layout after install:

```text
/opt/<user>/fleet/
  releases/<version>/fleet
  current -> releases/<version>

~/.config/fleet/config.yaml
~/.local/share/fleet/          # fleet.db, backups, updates, extensions-src
~/.cache/fleet/
```

### Non-interactive environment

| Variable | Meaning |
| --- | --- |
| `FLEET_PREFIX` | Install root (default `/opt/<user>/fleet`) |
| `FLEET_USER` | Owning Unix account |
| `FLEET_VERSION` | Release directory name |
| `FLEET_BIN` | Path to built `fleet` binary |
| `FLEET_UNIT` | Default `1`: as root, install `fleet@.service`. Set `0` to skip. |
| `FLEET_OFFICIAL_BUCKET` | Optional path to Official bucket catalogue to stage into the prefix (else `./official-bucket` next to `install.sh` if present) |
| `FLEET_HOME` | Optional home override for config/data/cache paths (tests / custom layouts) |

## Configuration

Seed file: [examples/config.example.yaml](examples/config.example.yaml).

```bash
mkdir -p ~/.config/fleet
cp docs/user/examples/config.example.yaml ~/.config/fleet/config.yaml
chmod 600 ~/.config/fleet/config.yaml
```

`install.sh` copies the same example when no config exists yet.

Important keys:

- `rtorrent.address` — `unix:///path/to.sock` or `scgi://host:port`
- `rtorrent.download_roots` — absolute directories Fleet may touch for erase_data, Files, shares, and path-guarded tools
- Optional `tools:` helpers (`mktorrent`, `dumptorrent`, `unrar`, `mediainfo`, `ffmpeg`, `sox`)
- Optional `geoip.database` (MaxMind Country MMDB) for peer country codes
- `buckets.official_path` — e.g. `../buckets/fleet-official-bucket` in a workspace checkout

World-writable paths are rejected; Fleet never applies `chmod 777`.

## systemd

Unit template: [examples/systemd/fleet@.service](examples/systemd/fleet@.service).

```bash
systemctl enable --now fleet@$USER
```

Start with an explicit config when debugging:

```bash
/opt/$USER/fleet/current/fleet -config ~/.config/fleet/config.yaml
```

See [Reverse proxy](reverse-proxy.md) for `ReadWritePaths` notes with `ProtectHome`.

## First-run auth

1. Open the UI and complete **/setup** (first administrator). There is no default password.
2. Later sign-ins use **/login**. Sessions use HttpOnly cookies; mutating API calls require `X-CSRF-Token`.
3. Disable auth only for local automation: `auth.disable: true` (not for production).

## Reverse proxy

Caddy, nginx, and Apache examples (path mode + subdomain): **[Reverse proxy](reverse-proxy.md)**.

Path-mode UI build:

```bash
FLEET_UI_BASE=/your/prefix/ make embed build
```

## Updates and rollback

Auto-update is always off. In **Settings → Updates**:

1. Choose channel `stable` or `pre`.
2. **Prepare** a release (download + verify).
3. **Apply** backs up SQLite and copies the binary under `<prefix>/releases/<version>/` when the prefix is writable.

Then flip `current` and restart (Fleet never rewrites the install symlink from the HTTP process):

```bash
ln -sfn releases/v1.2.0 /opt/$USER/fleet/current
systemctl restart fleet@$USER
```

SQLite backups use `VACUUM INTO` under `~/.local/share/fleet/backups/`.

## Uninstall

```bash
./uninstall.sh --user "$USER" --non-interactive
# add --purge-data only when config + SQLite + cache must go
```

## System metrics

`GET /api/v1/system/status` returns disk usage for each `download_roots` entry and load average when available. The status bar and **Settings → System** show the same data.

When the Metrics extension is active, it binds a **loopback-only** OpenMetrics scrape server (default `127.0.0.1:19105`). Optional bearer token is supported.
