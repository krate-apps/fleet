# Contributing to Fleet

Thanks for helping improve Fleet. This guide covers the **host** repository ([krate-apps/fleet](https://github.com/krate-apps/fleet)). Extensions, themes, and the official bucket live in sibling repos under the [krate-apps](https://github.com/krate-apps) org — see [Create an extension](docs/developer/create-extension.md) and [Publishing](docs/developer/publishing-extensions.md).

## Before you start

1. Read [Architecture](docs/developer/architecture.md) and [Build](docs/developer/build.md).
2. Search existing [issues](https://github.com/krate-apps/fleet/issues) and discussions before opening a new one.
3. For **security** findings, follow [SECURITY.md](SECURITY.md) — do not file public issues for exploitable bugs.

Large or cross-cutting changes (new RPC capabilities, auth model, packaging layout) are easier to review if you open an issue or draft PR first.

## Development setup

Requirements: Go 1.22+, Node.js 22+ (frontend build only). Full UI smoke is easiest with an rTorrent SCGI endpoint; gateway kinds (qBittorrent / Deluge / Transmission) are covered under [Clients](docs/user/clients.md).

```bash
git clone https://github.com/krate-apps/fleet.git
cd fleet
go mod tidy
cd web && npm install && npm run build && cd ..
make embed build
./bin/fleet -config docs/user/examples/config.example.yaml
```

Open http://127.0.0.1:8080/ — health: `GET /api/v1/health`.

```bash
make test      # unit + web typecheck + script smokes
make certify   # broader local gate
```

Path-mode UI builds: `FLEET_UI_BASE=/your/prefix/ make embed build` ([Reverse proxy](docs/user/reverse-proxy.md)).

## What we accept

| Kind        | Notes                                                                   |
| ----------- | ----------------------------------------------------------------------- |
| Bug fixes   | Prefer a failing test or clear reproduction                             |
| Docs / i18n | English product docs under `docs/`; keep tone factual                   |
| UX polish   | Match existing Vue patterns in `web/`                                   |
| Features    | Align with the public SDK contracts (`docs/developer/schemas/`, `sdk/`) |
| Extensions  | Prefer shipping in a dedicated `fleet-ext-*` repo + bucket listing      |

Fleet is a **multi-client UI**, not a BitTorrent engine. Changes that move torrent authority into Fleet SQLite are out of scope for V1.

## Pull requests

1. Branch from `main` with a short descriptive name (`fix/…`, `feat/…`, `docs/…`).
2. Keep the diff focused — one coherent change per PR when practical.
3. Include tests for behaviour changes under `internal/` / `web/` as appropriate.
4. Do not commit secrets, lab paths, or workspace-only CDC/parity notes (`docs/` here is the **published** English tree only).
5. Fill the PR description with summary + test plan. **No** trailers (`Co-authored-by`, `Signed-off-by`, `Made-with`, AI attribution).

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/) with a **mandatory body** after a blank line (explain *why*, not only what).

Allowed types:

```text
build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test, ux, i18n
```

Examples:

```text
fix: require process.exec allowlist

Fail closed when security.process_exec_allowlist and process_exec_dirs
are empty so extensions cannot run arbitrary host binaries by default.
```

Breaking changes: put `!` before `:` in the subject (for example `feat!: …`). Do **not** add a `BREAKING CHANGE:` footer.

Prefer small atomic commits over monoliths (backend / UI / docs / i18n split when distinct).

## Code guidelines

- **Go:** match existing packages under `internal/`; keep extension capabilities fail-closed where security-sensitive (`process.exec`, SSRF-safe `http.fetch`, fsops containment).
- **Web:** Vue 3 + TypeScript in `web/`; follow surrounding component style; no third-party JS in the main shell from extension packages.
- **Docs:** English only in this repo’s `docs/`; never mention private lab topology in published guides.
- **License:** MIT — by contributing you agree your changes are licensed under the same terms as [LICENSE](LICENSE).

## Related repositories

| Repo                                                                         | Role                                              |
| ---------------------------------------------------------------------------- | ------------------------------------------------- |
| [fleet](https://github.com/krate-apps/fleet)                                 | Host (this repo)                                  |
| [fleet-official-bucket](https://github.com/krate-apps/fleet-official-bucket) | Official catalogue                                |
| [fleet-template-ext](https://github.com/krate-apps/fleet-template-ext)       | Extension template                                |
| [fleet-bucket-action](https://github.com/krate-apps/fleet-bucket-action)     | Bucket CI (`validate` / `hashcheck` / `excavate`) |

## Dependency updates

Dependabot opens weekly grouped PRs for Go modules, `web/` npm packages, and GitHub Actions (see [`.github/dependabot.yml`](.github/dependabot.yml)). Review CI on those PRs like any other change.

## Questions

Open a GitHub issue for product questions, or see [FAQ](docs/user/faq.md) and the [docs index](docs/README.md).
