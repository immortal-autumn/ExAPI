# ExAPI container image

ExAPI publishes Linux `amd64` and `arm64` images to GitHub Container Registry.
Production deployments must use an immutable manifest-list digest from a
verified GitHub release. The current reviewed digest is recorded in
[`../docs/PROJECT_STATUS.md`](../docs/PROJECT_STATUS.md).

## Verify a release image

Set the exact reviewed values before pulling:

```bash
export REVIEWED_COMMIT=<40-character-release-commit>
export EXAPI_IMAGE=ghcr.io/immortal-autumn/sub2api2personal@sha256:<64-hex-digest>

docker pull "$EXAPI_IMAGE"
docker image inspect "$EXAPI_IMAGE" --format \
  '{{ index .Config.Labels "org.opencontainers.image.revision" }} {{ index .Config.Labels "org.opencontainers.image.source" }} {{ index .Config.Labels "org.opencontainers.image.version" }}'
gh attestation verify "oci://$EXAPI_IMAGE" \
  --repo immortal-autumn/ExAPI \
  --signer-workflow immortal-autumn/ExAPI/.github/workflows/release.yml
```

The revision label must equal `REVIEWED_COMMIT`, the source must be this fork,
and the version must match the signed release/tag. The release also publishes
an SPDX JSON SBOM and checksum file.

Do not deploy `latest`, a semantic-version tag, or a `candidate-*` tag to
production. Tags are discovery aliases; only a digest is immutable.

## Docker Compose

The checked-in Compose files are the supported deployment interface:

- `docker-compose.local.yml` stores state in deployment-local directories and
  is recommended for operator-managed production.
- `docker-compose.yml` uses named volumes and is used by the hardened rollout
  workflow.
- `docker-compose.standalone.yml` is the application-only variant.
- `docker-compose.dev.yml` is for local source builds, not production.

Start from the generated `.env` and pin all three runtime images:

```env
EXAPI_IMAGE=ghcr.io/immortal-autumn/sub2api2personal@sha256:<release-digest>
POSTGRES_IMAGE=postgres@sha256:<reviewed-digest>
REDIS_IMAGE=redis@sha256:<reviewed-digest>
POSTGRES_PASSWORD=<secret>
REDIS_PASSWORD=<secret>
SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data-v1
SUB2API_DATA_ENCRYPTION_KEYS_JSON={"data-v1":"<base64-32-byte-key>"}
```

Then render and inspect the exact configuration before starting it:

```bash
docker compose --env-file .env -f docker-compose.local.yml config --images
docker compose --env-file .env -f docker-compose.local.yml pull
docker compose --env-file .env -f docker-compose.local.yml up -d
docker compose --env-file .env -f docker-compose.local.yml ps
```

Never place real passwords or keyrings in a Compose YAML file. Keep `.env` mode
`0600`, back it up separately from PostgreSQL, and retain old encryption-key IDs
until all protected data has been rewrapped.

## Listener model

Private ExAPI deployments use independent public and control listeners:

- `EXAPI_PUBLIC_LISTEN_ADDR` serves API gateway traffic and public readiness.
- `EXAPI_CONTROL_LISTEN_ADDR` serves the operator UI and APIs.
- `EXAPI_CONTROL_HOSTS` and `EXAPI_OPERATOR_PEER_IPS` fail closed to exact
  control hosts and direct WireGuard peers.

Publish the control port only on the host's WireGuard address. Do not route it
through the public reverse proxy. See [`EDGE_SECURITY.md`](EDGE_SECURITY.md).

## Promotion and rollback

An image pull and healthy container start are not sufficient production
evidence. Follow [`PRODUCTION_ROLLOUT.md`](PRODUCTION_ROLLOUT.md) for backups,
restore verification, isolated canaries, attestation checks, promotion,
observation, and rollback.

For an application-only patch with no migration, keep versioned copies of the
environment and Compose input, update only the application service, and leave
PostgreSQL/Redis running:

```bash
COMPOSE_PROJECT_NAME=sub2api \
  docker compose --env-file .env.vX.Y.Z \
  -f docker-compose.vX.Y.Z.yml up -d --no-deps sub2api
```

Verify the running image reference, revision/version labels, health, public
boundary, and control plane from an allowlisted operator peer. Roll back with
the previous versioned environment and Compose file if any gate fails.

## Links

- [GitHub repository](https://github.com/immortal-autumn/ExAPI)
- [GitHub releases](https://github.com/immortal-autumn/ExAPI/releases)
- [Current project status](../docs/PROJECT_STATUS.md)
- [Deployment guide](README.md)
