# Publishing extensions

## Target model

1. Each `fleet-ext-<slug>` / `fleet-theme-<id>` publishes its own GitHub Release with a reproducible `.tar.gz` (scaffolds: [`fleet-template-ext`](https://github.com/krate-apps/fleet-template-ext), [`fleet-template-theme`](https://github.com/krate-apps/fleet-template-theme)).
2. `fleet-official-bucket` listings pin `architecture.linux-amd64.url` + `hash` per package.
3. Never point default installs at a moving branch URL.

## Checklist

1. Validate manifests against [docs/developer/schemas/](schemas/) (including `theme.schema.json`).
2. In the extension repo: `make pack` (scaffold from [`fleet-template-ext`](https://github.com/krate-apps/fleet-template-ext)) → `dist/<id>-linux-amd64.tar.gz` + `dist/SHA256SUMS`.
3. Publish a GitHub Release asset (template workflow uploads on `v*` tags).
4. Pin the bucket listing (`bucket/<id>.json`) with [`fleet-bucket-action/pin`](https://github.com/krate-apps/fleet-bucket-action).
5. Keep `homepage` / `source.repository` → `https://github.com/krate-apps/fleet-ext-<slug>` (or theme equivalent).

## Pin bucket listings

From a bucket checkout (or the Official workspace tree), after packing an extension:

```bash
FLEET_BUCKET_ROOT=. PIN_SUMS=/path/to/extension/dist/SHA256SUMS PIN_TAG=vX.Y.Z \
  python3 ../../actions/fleet-bucket-action/scripts/pin-listings.py
```

Or in CI:

```yaml
- uses: krate-apps/fleet-bucket-action/pin@v1
  with:
    path: .
    sums: /path/to/SHA256SUMS
    tag: vX.Y.Z
```

`pin` writes `architecture.linux-amd64.hash` (and `url` when `tag` is set) using `checkver.github` or a GitHub `homepage` / `source.repository` on the listing.

After assets are on GitHub Releases, the excavator can also bump version/url/hash on a schedule.

## Host artefacts

Publishing core tags and release assets is a separate host process — do not push or create GitHub releases unless explicitly asked.
