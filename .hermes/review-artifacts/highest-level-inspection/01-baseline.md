# Inspection Baseline

Captured: 2026-07-13 20:31 GMT

## Source

- Repository: `/home/opc/src/sub2api`
- Branch: `feat/local-admin-bypass`
- Commit: `44613bfaf950b76d066f01cd763444b52f80f7c5`
- Tracked source: clean
- Untracked files: five `.hermes/plans/*.md` planning artifacts only

## Deployment

- Compose: `/opt/sub2api/docker-compose.local.yml`
- App image tag: `sub2api:single-user-private-control`
- App image ID: `sha256:e8904927e22a7859a7d5ef50a036e542f79b900274d060811bce83b509803e49`
- App image created: `2026-07-13T19:50:06.836490564Z`
- App started: `2026-07-13T19:50:12.989155583Z`
- Image size: 37,631,785 bytes
- `sub2api`, `sub2api-postgres`, and `sub2api-redis`: healthy

## Allowlisted mode values

```text
SUB2API_PUBLIC_HOST=sub2api.research.for-immortal.cn
SERVER_PORT=8080
SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true
```

No unrelated environment values were read or recorded.

## Resource snapshot

```text
sub2api:          CPU 0.57%, memory 58.95 MiB, 12 PIDs
sub2api-postgres: CPU 0.90%, memory 73.82 MiB, 19 PIDs
sub2api-redis:    CPU 0.18%, memory 13.05 MiB, 7 PIDs
root filesystem:  183 GiB total, 91 GiB used, 50%
```

## Source-to-image note

The Docker build reports that Git commit information was not captured. The image was built after commit `d0ac8ea`; commit `44613bf` only changes the audit document and does not alter runtime code. Runtime/source traceability remains a review finding because the image itself does not carry a verifiable commit label.
