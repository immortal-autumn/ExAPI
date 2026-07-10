# ExAPI Upstream Compatibility Notes

ExAPI is a fork of `Wei-Shaw/sub2api`. The product-facing name is ExAPI, but a number of internal identifiers intentionally remain `sub2api` in the first rebrand pass.

## Intentionally retained identifiers

- Go module path: `github.com/Wei-Shaw/sub2api`.
- Generated Go imports under `backend/ent/**` and internal packages.
- Binary path: `/app/sub2api`.
- Systemd unit filename and runtime paths: `sub2api.service`, `/opt/sub2api`, `/etc/sub2api`.
- Docker user/group and service names: `sub2api`.
- Database defaults such as DB name `sub2api`.
- Redis/cache/localStorage keys that use `sub2api` prefixes.
- WebSocket subprotocols such as `sub2api-admin`.
- Historical compatibility references such as `sub2apipay`.

These names are kept to avoid breaking existing deployments, volumes, databases, browser storage, and generated code.

## Safe first-pass rebrand scope

The first ExAPI rebrand changes:

- visible UI defaults and browser titles;
- onboarding/setup copy;
- backend startup/setup visible strings;
- Docker labels and deployment documentation;
- README presentation and product direction.

## Full artifact rename requires a migration plan

Do not blindly replace `sub2api` with `exapi`. A full runtime rename should be a separate migration with backups and rollback instructions for:

- GitHub repository/module path;
- systemd unit name;
- Docker image/service/container names;
- binary path;
- Linux user/group;
- data/config directories;
- database name/user;
- Redis/cache/browser storage prefixes.
