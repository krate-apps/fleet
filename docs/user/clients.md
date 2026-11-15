# Clients

Fleet is a **multi-client** control plane: one UI over several BitTorrent engines. Torrents stay owned by each client — Fleet never becomes the engine.

## Supported clients

| Client | How it connects |
| --- | --- |
| **qBittorrent** | **Settings → Clients** (WebUI/API) |
| **Deluge** | **Settings → Clients** (WebUI/API) |
| **Transmission** | **Settings → Clients** (RPC/Web) |
| **rTorrent** | `rtorrent.address` in config (SCGI Unix socket or TCP) |

Register as many clients as you need. The torrent table merges rows as `hash@clientId`; toolbar actions and label edits target the selected row’s client. Capability chips show what each kind supports; detail tabs that a client cannot provide show an explicit “not supported” message.

### Path-guarded features

The Files browser, erase_data, public shares, and similar path checks use `rtorrent.download_roots` when you use rTorrent. Mount or list those directories accordingly. Other clients keep their own download paths; feature depth still varies by client API.

Gateway passwords are stored encrypted at rest (`enc:v1:` AES-GCM). The AES master key is resolved in order: systemd `CREDENTIALS_DIRECTORY/fleet-secrets.key` → Linux user keyring description `fleet-secrets` → `data_dir/.fleet-secrets.key` (`0600`). Legacy plaintext password rows remain readable and migrate on the next save. See the commented `LoadCredential=` line in [`examples/systemd/fleet@.service`](examples/systemd/fleet@.service).

## Logs

Per-client overrides on **Settings → Clients**:

- `journal_unit` — must stay on the allowlist (e.g. `deluge@$USER`)
- `log_path` — regular file under the Fleet process home

## Related extensions

- [Label Bridge](https://github.com/krate-apps/fleet-ext-labelbridge) — sync labels across clients
- [Cross-seed](https://github.com/krate-apps/fleet-ext-crossseed) — push completed downloads to other clients
- [Duplicate Finder](https://github.com/krate-apps/fleet-ext-duplicates) — cross-client duplicates
