# Build

Canonical toolchain and local build steps for the Fleet **host** repository. Other docs link here instead of repeating versions.

## Toolchain

| Tool | Version | When needed |
| --- | --- | --- |
| Go | **1.22+** | Always (host binary, tests, tools) |
| Node.js | **26+** | Frontend build / typecheck / lint (`web/`) — not required to *run* a prebuilt binary |

CI uses the same majors (see `.github/workflows/`).

## Development cycle

```bash
go mod tidy
cd web && npm ci && npm run build && cd ..
make embed build
./bin/fleet -config docs/user/examples/config.example.yaml
```

Open http://127.0.0.1:8080/ — health: `GET /api/v1/health`.

Shorthand after dependencies are installed: `make embed build`.

Path-mode UI (see [Reverse proxy](../user/reverse-proxy.md)):

```bash
FLEET_UI_BASE=/your/prefix/ make embed build
```

Connect any supported client (rTorrent, qBittorrent, Deluge, Transmission) — see [Clients](../user/clients.md). Empty `rtorrent.address` is fine; Fleet still serves the UI and polls configured gateways.

## Tests

```bash
make test          # Go unit + web typecheck + script smokes (+ bucket-action tests if sibling present)
make test-scripts  # install/uninstall/pack smokes only
make perf-cache    # torrent cache benchmarks
make certify       # test + short benches + e2e package
make e2e-smoke     # needs a running Fleet (+ typically a live client)
make load-smoke
make rtprobe RTPROBE_ADDR=unix:///path/to.sock
```

## Extension workers (optional workspace helper)

With checkouts under `../extensions/fleet-ext-*`, the host Makefile can build all workers in-place (no symlinks into `app/extensions/`):

```bash
make ext-workers
```

Prefer building/packing inside each extension repo (`make build` / `make pack`). See [Publishing](publishing-extensions.md).

Official bucket for local runs: set `buckets.official_path: ../buckets/fleet-official-bucket` or leave empty and rely on the default candidate path.

## Product scripts

At the Fleet host root (`app/`): `install.sh`, `uninstall.sh`. systemd, seed config, Docker, and proxy snippets live under `docs/user/examples/`.

## Tools

```bash
make tools-json    # bin/mktorrent + bin/dumptorrent (../deps or shallow clone into .deps/)
```

See [Tools](tools.md).

## Container image

Local image build: [Docker](../user/docker.md). Release images publish to `ghcr.io/krate-apps/fleet`.
