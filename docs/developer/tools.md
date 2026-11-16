# Tools

Long-running helpers surface as Fleet jobs with progress.

## Jobs reverse RPC

Workers with `jobs.schedule` / `jobs.read` may call `fleet.jobs.create`, `fleet.jobs.update`, `fleet.jobs.get`, and `fleet.jobs.cancel`. Jobs are scoped to the calling extension id.

## mktorrent / dumptorrent

Fleet ships reference CLIs:

| Binary | Role |
| --- | --- |
| `bin/fleet-dumptorrent` | Dump `.torrent` as JSON (`--json` / `-j` / `-J`) |
| `bin/fleet-mktorrent` | Create `.torrent` with JSON summary |

Build: `make tools-json` (also pulled in by `make build`).

```yaml
tools:
  mktorrent: /opt/$USER/fleet/tools/fleet-mktorrent
  dumptorrent: /opt/$USER/fleet/tools/fleet-dumptorrent
```

Other helpers (`unrar`, `mediainfo`, `ffmpeg`, `sox`, `rclone`) are optional path entries under `tools:` and are path-guarded by `download_roots` where applicable.

Operators can also set absolute paths in **Settings → Tools**. UI values are stored in SQLite and overlay `config.yaml` (empty field = YAML, then PATH). `GET/PUT /api/v1/settings/tools` applies changes live; changing `rclone` reloads active extension workers so `FLEET_TOOL_RCLONE` updates.

HTTP APIs: `POST /api/v1/tools/dumptorrent`, `POST /api/v1/tools/mktorrent` (job), `GET /api/v1/tools/status`. Extensions with `tools.mktorrent` may call `fleet.tools.mktorrent` over reverse RPC.
