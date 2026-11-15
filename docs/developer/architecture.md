# Architecture

## What Fleet is

Fleet is a single Go binary that:

1. Talks to your torrent **clients** — rTorrent (SCGI/XML-RPC) and optional gateways for qBittorrent, Deluge, and Transmission ([Clients](../user/clients.md))
2. Serves a Vue 3 frontend (embedded after `make embed`)
3. Keeps Fleet-owned state in **SQLite** (sessions, preferences, extensions — never torrent authority; engines stay in the clients)

## Layout

Published host git root is `app/`. Related products live as workspace siblings (separate repos):

```text
fleet/
  app/                             # this repo (github.com/krate-apps/fleet)
    cmd/ internal/ web/ sdk/ docs/ tests/
    install.sh uninstall.sh        # product scripts (repo root)
    docs/                          # guides + examples; JSON Schema in docs/developer/schemas/
  extensions/fleet-ext-*           # extension repos
  themes/fleet-theme-*             # theme repos
  buckets/fleet-official-bucket    # Official catalogue (org.fleet.official)
  templates/extension              # fleet-template-ext (https://github.com/krate-apps/fleet-template-ext)
  templates/theme                  # fleet-template-theme (https://github.com/krate-apps/fleet-template-theme)
  templates/bucket                 # fleet-template-bucket
  actions/fleet-bucket-action      # bucket CI (validate/hashcheck/excavate/pin)
  docs/                            # internal backup (CDC / old manuals) — not product docs
```

**Where to edit what**

| Concern | Location |
| --- | --- |
| User + developer guides | `app/docs/` |
| systemd / reverse-proxy / seed config | `app/docs/user/examples/` · [reverse-proxy.md](../user/reverse-proxy.md) |
| Installer / uninstall | `app/` root (`install.sh`, `uninstall.sh`) |
| Extension source | `extensions/fleet-ext-*` |
| Theme source | `themes/fleet-theme-*` |
| Catalogue listings | `buckets/fleet-official-bucket` |
| New package scaffolds | `templates/` |

`install.sh` stages an Official catalogue into `<prefix>/packaging/official-bucket` when `FLEET_OFFICIAL_BUCKET` is set or `./official-bucket` is shipped next to the installer (release layout). It does **not** walk the monorepo workspace.

## Instance model

Each Unix account owns an isolated Fleet instance: binary prefix, config, SQLite, and extensions.

## Extensions

Workers speak **JSON-RPC 2.0** on stdin/stdout. Fleet only exposes reverse RPC allowed by the package `capabilities`. Official packages use the public [SDK](sdk.md) contracts (`docs/developer/schemas/` + `sdk/` Go helpers) and must not call private Fleet APIs.

Develop and pack each extension in its own repo (`make build` / `make pack`). Point Fleet at an install path or the Official bucket — the host does not symlink workspace trees.

## Buckets

A bucket is a Git catalogue (`bucket.json` + `bucket/*.json`). The Official bucket id is `org.fleet.official`. Canonical tree: [fleet-official-bucket](https://github.com/krate-apps/fleet-official-bucket). Dev default path: `../buckets/fleet-official-bucket`.
