# JDownloader

Monitor and control a **MyJDownloader** download queue from Fleet.

`org.fleet.jdownloader` · official Fleet extension · SDK worker package

## Configuration

Turn this extension **on/off** with **Extensions → Activate / Deactivate**. Apply saves MyJD credentials (email, password, optional device name). The Configure UI shows the live download queue with pause / resume / remove.

Secrets stay in extension storage and are never returned to the UI after save (only a `passwordConfigured` flag).

## Install

Requires a running [Fleet](https://github.com/krate-apps/fleet) host.

### From the Official bucket

1. Enable the **Fleet Official** bucket if needed.
2. **Extensions** → `org.fleet.jdownloader` → Install → Configure.

## Package layout

```text
fleet-ext-jdownloader/
  fleet.ext.yaml
  ui/extension.json
  bin/jdownloader-worker
  cmd/
```

## Pack / release

```bash
make pack   # dist/<id>-linux-amd64.tar.gz + dist/SHA256SUMS
```
