# Create a bucket

A bucket is a Git repository of JSON listings that Fleet uses for discovery and install.

## Layout

```text
bucket.json                 # root (id, name, official?)
bucket/<id>.json            # one extension or theme listing per file
packages/…                  # optional local package trees
archives/…                  # optional cached .tar.gz assets
.github/workflows/          # uses krate-apps/fleet-bucket-action
```

Start from [`templates/bucket`](../../../templates/bucket/) (`fleet-template-bucket`). Copy `bucket/app-name.json.template` to `bucket/<id>.json`.

## Listing shape

Listings are JSON (`kind`: `extension` or `theme`). Remote installs use architecture entries:

```json
{
  "schema": 1,
  "kind": "extension",
  "id": "org.example.demo",
  "name": "Demo",
  "version": "1.0.0",
  "api": ">=1.0.0 <2.0.0",
  "homepage": "https://github.com/example/fleet-ext-demo",
  "architecture": {
    "linux-amd64": {
      "url": "https://github.com/example/fleet-ext-demo/releases/download/v1.0.0/org.example.demo-linux-amd64.tar.gz",
      "hash": "sha256:…"
    }
  },
  "checkver": { "github": "example/fleet-ext-demo" },
  "autoupdate": {
    "architecture": {
      "linux-amd64": {
        "url": "https://github.com/example/fleet-ext-demo/releases/download/v$version/org.example.demo-linux-amd64.tar.gz"
      }
    }
  },
  "source": { "path": "packages/demo" }
}
```

| Field | Notes |
| --- | --- |
| `source.path` | Relative to the bucket root (local/offline install) |
| `architecture.*.url` + `hash` | Remote `.tar.gz`; GitHub must be a **pinned** `releases/download/…` asset URL |
| `checkver` / `autoupdate` | Used by the excavator workflow (GitHub Releases) |

Never rely on a moving branch URL for default installs.

## Official vs community

- Official listings use `org.fleet.*` and skip the third-party consent gate.
- Community buckets set `official: false` in `bucket.json`.

Canonical Official catalogue: [fleet-official-bucket](https://github.com/krate-apps/fleet-official-bucket) (`buckets/fleet-official-bucket` in the workspace).  
Pin release hashes with [`fleet-bucket-action/pin`](https://github.com/krate-apps/fleet-bucket-action) (writes into that bucket repo’s `bucket/*.json`).

## Schemas

| File | Role |
| --- | --- |
| [bucket.schema.json](schemas/bucket.schema.json) | Root `bucket.json` |
| [bucket-app.schema.json](schemas/bucket-app.schema.json) | `bucket/*.json` listings |

## Validate / CI

Bucket CI is centralized in [`krate-apps/fleet-bucket-action`](https://github.com/krate-apps/fleet-bucket-action):

| Action | Role |
| --- | --- |
| `validate` | JSON listings, paths, hash format, pinned GitHub URLs |
| `hashcheck` | Download urls and verify sha256 |
| `excavate` | Bump version/url/hash from `checkver` + `autoupdate` |
| `pin` | Apply `SHA256SUMS` → listing hash (+ url from release tag) |

```yaml
- uses: krate-apps/fleet-bucket-action/validate@v1
```

Locally (Fleet workspace):

```bash
FLEET_BUCKET_ROOT=. python3 ../../actions/fleet-bucket-action/scripts/validate-listings.py
```

Pin `@v1` once the action repo is tagged; template/Official bootstrap with `@main` until then.
