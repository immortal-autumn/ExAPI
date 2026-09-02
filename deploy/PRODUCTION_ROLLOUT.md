# ExAPI production recovery and rollout

Production promotion is fail-closed. Do not change production until every
recovery, artifact, canary, monitoring, and observation record for the exact
image digest has passed and the final manifest has been signed and copied
off-host.

This runbook is version-neutral. Resolve the currently reviewed release,
commit, and digest from [`../docs/PROJECT_STATUS.md`](../docs/PROJECT_STATUS.md),
then replace every placeholder below. Do not edit generic examples to mirror a
single live deployment.

## ExAPI private-only preflight

ExAPI v0.2.0 and later run two independent listeners. Set
`EXAPI_PUBLIC_LISTEN_ADDR` for the API-key gateway and
`EXAPI_CONTROL_LISTEN_ADDR`, `EXAPI_ALLOW_CONTAINER_WILDCARD_CONTROL_BIND`, `EXAPI_CONTROL_BIND_HOST`,
`EXAPI_CONTROL_HOSTS`, and `EXAPI_OPERATOR_PEER_IPS` for the direct WireGuard
operator listener. In bridge-mode Compose, the listen address is a wildcard
inside the isolated container namespace and
`EXAPI_ALLOW_CONTAINER_WILDCARD_CONTROL_BIND=true` is required explicitly.
`EXAPI_CONTROL_BIND_HOST` must be the server's WireGuard
address, not `127.0.0.1` or a wildcard; the control port must not be published
through the public reverse proxy. Confirm the address is assigned to the
WireGuard interface and that the firewall permits only the listed operator
peer addresses before promotion.

After the final recovery set and canary evidence are complete, run the checked-in
host-side orchestrator below. It stops and verifies the exact reviewed application container ID,
keeps PostgreSQL running, runs the release-image migration and report verifier
as one-shot containers, archives the signed report/key/evidence through the
required encrypted adapter, and leaves the application stopped for inspection:

```bash
export EXAPI_IMAGE=ghcr.io/immortal-autumn/sub2api2personal@sha256:RELEASE_DIGEST
export EXAPI_REVIEWED_COMMIT=<40-character-tagged-commit>
export COMPOSE_FILE=/opt/sub2api/docker-compose.vX.Y.Z.yml
export COMPOSE_ENV_FILE=/opt/sub2api/.env.vX.Y.Z
export COMPOSE_PROJECT_NAME=sub2api
export EXAPI_ROLLOUT_ID=exapi-vX.Y.Z-<change-ticket>
export EXAPI_MIGRATION_REPORT_KEY_FILE=/protected/exapi-migration-report.key
export EXAPI_CONFIRMATION=DROP-SAAS-DATA-KEEP-USER-<lowest-active-admin-id>
export EXAPI_CONTROL_BIND_HOST=<server-wireguard-address>
export EXAPI_CONTROL_LISTEN_ADDR=0.0.0.0:8027
export EXAPI_ALLOW_CONTAINER_WILDCARD_CONTROL_BIND=true
export EXAPI_CONTROL_HOSTS=<server-wireguard-address>
export EXAPI_OPERATOR_PEER_IPS=<operator-wireguard-addresses>
export EXAPI_WIREGUARD_INTERFACE=wg0
export EXAPI_REPORT_ARCHIVE_COMMAND=/protected/adapters/archive-private-cutover-evidence
export EXAPI_REPORT_ARCHIVE_VERIFY_COMMAND=/protected/adapters/verify-private-cutover-archive
export EXAPI_MAINTENANCE_VERIFY_COMMAND=/protected/adapters/verify-exapi-maintenance
export EXAPI_BATCH_CLEANUP_VERIFY_COMMAND=/protected/adapters/verify-batch-cleanup
# Use exactly one of these two policies:
export EXAPI_LEGACY_LOCAL_BACKUP_DIR=/app/data/legacy-backups
# export EXAPI_NO_LOCAL_BACKUPS=true
deploy/ops/run-private-cutover.sh --dry-run
```

