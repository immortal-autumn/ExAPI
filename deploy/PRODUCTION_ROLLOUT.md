# ExAPI production recovery and rollout

Production promotion is fail-closed. Do not change production until every
recovery, artifact, canary, monitoring, and observation record for the exact
image digest has passed and the final manifest has been signed and copied
off-host.

## ExAPI private-only preflight

ExAPI v0.2.0 runs two independent listeners. Set
`EXAPI_PUBLIC_LISTEN_ADDR` for the API-key gateway and
`EXAPI_CONTROL_LISTEN_ADDR`, `EXAPI_CONTROL_HOSTS`, and
`EXAPI_OPERATOR_PEER_IPS` for the direct WireGuard operator listener. The
control port must not be published through the public reverse proxy.

Before starting a migrated database, run the offline command below from the
release image (never from the online server process):

```sh
test -d /app/data/backups
/app/with-migration-report-key.sh /protected/exapi-migration-report.key \
  /app/migrate-private-only \
  --confirm DROP-SAAS-DATA-KEEP-USER-<lowest-active-admin-id> \
  --backup-dir /app/data/backups \
  --report-file /app/data/private-migration-report.json
```

Generate `/protected/exapi-migration-report.key` exactly once on the protected
host with the no-clobber sequence below. It fails if the destination exists:

```sh
key_file=/protected/exapi-migration-report.key
test ! -e "$key_file"
(umask 077; set -C; openssl rand -hex 32 >"$key_file")
```

Keep it as exactly one printable 64-hex-character line, retain an encrypted
off-host copy with the signed report, and do not regenerate it when retrying or
verifying a cutover. The release-image wrapper requires mode `0600`, ownership
by the offline command user, exact length, and lowercase hexadecimal encoding;
it injects the key only into the offline migration process. The running
application does not need this key.
Replace `/app/data/backups` with the application-managed backup directory if it
differs.

If the installation never configured managed backups, first verify that the
`backup_records` setting is absent or an empty JSON array, then use this complete
alternative (without the backup-directory preflight):

```sh
/app/with-migration-report-key.sh /protected/exapi-migration-report.key \
  /app/migrate-private-only \
  --confirm DROP-SAAS-DATA-KEEP-USER-<lowest-active-admin-id> \
  --no-managed-backups \
  --report-file /app/data/private-migration-report.json
```

The command takes a serializable transaction/advisory lock, retains the
lowest-ID active admin, drops customer/commercial tables, records
`private_schema_version=1`, purges pre-cutover backups, and emits a signed
report. Before opening the destructive cutover transaction, database
initialization applies the release image's embedded forward migrations under
the normal migration lock; this is required for upgraded installations with
pending release migrations.
Do not run it until the verified pre-cutover recovery set exists.

## External prerequisites

The rollout is blocked until the operator supplies all of the following:

- S3-compatible backup and secret-recovery locations with encryption,
  versioning, retention, and credentials that cannot delete retained versions.
- Separate `age` recipients for database backups and the protected `.env` /
  keyring bundle. Private identities must be stored outside this host.
- An OCI/provider volume-snapshot adapter and permission to create and restore
  snapshots into disposable targets.
- OCI Health Checks/Notifications, or an equivalent off-host readiness and
  alert-delivery monitor.
- GitHub package, workflow, artifact-attestation, and OIDC permissions.
- Provider-specific egress enforcement for the synthetic canary.

All local evidence and scratch files belong under repository `tmp/`. Never put
private identities, environment files, database dumps, or plaintext secrets in
Git or in rollout manifests.

## 1. Verify the immutable release

Set the reviewed source and all image references by digest. Mutable tags are
informational only.

