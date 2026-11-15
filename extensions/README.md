# Extensions directory

Optional local package trees for host development (`extensions_dir` / install staging).

Official sources live in separate repos: `extensions/fleet-ext-*` (workspace) / [krate-apps/fleet-ext-*](https://github.com/orgs/krate-apps/repositories?q=fleet-ext-).

Build a worker in its own repo (`make build` / `make pack`). Install into Fleet via the Official bucket, a release archive, or a local path — do not symlink the monorepo into this directory.

## Catalogue

[fleet-official-bucket extensions.md](https://github.com/krate-apps/fleet-official-bucket/blob/main/extensions.md)

## Docs

- [Create an extension](../docs/developer/create-extension.md)
- [Publishing](../docs/developer/publishing-extensions.md)
- [Architecture](../docs/developer/architecture.md)

---

<p align="center">
  Developed by the <a href="https://runkrate.com">Krate</a> team.<br />
  Released under the MIT License.
</p>
