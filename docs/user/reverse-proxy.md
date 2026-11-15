# Reverse proxy

Fleet binds to loopback by default (`127.0.0.1:8080`). Put Caddy, nginx, or Apache in front for TLS and public URLs.

## Path mode vs subdomain

| Mode | UI build | Public URL |
| --- | --- | --- |
| **Subdomain / dedicated host** | default (`FLEET_UI_BASE=/`) | `https://fleet.example.com/` |
| **Path prefix** | `FLEET_UI_BASE=/fleet/` (trailing slash) | `https://example.com/fleet/` |

Path mode requires a rebuild so asset and API links stay under the strip prefix. The proxy must:

1. Redirect `/fleet` → `/fleet/`
2. Strip the prefix before the upstream
3. Forward `X-Forwarded-Proto`, and for path mode also `X-Forwarded-Prefix` / `X-Forwarded-PathBase`
4. Support WebSockets on `/api/v1/events/ws` (long-lived connection)

Set `auth.secure_cookies: true` behind HTTPS. Fleet also marks session cookies
`Secure` automatically when the request carries `X-Forwarded-Proto: https`
(or terminates TLS on the Fleet process itself).

## Examples

Full snippets live under [`examples/`](examples/):

| Proxy | Path mode | Subdomain |
| --- | --- | --- |
| **Caddy** | [examples/caddy/path-mode.caddy](examples/caddy/path-mode.caddy) | [examples/caddy/subdomain.caddy](examples/caddy/subdomain.caddy) |
| **nginx** | [examples/nginx/path-mode.conf](examples/nginx/path-mode.conf) | [examples/nginx/subdomain.conf](examples/nginx/subdomain.conf) |
| **Apache** | [examples/apache/path-mode.conf](examples/apache/path-mode.conf) | [examples/apache/subdomain.conf](examples/apache/subdomain.conf) |

### Build for path mode

```bash
FLEET_UI_BASE=/fleet/ make embed build
# then install / restart the fleet unit
```

Replace `/fleet` in the snippets with your prefix. Keep the trailing slash in `FLEET_UI_BASE`.

### Upstream

Examples assume Fleet at `http://127.0.0.1:8080`. Match `listen.host` / `listen.port` in your config ([examples/config.example.yaml](examples/config.example.yaml)).

## systemd

Unit template: [examples/systemd/fleet@.service](examples/systemd/fleet@.service).

```bash
# as root, after install.sh (or copy the unit yourself)
systemctl enable --now fleet@$USER
```

`ProtectHome=read-only` with `ReadWritePaths` for Fleet data dirs. Add each `rtorrent.download_roots` path under `ReadWritePaths` when erase_data / Files need write access outside those dirs.