```bash
export REVIEWED_COMMIT=40_HEX_COMMIT
export EXAPI_IMAGE=ghcr.io/immortal-autumn/sub2api2personal@sha256:RELEASE_DIGEST
export POSTGRES_IMAGE=postgres@sha256:REVIEWED_POSTGRES_18_ALPINE_DIGEST
export REDIS_IMAGE=redis@sha256:REVIEWED_REDIS_8_ALPINE_DIGEST

docker pull "$EXAPI_IMAGE"
docker image inspect "$EXAPI_IMAGE" --format \
  '{{ index .Config.Labels "org.opencontainers.image.revision" }} {{ index .Config.Labels "org.opencontainers.image.source" }} {{ index .Config.Labels "org.opencontainers.image.version" }}'
gh attestation verify "oci://$EXAPI_IMAGE" \
  --repo immortal-autumn/Sub2API2Personal \
  --signer-workflow immortal-autumn/Sub2API2Personal/.github/workflows/release.yml
COMPOSE_ENV_FILE=/protected/exapi-production.env \
  deploy/ops/validate-immutable-compose.sh -f deploy/docker-compose.yml
```

The OCI revision must equal `REVIEWED_COMMIT`; source must identify this fork;
the embedded version, manifest digest, and every platform digest must match the
release workflow's `exapi-image-manifest` artifact. The release must include an
SPDX JSON SBOM and GitHub OIDC/Sigstore attestations whose issuer, repository,
workflow identity, and subject digest all verify. CI actions are commit-pinned;
the application, PostgreSQL, and Redis production images are digest-only.

## 2. Create the recovery set

`create-recovery-set.sh` stops application admission, checkpoints PostgreSQL,
invokes the volume-snapshot adapter while the writer remains stopped, streams a
custom-format `pg_dump` through `age` directly to versioned S3 storage, then
restarts the application. It separately encrypts `.env` and all keyrings to a
different protected location and recipient. It records IDs and checksums, not
secret values.

The executable `SNAPSHOT_CREATE_COMMAND` receives these environment values:

- `EXAPI_ROLLOUT_ID`
- `EXAPI_WRITER_QUIESCED=true`
- `EXAPI_CHECKPOINT_COMPLETED=true`

It must print one JSON object containing `provider`, `snapshot_id`, `target`,
`created_at`, and `retention_until`. Times are RFC3339 UTC.

```bash
export COMPOSE_PROJECT_NAME=exapi-production
export COMPOSE_FILE=deploy/docker-compose.yml
export EXAPI_ENV_FILE=/protected/exapi-production.env
export RECOVERY_S3_URI=s3://exapi-recovery/database
export SECRETS_S3_URI=s3://exapi-keyroots/environment
export AGE_RECIPIENTS_FILE=/protected/database-recipients.txt
export SECRETS_AGE_RECIPIENTS_FILE=/protected/keyroot-recipients.txt
export SNAPSHOT_CREATE_COMMAND=/protected/adapters/create-oci-snapshot
export RECOVERY_RETENTION_UNTIL=2027-08-05T00:00:00Z
export SECRETS_RETENTION_UNTIL=2027-08-05T00:00:00Z
export OPS_TMP_DIR="$PWD/tmp/rollouts/pre-release"
deploy/ops/create-recovery-set.sh
```

If the S3 provider is not AWS, set `S3_ENDPOINT_URL`. Both buckets must report
versioning enabled. Keep database backup, snapshot, environment/keyrings, and
their decryption authority in independent failure domains.

## 3. Prove both restore paths independently

A successful backup job is not a gate. Restore the exact logical-object version
into a networkless disposable PostgreSQL container. Copy and strengthen the
example SQL with deployment-specific row counts and consistency assertions.

```bash
cp deploy/ops/restore-checks.example.sql tmp/restore-checks.sql
# Add expected users, groups, accounts, API keys, subscriptions, usage totals,
# costs, migration version, and encrypted-record assertions.
export RECOVERY_EVIDENCE="$PWD/tmp/rollouts/pre-release/recovery-evidence.json"
export AGE_IDENTITY_FILE=/offline/database-backup-identity.txt
export RESTORE_VERIFY_SQL_FILE="$PWD/tmp/restore-checks.sql"
export KEEP_RESTORE_TARGET=true
deploy/ops/verify-logical-restore.sh
```

Restore the independent provider snapshot through a separate executable adapter:

```bash
export SNAPSHOT_RESTORE_COMMAND=/protected/adapters/restore-and-verify-oci-snapshot
deploy/ops/verify-snapshot-restore.sh
```

