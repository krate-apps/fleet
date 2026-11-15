# Public webhook hooks

Fleet may expose auth-exempt ingest routes for active extensions:

```text
POST /api/v1/hooks/{extensionId}/{token}
```

Authentication (any one may succeed):

1. **Preferred:** `Authorization: Bearer <webhookToken>` or `X-Api-Key: <webhookToken>`
2. **Compatibility:** path segment `{token}` (Servarr/NZB-style URLs)

The secret must match the extension’s stored `webhookToken`. Core verifies it, then calls `ext.<short>.ingest` on the worker. Path-only auth still works but Fleet logs a warning — path tokens appear in access logs and `Referer`.

Examples:

| Extension ID | Worker method |
| --- | --- |
| `org.fleet.servarrbridge` | `ext.servarrbridge.ingest` |
| `org.fleet.nzbbridge` | `ext.nzbbridge.ingest` |

Workers enqueue via `fleet.intake.add` when they need the default client. NZB Bridge also uses host `fleet.tools.mktorrent` (capability `tools.mktorrent`) before intake.
