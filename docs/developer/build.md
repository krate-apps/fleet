# Build

## Requirements

- Go 1.22+
- Node.js 22+ (frontend build only)

## Development cycle

```bash
go mod tidy
cd web && npm install && npm run build && cd ..
make embed build
./bin/fleet -config docs/user/examples/config.example.yaml
```

Open http://127.0.0.1:8080/ — health: `GET /api/v1/health`.

Path-mode UI (see [Reverse proxy](../user/reverse-proxy.md)):

```bash
FLEET_UI_BASE=/your/prefix/ make embed build
```

## Tests

```bash
make test          # Go unit + web typecheck + script smokes (+ bucket-action tests if sibling present)
make test-scripts  # install/uninstall/pack smokes only
make perf-cache    # torrent cache benchmarks
make certify       # test + short benches + e2e package
make e2e-smoke     # needs Fleet / rTorrent
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

At the Fleet host root (`app/`): `install.sh`, `uninstall.sh`. systemd, seed config, and proxy snippets live under `docs/user/examples/`.

## Tools

```bash
make tools-json    # fleet-mktorrent / fleet-dumptorrent
```

See [Tools](tools.md).
