# User interface

## Sign in

- First visit on a new instance: create the administrator account (no factory password).
- Afterwards: sign in with that username and password.
- Sign out from **Settings → Account**.

## Torrents

- The main table merges torrents from every connected client (rTorrent + gateways); rows are keyed as `hash@clientId` — see [Clients](clients.md).
- Browse, select, start/stop/hash-check/erase from the main table (actions target the row’s client).
- Add magnets or `.torrent` files from the toolbar, or **drag-and-drop** a `.torrent` / magnet onto the list.
- Open a torrent for files, trackers, peers (optional country codes when `geoip.database` is set; editable peer notes), and **chunks**.
- **Download .torrent** fetches the session/tied source; each file row can download the on-disk payload (requires `download_roots`).
- Details show ↓ ETA, ↑ ETA, and Seeded duration when `finishedAt` is known.
- Filter, sort, and search preferences persist in browser `localStorage`.
- Themes live under **Settings → Appearance** — see [Themes](themes.md).

## RSS mode feeds (core)

**Settings → RSS feeds** lists mode feeds (`all`, `downloading`, `completed`, …) as absolute URLs for readers.

- Advertised absolute `url` can be overridden with public host, HTTP/HTTPS ports, prefer HTTPS, and path prefix.
- Leave those fields empty to keep auto detection from the reverse proxy (`Host`, `X-Forwarded-Proto`, `X-Forwarded-Prefix`).
- Readers must send the Fleet session cookie (same origin) unless auth is disabled.

Inbound RSS polling is provided by extension `org.fleet.rssfeeds` — see [Extensions](extensions.md).

## Files

Open **Files** in the top bar (requires `rtorrent.download_roots`):

- Browse directories under configured roots (dotfiles hidden; no symlink traversal).
- Multi-select Move / Copy / Delete (delete asks for confirmation).
- Preview images, bounded videos (≤100 MiB), and `.nfo` text; download a selected file.
- Create a **Share** link with TTL and max downloads (`/api/v1/share/{token}` — no login); revoke from the list.

Optional File Manager / Media / Share extensions add bulk RPC façades and richer tools.

## Status bar

Shows engine state, torrent count, first-root **disk free**, host **load**, and transfer rates.

## Keyboard shortcuts (desktop)

| Key | Action |
| --- | --- |
| `↑` / `↓` or `j` / `k` | Move focus in the filtered list |
| `Space` | Toggle selection on the focused row |
| `a` | Open Add torrent |
| `/` | Focus search |
| `s` / `p` | Start / pause selected |
| `Delete` / `Backspace` | Remove selected (confirmation) |
| `?` | Show shortcuts help |

Shortcuts are ignored while typing in inputs or when a modal is open.

## Jobs

Long work (for example creating a torrent) appears as a job with progress. Helpers are configured under `tools:` — see [Tools](../developer/tools.md).

## Logs

**Settings → Logs** aggregates recent lines from the Fleet ring buffer, allowlisted journald units (`fleet@<user>`, `rtorrent@<user>`, gateway units), and optional per-client log paths.
