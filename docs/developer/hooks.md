# Public webhook hooks

Fleet may expose auth-exempt ingest routes for active extensions:

```text
POST /api/v1/hooks/{extensionId}
```

Authenticate with the extension’s stored `webhookToken` using **one** of:

- `Authorization: Bearer <webhookToken>`
- `X-Api-Key: <webhookToken>`

Core verifies the secret, then calls `ext.<short>.ingest` on the worker. The token must **not** appear in the URL (access logs / `Referer`).

Examples:

| Extension ID | Worker method |
| --- | --- |
| `org.fleet.servarrbridge` | `ext.servarrbridge.ingest` |
| `org.fleet.nzbbridge` | `ext.nzbbridge.ingest` |

Workers enqueue via `fleet.intake.add` when they need the default client. NZB Bridge also uses host `fleet.tools.mktorrent` (capability `tools.mktorrent`) before intake.
