# pyLoad

Monitor and control a **local pyLoad** download queue from Fleet.

`org.fleet.pyload` · official Fleet extension · SDK worker package

## Configuration

Activate the extension, then set base URL (loopback or private LAN only), username, and password. The Configure UI polls the queue and exposes pause / resume / abort / restart / delete.

The worker dials pyLoad directly (not via `fleet.http.fetch`) because Fleet’s SSRF guard blocks localhost.

## Install

Requires a running [Fleet](https://github.com/krate-apps/fleet) host and a reachable pyLoad web API.

## Pack / release

```bash
make pack
```
