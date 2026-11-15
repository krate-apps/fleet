# Multi-client gateways

Fleet’s torrent authority for V1 remains **rTorrent**. Optional gateways register additional clients under **Settings → Clients**.

Gateway passwords are stored encrypted at rest (`enc:v1:` AES-GCM). The AES master key is resolved in order: systemd `CREDENTIALS_DIRECTORY/fleet-secrets.key` → Linux user keyring description `fleet-secrets` → `data_dir/.fleet-secrets.key` (`0600`). Legacy plaintext password rows remain readable and migrate on the next save. See the commented `LoadCredential=` line in [`examples/systemd/fleet@.service`](examples/systemd/fleet@.service).

## Supported kinds

- qBittorrent
- Deluge
- Transmission

## Behaviour

- The torrent list merges rows as `hash@clientId`.
- Toolbar start/stop/erase/reannounce and label edits target the selected row’s client.
- Capability chips show what each gateway supports.
- Detail tabs load files/trackers/peers when the client API allows; otherwise the UI shows an explicit “not supported” message (no dead panel).

## Logs

Per-client overrides on **Settings → Clients**:

- `journal_unit` — must stay on the allowlist (e.g. `deluge@$USER`)
- `log_path` — regular file under the Fleet process home

## Related extensions

- [Label Bridge](https://github.com/krate-apps/fleet-ext-labelbridge) — sync labels across clients
- [Cross-seed](https://github.com/krate-apps/fleet-ext-crossseed) — push completed downloads to other clients
- [Duplicate Finder](https://github.com/krate-apps/fleet-ext-duplicates) — cross-client duplicates

Fleet V1 certifies **rTorrent** only. Other gateway kinds are optional.
