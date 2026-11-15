# SDK

Public contracts for extensions, themes, and buckets. Machine-readable JSON Schema files live under [schemas/](schemas/). Go helpers used by the Fleet host (and by extension authors who import them) live under `sdk/` in the host repository (`sdk/manifest`, `sdk/ui`, `sdk/theme`, `sdk/backend`).

## Schemas

| File | Purpose |
| --- | --- |
| [manifest.schema.json](schemas/manifest.schema.json) | Extension package manifest (`fleet.ext.yaml`) |
| [ui-contribution.schema.json](schemas/ui-contribution.schema.json) | Declarative UI entry (`ui/extension.json`) |
| [ipc.schema.json](schemas/ipc.schema.json) | JSON-RPC 2.0 framing between Fleet and workers |
| [theme.schema.json](schemas/theme.schema.json) | Theme package manifest (`fleet.theme.yaml`) |
| [bucket.schema.json](schemas/bucket.schema.json) | Bucket root (`bucket.json`) |
| [bucket-app.schema.json](schemas/bucket-app.schema.json) | Bucket listing (`bucket/*.json`) |

Validate packages against these files. Go validation mirroring the contracts lives in `sdk/manifest`, `sdk/ui`, `sdk/theme`, and `internal/buckets`.

## Backend workers

Process workers speak **JSON-RPC 2.0** (one JSON object per line on stdin/stdout).

- Contract: [ipc.schema.json](schemas/ipc.schema.json)
- Go framing helpers: `sdk/backend` (`protocol.go`)
- Fleet supervises the child; a worker crash must not take down the core

Lifecycle methods include handshake, ping, shutdown, activate, deactivate, health, and upgrade. Workers may call reverse RPC on Fleet only for capabilities declared in `fleet.ext.yaml` (storage, HTTP fetch, torrents, files, jobs, tools, gateways, …).

### `process.exec`

Capability `process.exec` is **fail-closed**. Fleet refuses every binary until the operator configures:

- `security.process_exec_allowlist` — absolute paths (after symlink resolution), and/or
- `security.process_exec_dirs` — directories whose non-forbidden binaries are allowed

Shells and common interpreters stay denied even when listed. `http.fetch` also rejects private/loopback/metadata destinations (SSRF guard).

See [Create an extension](create-extension.md) and [config.example.yaml](../user/examples/config.example.yaml).

## Declarative UI (frontend)

Declarative UI contributions are the default: columns, actions, pages, panels, and forms.

- Contract: [ui-contribution.schema.json](schemas/ui-contribution.schema.json)
- Go validation: `sdk/ui`
- Fleet interprets JSON; **third-party JavaScript does not run** in the main app context

### Form fields

| Field / form key | Use |
| --- | --- |
| `kind: "text"` / `"number"` / `"boolean"` / `"url"` | Standard inputs |
| `kind: "textarea"` | Multiline DSL; optional `rows`; format in `help` |
| `kind: "select"` | Requires `options: [{ "value", "label" }]` |
| `help` | Short muted hint under the control |
| `loadMethod` | RPC on Configure mount; matching keys prefill |
| `submitMethod` | RPC for Apply; Fleet sends field values |

### Official first-party Vue

A small allowlist of **official** extensions may ship Configure UX as Vue components **bundled in the Fleet `web/` tree** (compiled into the embedded UI):

1. Extension id must be under `org.fleet.*` and listed in the shell allowlist.
2. Source lives in `web/src/` — never fetched or evaluated from the extension package on disk.
3. Third-party / community extensions remain declarative-only.

Packages still ship `ui/extension.json` and must not embed executable frontend code.

## API compatibility

Manifest `api` is a semver range for Fleet SDK compatibility (typical official packages: `>=1.0.0 <2.0.0`).

## Next steps

- [Create an extension](create-extension.md)
- [Create a bucket](create-bucket.md)
- [Publishing extensions](publishing-extensions.md)
