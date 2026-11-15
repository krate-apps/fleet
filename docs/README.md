# Fleet documentation

English user and developer guides for [Fleet](https://github.com/krate-apps/fleet).

Fleet is a self-hosted **multi-client** BitTorrent **web UI** for **qBittorrent**, **Deluge**, **Transmission**, and **rTorrent**. It is not a BitTorrent engine — your clients keep torrent authority.

## User guides

| Guide | Topics |
| --- | --- |
| [Install](user/install.md) | Requirements, install script, config, systemd, updates |
| [Docker](user/docker.md) | GHCR image, Compose example, external clients |
| [Reverse proxy](user/reverse-proxy.md) | Caddy, nginx, Apache (path mode + subdomain) + [examples/](user/examples/) |
| [UI](user/ui.md) | Torrents, Files, status bar, keyboard shortcuts |
| [Clients](user/clients.md) | Multi-client gateways (qBittorrent, Deluge, Transmission) |
| [Extensions](user/extensions.md) | Buckets, install, activate, consent |
| [Themes](user/themes.md) | Built-in and installable appearance packs |
| [Coming from…](user/coming-from.md) | Deluge / ruTorrent → Fleet maps |
| [FAQ](user/faq.md) | Common questions |

## Developer guides

| Guide | Topics |
| --- | --- |
| [Architecture](developer/architecture.md) | Process model, SQLite, extensions, packaging layout |
| [Build](developer/build.md) | Local build and tests |
| [SDK](developer/sdk.md) | Public contracts, worker IPC, declarative UI · [schemas/](developer/schemas/) |
| [Create an extension](developer/create-extension.md) | Manifest, worker, declarative UI, template |
| [Create a bucket](developer/create-bucket.md) | Listings, pins, template |
| [Hooks](developer/hooks.md) | Public webhook ingest routes |
| [Tools](developer/tools.md) | mktorrent / dumptorrent helpers |
| [Publishing extensions](developer/publishing-extensions.md) | Release assets, bucket pins |

## Project

- [Contributing](../CONTRIBUTING.md) — setup, PRs, Conventional Commits
- [Security](../SECURITY.md) — vulnerability reporting and operator hardening

## Related repositories

- Host: [krate-apps/fleet](https://github.com/krate-apps/fleet)
- Official catalogue: [fleet-official-bucket](https://github.com/krate-apps/fleet-official-bucket)
- Extension template: `templates/extension` (`fleet-template-ext`)
- Bucket template: `templates/bucket` (`fleet-template-bucket`)
- Bucket CI: [`fleet-bucket-action`](https://github.com/krate-apps/fleet-bucket-action) (`actions/fleet-bucket-action`)

Internal CDC, parity, and PERF notes live in the workspace `docs/` backup only — they are not part of this published tree.

---

<p align="center">
  Developed by the <a href="https://runkrate.com">Krate</a> team.<br />
  Released under the MIT License.
</p>
