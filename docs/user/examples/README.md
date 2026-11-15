# Install examples

Reference files for operators. Prefer copying from here rather than inventing paths under `packaging/`.

| File | Purpose |
| --- | --- |
| [config.example.yaml](config.example.yaml) | Seed `~/.config/fleet/config.yaml` |
| [systemd/fleet@.service](systemd/fleet@.service) | systemd template unit |
| [caddy/](caddy/) | Caddy path-mode and subdomain |
| [nginx/](nginx/) | nginx path-mode and subdomain |
| [apache/](apache/) | Apache path-mode and VirtualHost |

See [Install](../install.md) and [Reverse proxy](../reverse-proxy.md).

---

<p align="center">
  Developed by the <a href="https://runkrate.com">Krate</a> team.<br />
  Released under the MIT License.
</p>
