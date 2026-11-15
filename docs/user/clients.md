# Clients

Fleet is a **multi-client** control plane: one UI over several BitTorrent engines. Torrents stay owned by each client — Fleet never becomes the engine.

## Built-in and gateways

| Client | How it connects |
| --- | --- |
| **rTorrent** | `rtorrent.address` (SCGI Unix socket or TCP) in config — deepest path for Files, `download_roots`, erase_data, shares, media tools |
| **qBittorrent** | **Settings → Clients** gateway (WebUI/API) |
| **Deluge** | **Settings → Clients** gateway |
| **Transmission** | **Settings → Clients** gateway |

Register as many gateways as you need. The torrent table merges rows as `hash@clientId`; toolbar actions and label edits target the selected row’s client. Capability chips show what each kind supports; detail tabs that a client cannot provide show an explicit “not supported” message.

Gateway passwords are stored encrypted at rest (`enc:v1:` AES-GCM). The AES master key is resolved in order: systemd `CREDENTIALS_DIRECTORY/fleet-secrets.key` → Linux user keyring description `fleet-secrets` → `data_dir/.fleet-secrets.key` (`0600`). Legacy plaintext password rows remain readable and migrate on the next save. See the commented `LoadCredential=` line in [`examples/systemd/fleet@.service`](examples/systemd/fleet@.service).

## Logs

Per-client overrides on **Settings → Clients**:

- `journal_unit` — must stay on the allowlist (e.g. `deluge@$USER`)
- `log_path` — regular file under the Fleet process home

## Related extensions

- [Label Bridge](https://github.com/krate-apps/fleet-ext-labelbridge) — sync labels across clients
- [Cross-seed](https://github.com/krate-apps/fleet-ext-crossseed) — push completed downloads to other clients
- [Duplicate Finder](https://github.com/krate-apps/fleet-ext-duplicates) — cross-client duplicates

V1’s richest path-guarded features (Files browser, erase_data under `download_roots`) are certified against **rTorrent**. Gateway kinds are first-class in the UI and list merge; feature depth still varies by client API.
