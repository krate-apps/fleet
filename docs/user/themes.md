# Themes

## Built-in

Fleet ships **dark** and **light** as core appearance options under **Settings → Appearance**.

## Installable packs

Community / official packs register via `GET /api/v1/themes` after install under `<data_dir>/themes-src/`.

| Theme | ID | Repository |
| --- | --- | --- |
| Slate | `org.fleet.theme.slate` | [fleet-theme-slate](https://github.com/krate-apps/fleet-theme-slate) |
| Ocean | `org.fleet.theme.ocean` | [fleet-theme-ocean](https://github.com/krate-apps/fleet-theme-ocean) |
| Sand | `org.fleet.theme.sand` | [fleet-theme-sand](https://github.com/krate-apps/fleet-theme-sand) |

Official bucket listings: `bucket/org.fleet.theme.slate.json`, `bucket/org.fleet.theme.ocean.json`, `bucket/org.fleet.theme.sand.json`.

Each pack is a `fleet.theme.yaml` + CSS. Editorial source of truth is the `themes/fleet-theme-*` repositories (workspace `themes/`). Listings live in the Official bucket under `bucket/<id>.json`.

Start a new pack from [`fleet-template-theme`](https://github.com/krate-apps/fleet-template-theme) (workspace mirror: `templates/theme`).
