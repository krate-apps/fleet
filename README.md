# Fleet

Self-hosted **multi-client** BitTorrent web UI for **qBittorrent**, **Deluge**, **Transmission**, and **rTorrent**.

Fleet is not a BitTorrent engine. One Vue UI talks to your existing clients, ships as a single Go binary, and keeps Fleet-owned state in SQLite (sessions, preferences, extensions — never torrent authority). See [Clients](docs/user/clients.md).

## Documentation

**Start here:** [docs/](docs/README.md)

| | |
| --- | --- |
| Users | [Install](docs/user/install.md) · [Docker](docs/user/docker.md) · [Reverse proxy](docs/user/reverse-proxy.md) · [UI](docs/user/ui.md) · [Clients](docs/user/clients.md) · [Extensions](docs/user/extensions.md) · [Themes](docs/user/themes.md) · [Coming from…](docs/user/coming-from.md) · [FAQ](docs/user/faq.md) |
| Developers | [Architecture](docs/developer/architecture.md) · [Build](docs/developer/build.md) · [SDK](docs/developer/sdk.md) · [Create extension](docs/developer/create-extension.md) · [Create bucket](docs/developer/create-bucket.md) · [Hooks](docs/developer/hooks.md) · [Tools](docs/developer/tools.md) · [Publishing](docs/developer/publishing-extensions.md) |
| Project | [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) |

Official catalogue: [fleet-official-bucket](https://github.com/krate-apps/fleet-official-bucket).

## Quick start

Toolchain versions and the full `make embed build` cycle live in **[Build](docs/developer/build.md)** (single source of truth).

```bash
make embed build
./bin/fleet -config docs/user/examples/config.example.yaml
```

Open http://127.0.0.1:8080/ — API health: `GET /api/v1/health`. Register clients under **Settings → Clients** (and/or `rtorrent.address` in config).

## Layout

```text
cmd/fleet/          Go entrypoint (+ embedded web/)
internal/           API, auth, clients/gateways, sync, extensions, fsops, share, …
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

Does **not** install or reconfigure your torrent clients. Point Fleet at them via config and **Settings → Clients** — see [Install](docs/user/install.md), [Docker](docs/user/docker.md), and [Clients](docs/user/clients.md).

## Tests

```bash
make test
make certify
```

Details: [Build](docs/developer/build.md).

## License

[MIT](LICENSE) — Copyright (c) 2026 Krate.

Third-party dependency notices: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

---

<p align="center">
  Developed by the <a href="https://runkrate.com">Krate</a> team.<br />
  Released under the MIT License.
</p>
