# Docker

Fleet publishes a **Fleet-only** image to GitHub Container Registry. It does **not** bundle torrent engines — connect qBittorrent, Deluge, Transmission, and/or rTorrent as external clients ([Clients](clients.md)).

Image: `ghcr.io/krate-apps/fleet` (tags `vX.Y.Z` and `latest`, **linux/amd64** only for now).

The classic binary + [`install.sh`](install.md) path remains the default for bare-metal / systemd hosts. Docker is an additional install option.

## Quick start (Compose)

Examples live under [`examples/docker/`](examples/docker/):

```bash
mkdir -p config data
cp docs/user/examples/docker/config.yaml config/config.yaml
# edit config/config.yaml — set rtorrent.address and/or plan gateway URLs
cp docs/user/examples/docker/compose.yml .
docker compose up -d
```

Open http://127.0.0.1:8080/ and complete **/setup** (first administrator).

Volumes:

| Mount | Purpose |
| --- | --- |
| `./config/config.yaml` → `/config/config.yaml` (ro) | Fleet config (`listen.host` must be `0.0.0.0` in the container) |
| `./data` → `/data` | SQLite, extensions, secrets key, caches |

The image entrypoint chowns `/data` then runs as uid `1000`. Keep the config file readable by that user (mode `0644` is fine when mounted read-only).

## Pull without Compose

```bash
docker pull ghcr.io/krate-apps/fleet:latest
docker run --rm -p 8080:8080 \
  -v "$PWD/config/config.yaml:/config/config.yaml:ro" \
  -v "$PWD/data:/data" \
  --add-host=host.docker.internal:host-gateway \
  ghcr.io/krate-apps/fleet:latest
```

## Connecting external clients

- **qBittorrent / Deluge / Transmission** — **Settings → Clients** with URLs reachable from the container (`http://host.docker.internal:…` or a shared Compose network name).
- **rTorrent** — `rtorrent.address` (e.g. `scgi://host.docker.internal:5000`, or a Unix socket mounted under `/data`).
- Path-guarded Files / erase_data: mount download dirs and list them under `rtorrent.download_roots` when using rTorrent.

## TLS and cookies

Put a reverse proxy in front for HTTPS ([Reverse proxy](reverse-proxy.md)). Set `auth.secure_cookies: true` (or rely on `X-Forwarded-Proto: https`). Never set `auth.disable: true` with `listen.host: 0.0.0.0`.

## Build locally

From the host repository root (`app/`):

```bash
docker build -t fleet:local \
  --build-arg VERSION=0.0.0-dev \
  --build-arg COMMIT="$(git rev-parse --short HEAD)" \
  .
```

## Limits

- No torrent engine inside the image
- No multi-arch `arm64` image yet
- Official catalogue and extension workers still download/run as on a normal host install (workers need binaries under mounted or discovered extension trees)