Dry-run mode is read-only. It resolves exactly one retained application and
database container in the explicit `sub2api` project, verifies their Compose
project/service labels, records each container ID and image digest, hashes the
candidate Compose file, protected environment, fully rendered environment,
actual mounts, and database cluster identity, and verifies that the control
address belongs to the selected WireGuard interface. The secret-free mode-0600
report defaults to `tmp/cutover-targets/private-cutover-target.json`. Review
both its content and the printed digest, then bind the real run to it:

```bash
export EXAPI_EXPECTED_CUTOVER_TARGET_SHA256=<digest-printed-by-dry-run>
deploy/ops/run-private-cutover.sh
```

Any container recreation, image, mount, Compose/environment input, database
cluster, project, or WireGuard identity change between those commands aborts
before provider cleanup, key staging, image pull, or application downtime.
`COMPOSE_FILE`, `COMPOSE_ENV_FILE`, and `COMPOSE_PROJECT_NAME` are deliberately
mandatory; there is no production default that could select a similarly named
project. `COMPOSE_FILE` is exactly one reviewed candidate file for this
private-production cutover; restored-canary overlay lists use their separate
workflow and are not accepted here. The normal `/opt/sub2api` rolling heuristic is supported: the retained
application may name the versioned release Compose/environment in its labels
while PostgreSQL and Redis retain the prior versioned Compose/environment that
created them (for example `docker-compose.v0.2.7.yml` and `.env.v0.2.7`). Do not
assume they use the current candidate filenames. Each container's own labelled
files are resolved and hashed independently. A
retained container created by an older Compose version may lack the optional
environment-file provenance label; that absence is recorded explicitly while
the mandatory candidate environment is still resolved and hashed.

The script must be run while the normal application container process is
running (its `/ready` healthcheck may be intentionally false before cutover)
and the PostgreSQL service is healthy. It uses the same Compose project, protected
environment, reviewed image digest, data volume, and database network; do not
invoke `/app/migrate-private-only` from the online container or a different
database. It intentionally does not restart the application if migration,
verification, or evidence archival fails.

Before downtime, the orchestrator rechecks the approved target digest, pulls
the release digest, checks its revision label,
opens the protected source key exactly once without following any path
symlinks, rejects hardlink aliases, and copies those bytes into a protected
single-link `0600` staging file under this checkout's `tmp/` directory. It then
copies only that staged file into an ephemeral Docker volume as UID 1000/mode
0600 and uses the same staged bytes for archival; it never rereads the mutable
operator-supplied path. It also exercises the real entrypoint, key wrapper,
binaries, production data volume, backup path, and database network. The
maintenance adapter must prove public ingress is closed and the expected
running-replica count both before and after the stop. The post-stop gate also
requires zero unexpected PostgreSQL client sessions.
The batch-cleanup adapter runs before downtime and must inspect every configured
provider, archive its detailed manifest to immutable/versioned S3 storage, and
return exactly one JSON object with `schema_version=1`, `verified=true`, a fresh
RFC3339 UTC `verified_at`, and zero values for `sql_rows_remaining`,
`provider_jobs_remaining`, `provider_inputs_remaining`, and
`provider_outputs_remaining`. It must also return the off-host `s3://`
`evidence_uri`, its nonempty `evidence_version_id`, and the lowercase
`evidence_sha256` of that exact version. These fields are embedded in the signed
migration report. The database preflight independently requires
`batch_image_jobs` to contain zero rows. If either check fails, stop: cancel
every provider-side job, delete provider-managed inputs and outputs using the
retained account credentials, preserve cleanup evidence, and remove the
corresponding SQL rows before retrying. The cutover deliberately refuses to
erase those references itself.

Generate `/protected/exapi-migration-report.key` exactly once on the protected
host with the no-clobber sequence below. It fails if the destination exists:

```sh
key_file=/protected/exapi-migration-report.key
test ! -e "$key_file"
(umask 077; set -C; openssl rand -hex 32 >"$key_file")
```

