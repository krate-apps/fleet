# Security Policy

## Supported versions

Security fixes are applied on the current `main` branch and published in the next host release. Older tags are not backported unless a release note says otherwise.

| Version | Supported |
| --- | --- |
| Latest GitHub Release on [krate-apps/fleet](https://github.com/krate-apps/fleet/releases) | Yes |
| Older tags / forks | No (please upgrade) |

## Reporting a vulnerability

**Do not** open a public GitHub issue for security-sensitive findings (RCE, auth bypass, SSRF, secret leakage, privilege escalation, etc.).

Prefer one of:

1. [GitHub private vulnerability reporting](https://github.com/krate-apps/fleet/security/advisories/new) on this repository (preferred when enabled).
2. A private message to the maintainers via the [Krate](https://runkrate.com) contact channels, with a clear subject such as `Fleet security report`.

Please include:

- Affected version / commit (or “built from main at &lt;sha&gt;”)
- Impact and attack prerequisites (auth required? local-only? reverse proxy?)
- Reproduction steps or a minimal proof of concept
- Whether you are already coordinating disclosure elsewhere

We aim to acknowledge reports within a few business days and to ship a fix or mitigation before any coordinated public write-up.

## Hardening checklist (operators)

Fleet is a self-hosted multi-client BitTorrent UI (rTorrent, qBittorrent, Deluge, Transmission). Treat the host, reverse proxy, and client APIs as part of the trust boundary.

- Bind to loopback by default; put TLS termination in a reverse proxy ([Reverse proxy](docs/user/reverse-proxy.md)).
- Keep `auth.disable: false` except for local debugging on loopback (Fleet refuses `auth.disable` on non-loopback listen addresses).
- Set `auth.secure_cookies: true` behind HTTPS (Fleet also sets `Secure` when `X-Forwarded-Proto: https`).
- Configure `security.process_exec_allowlist` / `process_exec_dirs` before any extension may use `process.exec` (fail-closed when empty).
- Authenticate extension webhooks with `Authorization: Bearer` or `X-Api-Key` only — the token must not appear in the URL ([Hooks](docs/developer/hooks.md)).
- Keep `data_dir` permissions tight; gateway passwords are `enc:v1:` AES-GCM. The master key is resolved from systemd `CREDENTIALS_DIRECTORY/fleet-secrets.key`, then the Linux user keyring (`fleet-secrets`), then `data_dir/.fleet-secrets.key` (`0600`). Prefer `LoadCredential=` on the unit when possible.
- Pin catalogue installs with `sha256` hashes; do not install unsigned archives from untrusted buckets.

## Related docs

- [Install](docs/user/install.md) · [Reverse proxy](docs/user/reverse-proxy.md) · [SDK](docs/developer/sdk.md) · [Hooks](docs/developer/hooks.md)
- [Contributing](CONTRIBUTING.md)
