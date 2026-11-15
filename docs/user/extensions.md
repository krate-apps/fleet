# Extensions

Extensions add automation, notifications, file tools, and integrations without forking Fleet core.

## Install and activate

1. Open **Extensions**.
2. Manage **buckets** (catalogue sources). The Official bucket (`org.fleet.official`) can be disabled anytime.
3. Install from the catalogue, a local `.tar.gz` + sha256, or a pinned GitHub release URL.
4. Activate or deactivate discovered packages.

Third-party extensions ask for explicit consent: they run as processes under your Unix account and are **not** a strong sandbox. Official `org.fleet.*` packages skip that gate.

## Official catalogue

Browse the full list in [fleet-official-bucket/extensions.md](https://github.com/krate-apps/fleet-official-bucket/blob/main/extensions.md).

Common packages (listing file → id):

| Listing | ID | Role |
| --- | --- | --- |
| `bucket/org.fleet.rssfeeds.json` | `org.fleet.rssfeeds` | Poll RSS/Atom and enqueue magnets |
| `bucket/org.fleet.rssrewrite.json` | `org.fleet.rssrewrite` | Shared URL rewrite rules for RSS |
| `bucket/org.fleet.autodownload.json` | `org.fleet.autodownload` | Watch folders + IRC announces |
| `bucket/org.fleet.notifications.json` | `org.fleet.notifications` | ntfy / Gotify / Slack / Pushover / email |
| `bucket/org.fleet.servarrbridge.json` | `org.fleet.servarrbridge` | Sonarr/Radarr/Prowlarr/Lidarr webhooks |
| `bucket/org.fleet.nzbbridge.json` | `org.fleet.nzbbridge` | SABnzbd/NZBGet → torrent intake |
| `bucket/org.fleet.traffic.json` | `org.fleet.traffic` | Day/month/year + per-tracker totals |

Each repository README links the matching listing under `bucket/org.fleet.<slug>.json`.

## Local development layout

| Piece | Location |
| --- | --- |
| Catalogue | `buckets/fleet-official-bucket` ([GitHub](https://github.com/krate-apps/fleet-official-bucket)) |
| Extension sources | `extensions/fleet-ext-<slug>` (separate repos) |

Build and pack in each extension repo (`make build` / `make pack`). Install via the Official bucket, a release archive, or a local package path.

## Coming from another client

See [Coming from Deluge or ruTorrent](coming-from.md).
