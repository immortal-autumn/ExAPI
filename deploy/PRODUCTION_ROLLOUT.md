# ExAPI production rollout

Use this runbook for every production promotion. Production stays unchanged
until the source, backup, restore, and canary gates all pass.

## 1. Fix the release identity

Record the reviewed Git commit and the published image digest. Use only an
immutable reference:

```bash
export EXAPI_IMAGE='ghcr.io/immortal-autumn/sub2api2personal@sha256:REPLACE_WITH_REVIEWED_DIGEST'
docker pull "$EXAPI_IMAGE"
docker image inspect "$EXAPI_IMAGE" \
  --format '{{ index .Config.Labels "org.opencontainers.image.revision" }} {{ index .Config.Labels "org.opencontainers.image.source" }}'
```

The revision must equal the reviewed commit and the source must identify the
ExAPI fork. Do not promote a mutable tag.

## 2. Pass source and artifact gates

Require all repository CI jobs, the production dependency audit, release
contract, checksum/provenance checks, and image build to pass for that exact
commit. Keep the test output and image digest with the rollout record.

## 3. Back up and prove restore

Before starting a new image:

1. Create an encrypted application backup and an independent PostgreSQL
   snapshot.
2. Copy `/etc/sub2api.env` or the Compose `.env` through a separate protected
   channel. Database backups cannot recover the external data, gateway-digest,
   or backup roots.
3. Record checksums and retention locations without recording secret values.
4. Restore the snapshot into an isolated canary database and verify users,
   groups, API keys, subscriptions, usage totals, and estimated costs.

Do not treat “backup completed” as a gate; a successful restore is the gate.

## 4. Run an isolated canary

Use the canonical named-volume Compose file in a separate project. The canary
names and port are deliberately distinct from production:

```bash
export COMPOSE_PROJECT_NAME=exapi-canary
export EXAPI_CONTAINER_NAME=exapi-canary
export EXAPI_POSTGRES_CONTAINER_NAME=exapi-canary-postgres
export EXAPI_REDIS_CONTAINER_NAME=exapi-canary-redis
export BIND_HOST=127.0.0.1
export SERVER_PORT=18080
export EXAPI_IMAGE='ghcr.io/immortal-autumn/sub2api2personal@sha256:REPLACE_WITH_REVIEWED_DIGEST'

docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml pull
docker compose -f deploy/docker-compose.yml up -d
```

Restore only into the canary project volumes. A clone of production encrypted
data requires protected copies of the same external data and gateway-digest
roots; never generate replacement roots for that clone. Block outbound traffic
until notification targets and upstream credentials have been scrubbed or
replaced with canary-only values.

## 5. Verify behavior

Both probes must return JSON, not SPA HTML:

```bash
curl --fail --show-error http://127.0.0.1:18080/health
curl --fail --show-error http://127.0.0.1:18080/ready
```

Then verify login, admin access, API-key authentication, quota enforcement,
usage/cost recording, export, and one non-destructive request per enabled
provider. Hold the canary for an operator-defined observation window and check
error rate, latency, database connections, Redis errors, and worker backlog.

## 6. Promote or roll back

Promotion uses the exact canary digest and the production project’s existing
external keyrings. Re-run `/health`, `/ready`, and the smoke matrix immediately
after promotion.

Migrations are forward-only. Image rollback is permitted only when the old
binary is explicitly compatible with the migrated schema. Otherwise stop the
new workload, restore the proven pre-rollout database snapshot, restore the
matching external roots if they changed, and then start the previous digest.

Keep the canary project and pre-rollout backups until the observation window
closes. Remove them deliberately after the rollout record is complete.