The adapter receives rollout, provider, snapshot, and target IDs. It must create
a disposable target, never overwrite production, and return matching IDs plus
`disposable_target`, `restored_at`, `verified=true`, `egress_denied=true`,
`integrity_verified=true`, and `decryption_verified=true`.

Private production must reject every UI/API request for an in-place database
restore, including `POST /api/v1/admin/backups/:id/restore`, with `403`. Recovery
is an offline operation into a disposable target or a stopped replacement.

## 4. Run two different canaries

### Restored production-data canary

Use the PostgreSQL volume and database retained by the verified logical restore;
never create an empty canary database and call it restored. Read the evidence
without evaluating it as shell code, confirm that the named volume is actually
mounted by the verified networkless target, and then remove only the disposable
container so Compose can attach its external volume:

```bash
export LOGICAL_RESTORE_EVIDENCE="$PWD/tmp/rollouts/pre-release/logical-restore/logical-restore-evidence.json"
readarray -t RESTORE_SOURCE < <(python3 - "$LOGICAL_RESTORE_EVIDENCE" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
if d.get("verified") is not True or d.get("network_mode") != "none":
    raise SystemExit("logical restore evidence is not verified and networkless")
for key in ("disposable_target", "volume", "database", "backup_sha256"):
    value = d.get(key)
    if not isinstance(value, str) or not value:
        raise SystemExit(f"logical restore evidence omitted {key}")
    print(value)
PY
)
(( ${#RESTORE_SOURCE[@]} == 4 ))
export RESTORED_LOGICAL_TARGET=${RESTORE_SOURCE[0]}
export RESTORED_POSTGRES_VOLUME=${RESTORE_SOURCE[1]}
export RESTORED_POSTGRES_DATABASE=${RESTORE_SOURCE[2]}
export RESTORED_BACKUP_SHA256=${RESTORE_SOURCE[3]}
test "$(docker inspect "$RESTORED_LOGICAL_TARGET" --format \
  '{{range .Mounts}}{{if eq .Destination "/var/lib/postgresql/data"}}{{.Name}}{{end}}{{end}}')" \
  = "$RESTORED_POSTGRES_VOLUME"
docker rm -f "$RESTORED_LOGICAL_TARGET"

export COMPOSE_PROJECT_NAME=exapi-canary-restored
export EXAPI_CONTAINER_NAME=exapi-canary-restored
export EXAPI_POSTGRES_CONTAINER_NAME=exapi-canary-restored-postgres
export EXAPI_REDIS_CONTAINER_NAME=exapi-canary-restored-redis
export BIND_HOST=127.0.0.1 SERVER_PORT=18080

docker compose --env-file /protected/exapi-canary-restored.env \
  -f deploy/docker-compose.yml \
  -f deploy/docker-compose.canary-restored.yml config
COMPOSE_ENV_FILE=/protected/exapi-canary-restored.env REQUIRE_INTERNAL_NETWORK=true \
  deploy/ops/validate-immutable-compose.sh -f deploy/docker-compose.yml \
  -f deploy/docker-compose.canary-restored.yml
docker compose --env-file /protected/exapi-canary-restored.env \
  -f deploy/docker-compose.yml \
  -f deploy/docker-compose.canary-restored.yml up -d
```

`/protected/exapi-canary-restored.env` must be mode `0600` or stricter and
contain the exact release images and matching production keyroots. It must set
`POSTGRES_USER=postgres`, matching the superuser created by
`verify-logical-restore.sh`; `RESTORED_POSTGRES_VOLUME` and
`RESTORED_POSTGRES_DATABASE` come from the verified evidence above. The overlay
requires both values and treats the restored volume as external, so Compose
cannot silently substitute a new volume or the default database.

The restored-data project remains loopback-only and on an `internal: true`
network for its entire life. Never briefly enable provider egress. Verify
migration state, database integrity, all encrypted-field decryption, login,
API-key authentication, Cockpit totals, and ordered shutdown.

### Empty synthetic provider canary