Keep it as exactly one printable 64-hex-character line, retain an encrypted
off-host copy with the signed report, and do not regenerate it when retrying or
verifying a cutover. The host source must be a regular, non-symlink file with
mode `0600` and exactly one hard link; every parent directory in its absolute
path must also be non-symlink. The orchestrator snapshots it once, copies the
snapshot into an ephemeral Docker volume owned by the offline UID, and the
release-image wrapper then re-enforces owner, mode, link count, exact length,
and lowercase hexadecimal encoding. It injects the key only into the offline
migration process. The running application does not need this key.

`EXAPI_LEGACY_LOCAL_BACKUP_DIR` is only for an explicitly identified legacy
local backup directory whose pre-cutover files may be purged after commit. The
current S3 backup service is a separate object store: its `backup_records`
metadata and objects remain preserved and must not be treated as a local path.
The migration report path must be outside this legacy backup root. The command
snapshots every file regardless of filesystem mtime and, after the allowlisted
purge, re-scans the rooted tree; any late or uncommitted entry stops finalization.
If no legacy local backup directory was ever configured, verify that fact from
the deployment records and use this complete alternative instead; it does not
infer the choice from S3 `backup_records`:

```bash
unset EXAPI_LEGACY_LOCAL_BACKUP_DIR
export EXAPI_NO_LOCAL_BACKUPS=true
deploy/ops/run-private-cutover.sh --dry-run
# Review the report, export its printed EXAPI_EXPECTED_CUTOVER_TARGET_SHA256,
# then run deploy/ops/run-private-cutover.sh without --dry-run.
```

The command takes a serializable transaction/advisory lock, retains the
lowest-ID active admin, drops customer/commercial tables, records
`private_schema_version=2`, requires the batch-image job table to be empty,
purges pre-cutover legacy local backups, and emits a signed
report. The release image applies pending embedded forward migrations during
database initialization under the normal migration lock before the destructive
transaction. The verifier then checks the signed report HMAC, payload digest,
durable database marker, decoded-key fingerprint, and private runtime readiness.

If the database already contains an unreleased schema-v1 private-state row,
resume with its original 0600 signed report at the same `--report-file`, its
original signing key, and the original local-backup policy/root. The command
verifies the legacy report against the locked database metadata, confirms every
previously purged path is still absent, requires fresh zero-state batch cleanup
evidence, and transactionally translates the row to v2. It never reconstructs
destructive counts from the already-modified database.
If that historical report was written inside the legacy backup root, first make
a protected, single-link 0600 byte-for-byte copy outside that root, verify its
SHA-256 against the durable schema-v1 marker, and use the external copy as
`--report-file`; the v2 command deliberately rejects all report/root overlap.
The archive adapter receives the staged report, verifier evidence, and staged
key paths. It also receives `EXAPI_MIGRATION_REPORT_KEY_FILE_SHA256` (SHA-256 of
the exact 65-byte lowercase-hex-plus-newline file) and
`EXAPI_MIGRATION_REPORT_KEY_SHA256` (SHA-256 of the 32 decoded signing-key
bytes). It must return one JSON object with `encrypted=true`, encrypted
report/evidence/key URIs, immutable version IDs, a future retention time, and
these unambiguous digest fields:

- `report_file_sha256`: SHA-256 of the complete signed report file.
- `evidence_file_sha256`: SHA-256 of the complete verifier evidence file.
- `key_file_sha256`: SHA-256 of the exact printable key file archived.
- `report_key_sha256`: SHA-256 of the decoded signing-key bytes.

`report_key_sha256` must match the verifier evidence, which has already matched
the same decoded key against the durable database cutover evidence. It is not
interchangeable with `key_file_sha256`. All three URIs must be off-host `s3://`
objects with a nonempty bucket and object key and without credentials, query, or
fragment components. A separate archive-verification adapter must independently inspect
those exact versions and return `verified=true`, `encrypted=true`,
`immutable=true`, matching URIs, version IDs, all four digest fields, and the
same retention timestamp. Review both evidence files before starting the
application.

If migration committed but report verification or archival failed, leave the
application stopped and resume the same rollout ID. Resume mode requires the
retained application container to remain stopped and repeats the idempotent
cutover finalizer, verification, and archive gates:

```bash
export EXAPI_ROLLOUT_ID=exapi-vX.Y.Z-<same-change-ticket>
export EXAPI_RESUME_PRIVATE_CUTOVER=true
deploy/ops/run-private-cutover.sh
```

