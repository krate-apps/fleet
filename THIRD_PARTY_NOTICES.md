# Third-party notices

Fleet itself is MIT (see `LICENSE`). This file inventories **direct and transitive** dependency licences observed from the local module/npm trees on 2026-11-11.

No copyleft (GPL/AGPL) runtime dependency was found in the Go module graph. Frontend production dependencies (`vue`, `vue-router`) are MIT; build-time tooling includes Apache-2.0 / BSD / ISC / MIT / Python-2.0 (`argparse` via eslint stack).

## Go modules (runtime + test graph)

| SPDX (classified) | Module |
|---|---|
| ISC | `github.com/coder/websocket@v1.8.12` |
| MIT | `github.com/dustin/go-humanize@v1.0.1` |
| BSD-3-Clause | `github.com/google/uuid@v1.6.0` |
| MIT | `github.com/mattn/go-isatty@v0.0.20` |
| MIT | `github.com/ncruces/go-strftime@v0.1.9` |
| BSD-3-Clause | `github.com/remyoudompheng/bigfft@…` |
| BSD-3-Clause | `golang.org/x/crypto@v0.28.0` |
| BSD-3-Clause | `golang.org/x/sys@v0.26.0` |
| MIT | `gopkg.in/yaml.v3@v3.0.1` |
| BSD-3-Clause | `modernc.org/sqlite@v1.34.5` and related `modernc.org/*` |
| BSD-3-Clause | `gopkg.in/check.v1` (test transitive) |
| Apache-2.0 | `github.com/google/pprof` (tooling transitive) |

Full machine-readable dump: `../docs/backup/packaging-manual/third-party-inventory.txt` (generated).

## npm (web)

Production:

| SPDX | Package |
|---|---|
| MIT | `vue` |
| MIT | `vue-router` |

Dev/build tooling is predominantly MIT/Apache-2.0/BSD/ISC. See inventory file for the complete list (157 packages observed in `web/node_modules`).

## Optional external helpers (not Go modules)

`mktorrent` / `dumptorrent` JSON forks are **optional** host binaries configured in `tools:`.