Create a second project with new empty volumes, synthetic records, canary-only
provider credentials, and no production data or keyroots. Apply a provider
allowlist through the host/cloud egress enforcement adapter before starting it.
Do not claim an allowlist from application URL settings alone. Run one
non-destructive request for each enabled provider and retain enforcement proof.

Both `/health` and `/ready` must be JSON rather than SPA HTML. Use
`observe-rollout.sh` with metrics and network-proof adapters. Restored and
synthetic canaries each run for at least 30 minutes; readiness is checked every
30 seconds or faster.

```bash
export OBSERVATION_CLASS=restored-data # then synthetic-provider
export TARGET_BASE_URL=http://127.0.0.1:18080
export CONTAINER_NAME=exapi-canary-restored
export IMAGE_DIGEST=${EXAPI_IMAGE##*@}
export METRICS_COMMAND=/protected/adapters/query-rollout-metrics
export NETWORK_PROOF_COMMAND=/protected/adapters/prove-canary-network
deploy/ops/observe-rollout.sh
```

## 5. Monitoring and alert proof

Before promotion, configure an off-host monitor against `/ready` at 30-second
intervals. Trigger a synthetic critical alert and retain provider, delivery ID,
recipient confirmation, and RFC3339 delivery time. An application row with
`email_sent=false` or a merely queued notification does not pass. The final
manifest requires `external_readiness_configured=true` and
`alert_probe.delivered=true`.

## 6. Promote and observe

Promote the exact digest tested in both canaries. Run migrations once, keep the
pre-release recovery set, and observe production for at least 60 minutes with
`OBSERVATION_CLASS=production`. Every observation must meet all of these gates:

- zero `/ready` failures at a maximum 30-second interval;
- zero container restarts and zero new P0/P1 alerts;
- error rate below 1% when at least 100 requests were observed; otherwise zero
  unexpected `5xx` responses;
- p95 no more than 20% over baseline when at least 100 latency samples exist.

## 7. Sign the rollout record and retain rollback capability

Copy `deploy/ops/rollout-manifest.example.json` into `tmp/` and assemble the
exact evidence into the schema enforced by `tools/check_rollout_manifest.py`.
The deliberately failing defaults prevent an unverified template from being
promoted. The record includes source and OCI identity,
manifest/per-platform digests, SPDX checksum, provenance identity, both restore
results, protected key IDs, monitoring/alert proof, both canaries, production
observation, private live-restore rejection, no-new-privileges, immutable
runtime images, migration state, retention, and rollback contract.
The restored-data canary's `source` object must repeat the verified logical
restore's disposable target, external PostgreSQL volume, database, and encrypted
backup checksum; validation rejects any mismatch.

```bash
python3 tools/check_rollout_manifest.py tmp/rollouts/ROLLOUT/manifest.json
export ROLLOUT_MANIFEST=tmp/rollouts/ROLLOUT/manifest.json
export ROLLOUT_MANIFEST_S3_URI=s3://exapi-rollout-records/ROLLOUT
export ROLLOUT_MANIFEST_RETENTION_UNTIL=2027-08-05T00:00:00Z
export COSIGN_CERTIFICATE_IDENTITY_REGEXP='^https://github\.com/immortal-autumn/Sub2API2Personal/\.github/workflows/release\.yml@'
export COSIGN_CERTIFICATE_OIDC_ISSUER=https://token.actions.githubusercontent.com
deploy/ops/publish-rollout-manifest.sh
```

The publication command validates, checksums, signs, verifies, uploads, and
requires version IDs for the manifest, checksum, and Sigstore bundle. A KMS or
hardware key can be used with `COSIGN_KEY_REF` and `COSIGN_VERIFY_KEY`.

Before ciphertext-only cutover, an explicitly schema-compatible image rollback
is allowed. Take another proven recovery set immediately before cutover. After
plaintext is cleared, image-only rollback is prohibited: restore the matching
snapshot, exact prior image digest, prior `.env`, and matching prior keyroots.
Do not destroy canaries or recovery objects until the production observation
window and signed-record publication have both completed.
