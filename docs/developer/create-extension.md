# Create an extension

Start from [`fleet-template-ext`](https://github.com/krate-apps/fleet-template-ext) (workspace mirror: `templates/extension`), or copy a small official package such as `fleet-ext-postaddcheck`.

## Package layout

```text
my-ext/
  fleet.ext.yaml          # manifest
  ui/extension.json       # declarative UI (optional)
  bin/my-worker           # JSON-RPC worker over stdin/stdout (optional)
  cmd/…                   # Go sources
  scripts/pack.sh         # release archive helper (from template)
  Makefile                # build / test / pack
  go.mod
  LICENSE
```

Manifest essentials: `id`, `name`, `version`, `api`, optional `frontend` / `backend`, `capabilities`, `engines`. Validate against [manifest.schema.json](schemas/manifest.schema.json).

Official packages use ids under `org.fleet.*`. Community packages must use a different namespace and will hit the third-party consent gate.

## Backend worker

- Protocol: JSON-RPC 2.0 on stdin/stdout ([ipc.schema.json](schemas/ipc.schema.json); helpers in `sdk/backend`)
- Lifecycle: handshake / activate / deactivate / health / shutdown
- Reverse RPC from Fleet may expose storage, HTTP fetch, torrent add/list/action/setLabel, files, throttle, jobs, tools — only what the manifest allows
- If you need `process.exec`, declare the capability **and** document the absolute binaries operators must add under `security.process_exec_allowlist` / `process_exec_dirs` (empty config denies all exec)

Minimal template worker answers lifecycle methods and a sample `ext.template.getSettings` / `saveSettings` pair. Full narrative: [SDK → Backend workers](sdk.md#backend-workers).

## Declarative UI

Prefer `loadMethod` / `submitMethod` for settings forms ([ui-contribution.schema.json](schemas/ui-contribution.schema.json)). Action-only forms keep custom handlers. See [SDK → Declarative UI](sdk.md#declarative-ui-frontend).

## README configuration section

Document every settings field and global action in the package `README.md` under **Configuration**. Official packages generate that block from `ui/extension.json`:

```bash
make docs-config
# python3 scripts/gen-readme-config.py .
```

The script upserts markdown between `<!-- fleet-config:start -->` and `<!-- fleet-config:end -->` (Install / Pack / footer stay untouched). Skip documenting a master Enabled checkbox — **Extensions → Activate** is the only on/off. Optional first-party UI notes live in `scripts/readme-config-extras/<extensionId>.md` next to the generator in the template.

## Local install into Fleet

1. `make build` in the extension repo
2. Stage under `~/.local/share/fleet/staging-*` (or install via UI Direct install)
3. `POST /api/v1/extensions/install` with `{ "source": "local", "path": "…" }`

Note: systemd `PrivateTmp` hides host `/tmp` from the Fleet process — avoid relying on `/tmp` for staging.

## Next steps

- Webhooks: [Hooks](hooks.md)
- Jobs / tools: [Tools](tools.md)
- Catalogue listing: [Create a bucket](create-bucket.md)
- Release: [Publishing extensions](publishing-extensions.md)
