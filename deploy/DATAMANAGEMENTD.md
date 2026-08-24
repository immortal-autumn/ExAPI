# datamanagementd deployment guide (Data Management)

English (default) | [简体中文](DATAMANAGEMENTD_CN.md)

This guide explains how to deploy `datamanagementd` on the host and connect it
to the main process to enable **Data Management**.

## 1. Constraints

- The main process always probes `/tmp/sub2api-datamanagement.sock`.
- The admin Data Management feature is enabled only when that Unix socket is
  reachable and its `Health` call succeeds.
- `datamanagementd` persists metadata in SQLite and does not depend on the main
  PostgreSQL database.

## 2. Build and run on the host

```bash
cd /opt/sub2api-src/datamanagement
go build -o /opt/sub2api/datamanagementd ./cmd/datamanagementd

mkdir -p /var/lib/sub2api/datamanagement
chown -R sub2api:sub2api /var/lib/sub2api/datamanagement
```

Manual start example:

```bash
/opt/sub2api/datamanagementd \
  -socket-path /tmp/sub2api-datamanagement.sock \
  -sqlite-path /var/lib/sub2api/datamanagement/datamanagementd.db \
  -version 1.0.0
```

## 3. Run with systemd (recommended)

The repository provides `deploy/sub2api-datamanagementd.service`:

```bash
sudo cp deploy/sub2api-datamanagementd.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now sub2api-datamanagementd
sudo systemctl status sub2api-datamanagementd
```

Follow the logs:

```bash
sudo journalctl -u sub2api-datamanagementd -f
```

The one-click installer can install an existing binary or build from source:

```bash
# Option 1: install an existing binary
sudo ./deploy/install-datamanagementd.sh --binary /path/to/datamanagementd

# Option 2: build from source, then install
sudo ./deploy/install-datamanagementd.sh --source /path/to/sub2api
```

## 4. Docker integration

When `sub2api` runs in Docker, mount the host socket at the same path inside the
container:

```yaml
services:
  sub2api:
    volumes:
      - /tmp/sub2api-datamanagement.sock:/tmp/sub2api-datamanagement.sock
```

Maintain this mount in `docker-compose.override.yml` so the main Compose file
does not need to be overwritten.

## 5. Dependencies

`datamanagementd` requires the following tools when it executes backups:

- `pg_dump`
- `redis-cli`
- `docker` (only when `source_mode=docker_exec`)

A missing dependency causes the corresponding task to fail; the task details
include the error.