After the archive evidence is copied off-host and independently reviewed, start
only the reviewed digest explicitly:

```bash
COMPOSE_PROJECT_NAME=sub2api \
COMPOSE_ENV_FILE=/opt/sub2api/.env.vX.Y.Z \
  EXAPI_IMAGE=ghcr.io/immortal-autumn/sub2api2personal@sha256:RELEASE_DIGEST \
  docker compose --env-file /opt/sub2api/.env.vX.Y.Z \
  -f /opt/sub2api/docker-compose.vX.Y.Z.yml up -d --no-deps sub2api
```

Then verify `/ready`, the WireGuard-bound control listener, and the external
monitor before declaring cutover complete.

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
  --repo immortal-autumn/ExAPI \
  --signer-workflow immortal-autumn/ExAPI/.github/workflows/release.yml
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
- `EXAPI_SNAPSHOT_SCHEMA_VERSION=1`
- `EXAPI_WRITER_QUIESCED=true`
- `EXAPI_CHECKPOINT_COMPLETED=true`
- `EXAPI_CHECKPOINT_AT`
- `EXAPI_SNAPSHOT_SOURCE_CONTAINER_ID`
- `EXAPI_SNAPSHOT_SOURCE_IMAGE_ID`
- `EXAPI_SNAPSHOT_SOURCE_MOUNTS_SHA256`

It must print one schema-versioned JSON object containing `provider`,
`snapshot_id`, `target`,
`created_at`, `retention_until`, the exact rollout/checkpoint/quiescence fields,
and a `source` object containing `container_id`, `image_id`, and
`mounts_sha256`. Times are RFC3339 UTC. The recovery script rejects stale
evidence or any value not bound to the database container that was actually
checkpointed.

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

The repository-owned `deploy/ops/restore-checks.required.sql` always runs
first; `RESTORE_VERIFY_SQL_FILE` only adds deployment-specific assertions and
cannot replace the minimum schema, migration-checksum, and active-admin checks.
The evidence records SHA-256 values and separate verification flags for both
validators. Omitting the optional file still runs the mandatory validator.

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
# The observer/network adapter reads this file as protected evidence. It must
# be a root-owned, regular, non-symlink file with mode 0600; verify-logical-
# restore.sh creates new evidence with umask 077 and applies this mode.
export EXAPI_RESTORED_COUNTS_FILE="$LOGICAL_RESTORE_EVIDENCE"
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

For restored-data observations, the network adapter compares users, active
accounts, active API keys, batch image jobs, schema migrations, and groups with
`EXAPI_RESTORED_COUNTS_FILE`. The proof includes that file's SHA-256 and restore
rollout ID, and the observer requires both fields in the final evidence. A
stale hard-coded account/key count or a writable/symlinked evidence file must
fail closed.

```bash
export OBSERVATION_CLASS=restored-data # then synthetic-provider
# Use the exact IP captured from docker inspect; the observer rejects an
# unrelated loopback/public URL even when it happens to be healthy.
export TARGET_BASE_URL=http://RESTORED_CONTAINER_IP:8080
export CONTAINER_NAME=exapi-canary-restored
export IMAGE_DIGEST=${EXAPI_IMAGE##*@}
export METRICS_COMMAND=/protected/adapters/query-rollout-metrics
export NETWORK_PROOF_COMMAND=/protected/adapters/prove-rollout-network
export EXAPI_RESTORED_POSTGRES_EXPECTED_ID=FULL_RESTORED_POSTGRES_CONTAINER_ID
export EXAPI_RESTORED_POSTGRES_EXPECTED_DATA_SOURCE=VERIFIED_RESTORED_DATA_MOUNT
# Synthetic observations additionally require a protected, non-secret-output
# exercise adapter that makes a provider request inside the observation window.
# export EXAPI_OBSERVATION_EXERCISE_COMMAND=/protected/adapters/exercise-synthetic-provider
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
export COSIGN_CERTIFICATE_IDENTITY_REGEXP='^https://github\.com/immortal-autumn/ExAPI/\.github/workflows/release\.yml@'
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
