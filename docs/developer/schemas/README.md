# JSON Schema contracts

Canonical machine-readable contracts for Fleet packages. Start from the [SDK](../sdk.md) guide for narrative docs.

| File | Purpose |
| --- | --- |
| [manifest.schema.json](manifest.schema.json) | Extension package manifest (`fleet.ext.yaml`) |
| [ui-contribution.schema.json](ui-contribution.schema.json) | Declarative UI entry (`ui/extension.json`) |
| [ipc.schema.json](ipc.schema.json) | JSON-RPC 2.0 framing between Fleet and workers |
| [theme.schema.json](theme.schema.json) | Theme package manifest (`fleet.theme.yaml`) |
| [bucket.schema.json](bucket.schema.json) | Bucket root (`bucket.json`) |
| [bucket-app.schema.json](bucket-app.schema.json) | Bucket listing (`bucket/*.json`) |

Go validation mirroring these contracts lives in `sdk/manifest`, `sdk/ui`, `sdk/theme`, and `internal/buckets`.

---

<p align="center">
  Developed by the <a href="https://runkrate.com">Krate</a> team.<br />
  Released under the MIT License.
</p>
