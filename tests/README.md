# Fleet tests beyond unit packages

| Path | Purpose |
|---|---|
| `internal/**` (`go test`) | Unit / package tests (CI) |
| `tests/e2e/` | Black-box HTTP checks against a running Fleet |
| `tests/load/` | Smoke load / reconnect helpers for PERF notes |

Unit tests remain the default gate (`make test`). E2E and load scripts are **opt-in** and expect a local process (and often `auth.disable: true` or a valid session).

```bash
make test          # units + web typecheck
make perf-cache    # sync cache benchmarks
make e2e-smoke     # starts nothing; hits BASE_URL
make load-smoke    # HTTP polling smoke
```

---

<p align="center">
  Developed by the <a href="https://runkrate.com">Krate</a> team.<br />
  Released under the MIT License.
</p>
