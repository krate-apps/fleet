# Fleet

Modern, self-hosted BitTorrent web UI for an existing **rTorrent** instance.

Fleet is not a BitTorrent engine. It connects to rTorrent over SCGI/XML-RPC, serves a Vue 3 frontend from a single Go binary, and keeps its own state in SQLite (sessions, preferences, extensions — never torrent authority).

## Documentation

**Start here:** [docs/](docs/README.md)

| | |
| --- | --- |
| Users | [Install](docs/user/install.md) · [Reverse proxy](docs/user/reverse-proxy.md) · [UI](docs/user/ui.md) · [Clients](docs/user/clients.md) · [Extensions](docs/user/extensions.md) · [Themes](docs/user/themes.md) · [Coming from…](docs/user/coming-from.md) · [FAQ](docs/user/faq.md) |
| Developers | [Architecture](docs/developer/architecture.md) · [Build](docs/developer/build.md) · [SDK](docs/developer/sdk.md) · [Create extension](docs/developer/create-extension.md) · [Create bucket](docs/developer/create-bucket.md) · [Hooks](docs/developer/hooks.md) · [Tools](docs/developer/tools.md) · [Publishing](docs/developer/publishing-extensions.md) |

Official catalogue: [fleet-official-bucket](https://github.com/krate-apps/fleet-official-bucket).

## Status

Milestone **M6 — V1 public candidate**: official SDK extensions, native Files browser, system disk/load metrics, and installable themes. This is **not** a claim of full ruTorrent behavioural parity.

## Requirements

- Go 1.22+
- Node.js 22+ (frontend build only; not required in production)
- An existing rTorrent with SCGI

## Quick start (development)

```bash
go mod tidy
cd web && npm install && npm run build && cd ..
make embed build
./bin/fleet -config docs/user/examples/config.example.yaml
```

Open http://127.0.0.1:8080/ — API health: `GET /api/v1/health`.

## Layout

```text
cmd/fleet/          Go entrypoint (+ embedded web/)
internal/           API, auth, rtorrent, sync, extensions, fsops, share, …
web/                Vue 3 + TypeScript + Vite
sdk/                Go helpers for extension contracts (manifest, UI, theme, IPC)
docs/               User + developer guides; JSON Schema under docs/developer/schemas/
install.sh          Product installer (root)
uninstall.sh        Product uninstall
tests/              e2e / load / certify helpers
```

Workspace (siblings of `app/`): `extensions/`, `themes/`, `buckets/fleet-official-bucket`, `templates/`. See [Architecture](docs/developer/architecture.md).

## Install (minimal)

```bash
make build
./install.sh --non-interactive --user "$USER" --bin ./bin/fleet
```

Does **not** install or reconfigure rTorrent. Point `rtorrent.address` in `~/.config/fleet/config.yaml` at your existing socket or TCP SCGI endpoint. Set `rtorrent.download_roots` for erase_data, Files, shares, and media tools.

See [Install](docs/user/install.md) for systemd, reverse proxy (`FLEET_UI_BASE`), updates, and uninstall.

## Tests

```bash
make test
make certify
```

See `tests/README.md` and [Build](docs/developer/build.md).

## License

[MIT](LICENSE) — Copyright (c) 2026 Krate.

Third-party dependency notices: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

---

<p align="center">
  Developed by the <a href="https://runkrate.com">Krate</a> team.<br />
  Released under the MIT License.
</p>
