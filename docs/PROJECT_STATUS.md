# ExAPI project status

This is the canonical, dated status record for the ExAPI fork. Update it when a
release is published, production is promoted or rolled back, an operational
invariant changes, or a known external blocker is resolved. Generic procedures
belong in `deploy/`; this page records the currently reviewed facts.

## Current release

Last reviewed: **2026-09-05 (Europe/London)**

| Item | Current value |
|---|---|
| Product version | `0.2.16` (released and deployed to OPC on 2026-09-03) |
| GitHub repository | `immortal-autumn/ExAPI` |
| Git tag | `v0.2.16` |
| Main branch | `main` (currently `f79ef301b`; release branch remains separate) |
| Release branch | `revision/exapi-v0.2.1` |
| Reviewed commit | `9c14b10843b175dac8ef0546866a141504bcaed4` |
| OCI image | `ghcr.io/immortal-autumn/sub2api2personal@sha256:d3b889b74dcd15c9952b409ce27f05db1898f93541eac17cf7675088d6af65b0` |
| GitHub release | <https://github.com/immortal-autumn/ExAPI/releases/tag/v0.2.16> |
| Release workflow | <https://github.com/immortal-autumn/ExAPI/actions/runs/33756324283> |
| Upstream baseline | Sub2API `v0.1.171`, constrained by `upstream.lock.json` |

The GitHub repository was renamed from `Sub2API2Personal` to `ExAPI` on
2026-08-20. The existing v0.2.5 GHCR package path remains
`sub2api2personal` for image and deployment compatibility; future release work
must keep that compatibility decision explicit or publish a separately
verified package migration.

The v0.2.16 release artifact is pinned to the immutable digest recorded above;
its OCI labels match version `0.2.16` and reviewed commit
`9c14b10843b175dac8ef0546866a141504bcaed4`. The v0.2.15 release remains
available as the preceding reviewed deployment. The prior v0.2.8 tag
remains immutable but was never published because its commit still contained
`backend/cmd/server/VERSION=0.2.7`; it must not be retagged.

## v0.2.16 API-key usability release

The v0.2.16 release is tagged from commit
`9c14b10843b175dac8ef0546866a141504bcaed4`. It clarifies in English and
Simplified Chinese that the complete API key secret is displayed only in the
successful creation response, makes the required group selection error visible
inline, and explains that a redacted existing key cannot be recovered and must
be replaced. The release workflow (run `33756324283`) passed; frontend tests
(267 files, 1,438 tests), typecheck, lint, and production build passed.

The exact application image promoted to OPC is pinned at
`ghcr.io/immortal-autumn/sub2api2personal@sha256:d3b889b74dcd15c9952b409ce27f05db1898f93541eac17cf7675088d6af65b0`.
Its OCI labels report version `0.2.16` and revision
`9c14b10843b175dac8ef0546866a141504bcaed4`.

## v0.2.11 release validation outcome

The annotated `v0.2.11` release was published from commit `b2a5889b4` and its
release/security/CI workflows passed. Its immutable OCI manifest is
`sha256:1a27d4282b714ecf5b50234a5a55f5ea89c3e353b897fa8b5877cdaf71eda673` and
the image labels match version `0.2.11` and that commit. It was pulled to OPC
for validation but was not promoted to production.

A fresh synthetic-provider canary against the exact digest failed before
cutover: the positional private identity map omitted the newly inserted
`prompt_audit_events` entry, causing `user_group_rate_multipliers` to be
snapshotted with a nonexistent `id` column. The canary was isolated and cleaned
up automatically; production remained on v0.2.10. This release is therefore
superseded for deployment, even though its artifact gates passed.

## v0.2.12 release validation outcome

The `v0.2.12` artifact was built and attested with manifest digest
`sha256:e0e7e22dd11a38cdd8b97e4f11750dff1613e602a0342907e9b6aa2b95c9f2f3`.
Its first fresh synthetic-provider canary passed the previously discovered
user/group mapping, then failed on the next real schema shape: `user_affiliates`
uses `user_id` as its primary key and has no `id` column. The isolated canary was
cleaned up automatically and production remained on v0.2.10. The artifact is
superseded and must not be promoted.

## v0.2.13 release-candidate preparation

`backend/cmd/server/VERSION` now declares `0.2.13`. The candidate adds an
explicit identity kind for user-primary-key tables and covers both
`user_affiliates` references and `passkey_user_handles` in regression tests. It
must receive a new annotated tag and digest; v0.2.11 and v0.2.12 artifacts and
canary evidence cannot be reused. Promotion remains blocked until the new digest
passes the full release workflow, fresh restored-data and synthetic-provider
canaries, and immediate off-host readiness/alert verification.

The fail-closed private-cutover matrix remains covered: retained API-key,
usage/billing, and operational ownership references are reassigned or nulled;
customer-only rows are deleted, historical snapshots are retained, and signed
row-count/identity checksums are recorded. Missing optional tables/columns are
recorded as skipped, while a non-nullable historical reference aborts the
transaction. Legacy `user_external_identities` rows are covered explicitly.

## v0.2.14 release and deployment outcome

The v0.2.13 release workflow was superseded before promotion: its CI failed
only because two newly added Go test map entries were not `gofmt`-aligned. No
image from that tag was deployed. The formatting fix is committed separately,
`backend/cmd/server/VERSION` now declares `0.2.14`, and the candidate must use
a new annotated tag, immutable digest, attestations, and fresh canary evidence.
The new annotated tag was published from commit `4b0352fa87720425bf4fb5c23aa91e2c0e212c9e`.
The attested multi-architecture OCI manifest is
`sha256:e8a6d161a1acb5d454a13526ef2914533d077fd5aefae7a412bc45f58513857d`, with
amd64 digest `sha256:6e979419dd9ab1f1ebbccd664a1cc2b21cd439dfa5caa1ba63cbd99e2ff895b9`,
arm64 digest `sha256:59fa4a44007bd57a7516b35ccefb1077107e8fdecd98b8546893c080ab876e70`,
and SPDX SBOM SHA-256 `f071f64fcc656ed6d6f4ef4b17f435da294265cbb239b9772716b76b61010250`.
The release workflow and artifact attestation passed.

## v0.2.15 administrator-only release

The v0.2.15 release is tagged at `a63f68a11b08cdee6ead8e4cce41332cb4e83ac3`
and uses the immutable multi-architecture image
`ghcr.io/immortal-autumn/sub2api2personal@sha256:f25727e7dce06ce62ab921027346c27e86a92dabdd0e6e7dc7791333526889b0`.
The OPC production container reports version `0.2.15` and that exact revision
and digest. The synthetic-provider canary passed before promotion; restored
data required the reproof recorded below.

## v0.2.15 restored-observation reproof

The first v0.2.15 restored-data observations completed their 30-minute
readiness windows, but the network proof adapter still expected the obsolete
`1|23|8|0|...` database cardinality. The verified v0.2.15 logical restore
contains `1|3|3|0|246|9`, so the adapter failed after shell redirection created
an empty `network-proof.json`; the missing final evidence was a fail-closed
observer result, not an empty successful proof. No production data was changed.

The release source now compares all six cardinalities against the protected
logical-restore evidence, reads and hashes that evidence through one
root-owned `0600` non-symlink file descriptor, and records its SHA-256 and
restore rollout ID in the network proof. Logical-restore output is created with
`umask 077`; the observer requires the new evidence binding. The OPC v4 adapter
was atomically updated to this source and its deployed SHA-256 is
`d300f6e66288adca9ffd7d1cd1a977484c2f28461f67f41fb71b21542e4aabe8`.

A fresh 30-minute restored-data observation then passed using rollout
`exapi-v0215-restored-reproof-20260902a`. It completed 60/60 readiness checks
with zero failures, zero restarts, zero unexpected 5xx, error rate `0.0`, and
30-minute p95 within the baseline gate. Its network proof records
`egress_denied=true`, `integrity_verified=true`, `decryption_verified=true`,
and the SHA-256 of the protected logical-restore evidence. The final evidence
is retained under the checkout-local `tmp/rollouts/` policy on OPC. Its
evidence SHA-256 is
`6d91254ed770bf5ca5e10e844c845a2e154621e9680bb68b6eb7fb8bc7e29116`, the
network proof SHA-256 is
`dead48986f23334e91bdbc591fd28c6838da999854e8fa2cf16a5235a52cdb24`, and the
readiness trace SHA-256 is
`98ddd3ab4f5119fce3473d101d6618946c217265a6d1326cea3fed250c90cbb4`.

The final v0.2.15 rollout manifest is retained at
[`tmp/rollouts/exapi-v0215-cutover-20260902a/rollout-manifest.json`](../tmp/rollouts/exapi-v0215-cutover-20260902a/rollout-manifest.json)
with SHA-256
`4a3b2cb531f56643222ea1067656e738827a9b3882beee7b8ef7c0fe09e5a26e`. The
manifest was accepted by `tools/check_rollout_manifest.py`, keyed-cosign signed,
verified, and published with COMPLIANCE retention through 2027-09-03 at
`s3://exapi-rollout-records/exapi-v0215-cutover-20260902a`. The retained object
versions are `manifest.json`
`d38af49a-aa21-48d6-ad50-d8ec3fd08d7d`, `manifest.json.sha256`
`a1b4d45f-c75e-4815-83ab-16acbbf44afe`, `manifest.sigstore.json`
`134739de-a5cc-48e1-a5ab-1a81621d28ef`, and `oci-provenance-verification.json`
`1a1f3e29-3cdf-4683-ab10-374c8a447600`.

The manifest binds the v0.2.15 image manifest (linux/amd64
`df820aeed3803cd528509c3753a8d926e66a2c103b6dc1068d1c506f0dbb1362`,
linux/arm64 `393de69cd6d8346b11e40d88677605f3c449fcdb324c27aaa220265868caed46`)
and SPDX SBOM SHA-256
`a7e7e46f923ecfcfcd2b838fc851385280c580face53866ae173d3a5a36677e8` to the
release provenance. It records recovery set `exapi-v0215-recovery-20260902c`
(logical backup version `8f78e26e-f7a8-450c-b502-9814addf1369`, snapshot
`e786ac46-d37d-46cd-a725-9ef6a3a2eabb`), the synthetic canary
`exapi-v0215-syn-20260902a`, and the final restored-data reproof above. The
private migration archive is independently verified, encrypted, immutable, and
retained at `s3://exapi-cutover-evidence/exapi-v0215-cutover-20260902a`; its
database and key objects retain separate version IDs. The off-host monitor
delivered the 503 alert at `2026-09-02T20:19:09Z` and the 200 recovery at
`2026-09-02T20:22:15Z`.

## Administrator hardening review branch

The release/review branch `revision/exapi-v0.2.1` has continued
administrator-surface hardening through 2026-09-01. The following independently
revertible commits have passed their local focused/full checks and GitHub CI and
security scans:

The production-status phrases in the entries below describe the state when each
review commit was recorded. Every listed commit is an ancestor of v0.2.16 and is
therefore included in the deployed image unless explicitly marked otherwise.

- `30f5dc180`: routine proxy credential minimization;
- `3453be8cf`: proxy destructive-action UI guards;
- `faca8d3e0`: proxy create/update in-flight submission guards;
- `99cfb8a64`: proxy JSON-import in-flight submission guard.
- `18d0b7390`: proxy partial-update contract; omitted fields are preserved and
  explicit nullable values can clear stored settings.
- `27f6b9697` and `4cb556a42`: fresh Antigravity capability diagnostics,
  bounded probe reasons, and provider-body redaction; both passed GitHub CI and
  security scanning but are not yet promoted to production.
- `403a6a928`: user and operator error-detail views now render only bounded,
  redacted provider-response metadata; the first-open user detail modal also
  loads its selected record deterministically. Focused and full frontend tests,
  typecheck, build, and bundle gates pass.
- `e2ebfda27`: removed the Driver.js onboarding runtime, global onboarding
  stylesheet, tour store/composable/steps, and stale onboarding locale payloads
  from the administrator control-plane build. Stable workflow hooks are now
  `data-testid` selectors; focused and full frontend tests, typecheck, build,
  bundle budgets, and changed-file lint pass.
- `bf7e501aa`: bounded Grok SSO import request bodies to 25 MiB and added a
  handler test proving oversized requests are rejected before an OAuth client
  call.
- `593e13727`: allowed Antigravity OAuth imports to derive names from the
  validated email/default, added confirmation dialogs for administrator API
  key regeneration/deletion, and rejected oversized hand-entered Grok SSO
  batches before the API call.
- `25baef30f`: separated the private administrator API-key route with an
  operator-mode wrapper and routed group changes through the existing admin
  contract; explicit unbind now uses the documented `group_id: 0` sentinel.
  Focused/full frontend checks and local type/lint gates pass; production has
  not been changed.
- `e34ef0eb9`: API-key create idempotency now stores only a sanitized response;
  the first response retains the one-time secret while persisted replays omit
  it. The implementation and focused handler tests are recorded in
  [`phase-2-api-key-idempotency.md`](../tmp/usability-review/current/phase-2-api-key-idempotency.md):
  the browser does not yet attach a key automatically because a secret-redacted
  replay cannot recover a key when the original response is lost; this remains
  an explicit follow-up API contract decision.
- `efca8436b`: extended the stable `410 Gone` retired-customer contract to the
  historical personal-profile API prefix `/api/v1/users` and the public
  model-plaza prefix `/api/v1/model-plaza`, with strict segment-boundary tests.
  Production has not been changed.
- `049ece0f3`: closed the remaining dynamic subscription gap under
  `/api/v1/admin/groups/:id/subscriptions`; exact resource-segment matching
  now returns the same bilingual `410 Gone` contract without retiring the
  operational group API. Production has not been changed.
- `9dacfcf2f`: bounded offline migration-report verification to 4 MiB and
  added descriptor/path identity and post-read stability checks to fail closed
  on replacement or mutation. Production has not been changed.
- `bfbc68c94`: proxy backup imports now surface post-create and post-reuse
  `UpdateProxy` failures in structured results and failure counts instead of
  reporting a misleadingly complete import. Production has not been changed.
- `2d3d24d5f`: migration-report path checks now use `Lstat` and reject a
  symlink or non-regular replacement during verification. Production has not
  been changed.
- `ea8c669c7`: proxy-only imports now count post-create and post-reuse status
  synchronization failures in `proxy_failed`, matching their structured error
  entries. Production has not been changed.
- `6d8e39679`: proxy-only and combined imports now use the same explicit
  synchronization-failure wording, making structured partial-failure results
  consistent for operators. Production has not been changed.
- `72aa62eea`: added regression coverage for reused and newly-created proxy
  synchronization failures and committed the Phase 3 evidence records. The
  complete GitHub CI and security scan passed; production has not been changed.
- `f850a0f14`: moved the operator batch-image page to the explicit
  `/admin/batch-images` control-plane route, while retaining `/batch-image` as
  a bilingual retirement page; navigation, prefetch, route-matrix tests,
  typecheck, and changed-file lint passed. Production has not been changed.
- `7a9917c3f`: serialized per-key administrator API-key group changes with a
  pending guard and disabled control; the deferred-request regression test,
  typecheck, and changed-file lint passed. Production has not been changed.
- `049ee862b`: scheduled account-test plans now use a bounded PostgreSQL
  claim lease (`FOR UPDATE SKIP LOCKED`) before upstream work and lease-owned
  completion, preventing duplicate execution across replicas and stale
  workers. Focused tests, repository/service tests, `go vet`, and race tests
  passed. Production has not been changed.
- `4e68f560e` and `f8cee085c`: the batch-image Codex instruction is now
  localized for English and Chinese interfaces. Literal JSON braces are
  tokenized before Vue i18n compilation and restored only when copied, with
  regression coverage for both languages and missing-template fail-closed
  behavior. GitHub CI, frontend quality gates, and security scanning passed;
  production has not been changed.

The review branch also contains work hardening proxy partial updates: omitted
lifecycle and credential fields are preserved, while explicit null/empty values
clear nullable settings. This work was review-only before v0.2.14 and was not
present in the v0.2.10 production image; it is an ancestor of and included in
the deployed v0.2.16 image.

The latest review-only commits added a bilingual administrator-only product
surface contract (`bdd4329e3`), made all supported deployment examples default
to `RUN_MODE=simple` while preserving the backend's legacy `standard` fallback,
and derived OAuth/setup-token account display names from an explicit name,
validated email, or provider fallback (`8b25ba36c`). Focused account/private-route
tests, frontend typecheck/build, deployment bind/security/rollout contracts, and
the release contract passed. They were not present in the v0.2.10 image, but are
ancestors of and included in the deployed v0.2.16 image.

The review commit `da3e56788` exposes the retained private security
boundary in the administrator settings view, prevents a read-only security tab
from triggering a global settings save, and localizes the affected security,
OAuth, pagination, and 404 copy in English and Simplified Chinese. It adds
regression coverage for the security tab and retained-tab contract; the full
frontend suite (267 files, 1,435 tests), typecheck, lint, production build,
bundle budgets, and deployment contracts passed. This commit is on
`revision/exapi-v0.2.1` and was not present in the v0.2.10 image; it is included
in the deployed v0.2.16 image. GitHub CI run `33607824144` and Security Scan run
`33607824171` completed successfully for the resulting documentation state.

## v0.2.16 OPC production deployment

Production is currently deployed as Compose project `sub2api` from
`/opt/sub2api` using `/opt/sub2api/.env.v0.2.16` and
`/opt/sub2api/docker-compose.v0.2.16.yml`. The application container is pinned
to manifest digest
`sha256:d3b889b74dcd15c9952b409ce27f05db1898f93541eac17cf7675088d6af65b0`,
reports `0.2.16`, and has revision
`9c14b10843b175dac8ef0546866a141504bcaed4`. PostgreSQL and Redis remain on
their existing persistent volumes; all three production services are healthy
with zero restarts and `/ready` returns `{"status":"ready"}`.

The allowlisted WireGuard operator peer reached `/api/v1/operator/me` and the
private `/api/v1/keys` route successfully. A real API-key creation check
returned the complete secret in the first response, and the test key was then
deleted through `DELETE /api/v1/keys/:id`; the production list no longer
contains the `ui-canary-key` probe record. The retained account and production
key records were not otherwise changed.

## 2026-09-05 OPC chunk-load recovery deployment

The application service was safely recreated with `--no-deps` from the
committed fix `7560df6cc` (`fix(web): prevent stale chunks from triggering reload loops`).
PostgreSQL and Redis were not recreated and remain healthy with zero restarts.
The running application is pinned to the local immutable image reference
`exapi-chunkfix@sha256:3d4a3377a690df44d5de18ba93dc8077d1e65bc2fa05fb28e0d868f6f4879875`,
with OCI revision `7560df6cc` and version `0.2.16-chunkfix`. Its protected Compose
environment is `/opt/sub2api/.env.v0.2.16-chunkfix-7560df6cc` (mode `0600`); the
original v0.2.16 environment is retained unchanged for rollback.

Post-deployment checks from the allowlisted operator path returned
`/ready={"status":"ready"}`, SPA routes returned `200 text/html`, and a missing
fingerprinted asset returned `404 text/plain` with `Asset not found` instead of
`index.html`. The served application bundle no longer contains the old
`window.location.reload()` chunk-recovery path. This is a locally built,
immutable digest deployment and is not yet a GHCR release or attested release
artifact; publish and verify a signed registry image before using this fix as a
new versioned release elsewhere.

## v0.2.15 OPC production deployment (historical)

Before v0.2.16 promotion, production was deployed as Compose project `sub2api` from
`/opt/sub2api` using `/opt/sub2api/.env.v0.2.15` and
`/opt/sub2api/docker-compose.v0.2.15.yml`. The application container is pinned
to manifest digest
`sha256:f25727e7dce06ce62ab921027346c27e86a92dabdd0e6e7dc7791333526889b0`,
reports `0.2.15`, and has revision `a63f68a11b08cdee6ead8e4cce41332cb4e83ac3`.
At the last review, all three services were healthy with zero restarts and
`/ready` returned `{"status":"ready"}`. PostgreSQL and Redis were recreated
by the dependency reconciliation described below, but their persistent data
was retained. The completed 60-minute production observation recorded 120/120
readiness checks, zero readiness failures, restarts, unexpected 5xx, and new
P0/P1 alerts; error rate was `0.0` and p95 was `4.0 ms` against a `4.852 ms`
baseline. The signed rollout manifest and its provenance evidence are retained
in the versioned off-host record described above.

After promotion, the operator-only account update endpoint corrected one legacy
OAuth record whose display name was the placeholder `1`; it now uses the
validated identity-derived name. The update changed only the display name
(credentials, groups, scheduler state, and status were preserved). A follow-up
account-list read confirmed all three configured accounts remain active and
schedulable.

### 2026-09-02 deployment recovery note

During the post-check deployment, a full Compose `up -d --no-build --pull
never` unexpectedly reconciled dependency configuration and recreated the
PostgreSQL and Redis containers. The persistent volumes and database were not
removed or initialized. Redis initially failed because its existing data files
were owned by `root` while the hardened container runs as UID/GID `999:1000`.
The operator stopped Redis, corrected ownership on `/opt/sub2api/redis_data`,
and restarted it; Redis loaded the existing AOF successfully (`DBSIZE=72`).
PostgreSQL and Redis then returned healthy, with zero restarts, and the
application container was reconciled separately with:

```text
docker compose --env-file .env.v0.2.15 \
  -f docker-compose.v0.2.15.yml \
  up -d --no-deps sub2api
```

The retained data was rechecked after recovery (users `1`, accounts `3`, API
keys `3`, groups `7`). Future production application updates must use the
`--no-deps sub2api` form; changing PostgreSQL or Redis requires an explicit
maintenance window, recovery set, and independent migration plan.

## v0.2.14 production deployment (historical)

The pre-cutover recovery set `exapi-v0214-recovery-20260902a` is retained in
encrypted, versioned off-host objects through 2027-09-03. The logical restore
uses disposable target `exapi-logical-restore-exapi-v0214-recovery-20260902a`,
volume `exapi-logical-restore-exapi-v0214-recovery-20260902a-data`, database
`exapi_restore`, and backup version `b6156b4a-75b4-4877-8684-7d58234dde0c`.
The independent physical restore uses snapshot
`7af6943e-9a5d-4f06-bc8f-282e55ccc333`; both restore paths verified encryption,
integrity, and network isolation.

The v0.2.14 restored-data canary (`exapi-v0214-restored-observe-20260902c`) and
synthetic-provider canary (`exapi-v0214-synthetic-20260902a`) each ran 30 minutes
with 60/60 readiness checks, zero failures, zero restarts, zero unexpected 5xx,
and provider/egress or decryption/integrity proofs as applicable. The controlled
off-host monitor delivered both the 503 critical alert and the 200 recovery alert.

The final rollout manifest is recorded locally at
[`tmp/rollouts/exapi-v0214-cutover-20260902a/rollout-manifest.json`](../tmp/rollouts/exapi-v0214-cutover-20260902a/rollout-manifest.json),
SHA-256 `b80f4b13211c78f19b9d5503cdf86c6e1c39461d558a07279a0caec0fcca2eb5`.
It was validated, keyed-cosign signed, provenance-verified, and retained under
`s3://exapi-rollout-records/exapi-v0214-cutover-20260902a` with COMPLIANCE
retention through 2027-09-03. Object versions are:
`manifest.json` `14dd653e-b76e-452d-8b49-f7b44121c967`, checksum
`69f549d8-fb36-4701-bea4-f217ef07c967`, signature bundle
`fcfbfb51-7081-476a-baf8-4e5010665d2f`, and provenance evidence
`0fd6d018-7fe9-414b-ae67-e41701e45cc2`.

Post-publication local gates remain green: frontend Vitest `266` files/
`1432` tests passed, `vue-tsc` typecheck passed, both bundle budgets passed,
the snapshot-evidence unit suite passed (`5` tests), and the production-rollout
contract passed.

The final production observation (`exapi-v0214-production-observe-20260902a`)
ran for 60 minutes with 120 readiness probes:

- readiness failures: `0`; restarts: `0`; unexpected 5xx: `0`;
- error rate: `0.0`; p95: `4.0 ms` versus a `4.852 ms` baseline;
- new P0/P1 alerts: `0`; production topology and dependency identities: verified.

The public `/health` and `/ready` endpoints returned 200 after maintenance was
disabled, and the WireGuard-bound control endpoint returned 200 from an
allowlisted operator peer. Private listener addresses are intentionally omitted
from this public status record.

Do not copy protected environment files, database dumps, provider credentials,
or signing keys into this repository. Deployment evidence and scratch output
must stay under a checkout-local `tmp/` directory or an approved protected
off-host archive.

## Network and readiness contract

ExAPI uses two listeners:

- The public listener exposes gateway endpoints and `/ready`. In private mode,
  the control UI root and control APIs are deliberately hidden with HTTP 404.
- The control listener exposes the operator UI and control APIs only to exact
  configured WireGuard peers and hosts. API requests require
  `X-ExAPI-Control-Request: 1`; unsafe browser requests also require a matching
  origin.

A request from the server itself is not an operator-peer test and may correctly
receive 404 on the control listener. Validate the control plane from an
allowlisted WireGuard peer. The reviewed production checks returned 200 for the
control root, control `/ready`, and account-list API from an authorized peer,
while the public root remained 404 and public `/ready` returned 200.

See [`deploy/EDGE_SECURITY.md`](../deploy/EDGE_SECURITY.md) for the generic
boundary and [`deploy/PRODUCTION_ROLLOUT.md`](../deploy/PRODUCTION_ROLLOUT.md)
for promotion requirements.

## v0.2.14 account-probe behavior

Manual provider tests now write a sanitized snapshot to
`account.extra.account_test_probe`. That snapshot is display/diagnostic state,
not scheduler state:

- A failed manual test does not change `account.status` or `schedulable`.
- Scheduled/background tests do not overwrite the latest manual result.
- Credentials and raw provider response bodies are never stored in the
  snapshot.
- Credential, account-type, route, proxy, or duplicate-account changes clear a
  stale result; OAuth access-token rotation alone does not.
- CRS synchronization preserves the local snapshot when stable provider
  identity is unchanged.
- The admin account table shows an accessible, localized **Probe Failed** badge
  independently of the active/schedulable badge.
- Probe classification distinguishes `model_unsupported` from
  `quota_exhausted` when the provider gives an invalid/not-found model signal.
- If a fresh Antigravity quota snapshot is already cached, an implicit account
  test prefers a recommended, non-exhausted text model. Explicit model choices
  remain authoritative, and the selector never performs a quota refresh.
- Provider response bodies are reduced to bounded status/classification errors
  before they reach probe SSE output or account error state.

Antigravity manual usage refreshes pass `force=true`, bypass both backend cache
checks, and use a distinct singleflight key so the operator action reaches the
provider. Generic Google 429 messages containing "resource has been exhausted"
are classified as quota exhaustion.

The detailed contract and troubleshooting flow are in
[`ACCOUNT_PROBES.md`](ACCOUNT_PROBES.md).

## Current external account condition

This is an observed provider condition, not a release-health failure, and will
become stale as upstream quota changes.

On 2026-08-20, the configured Antigravity OAuth account remained active and
schedulable. A forced usage query reached Google and returned a PRO tier with 24
model quota entries and no usage-query error. Manual inference probes for the
legacy default, an advertised Claude model, and an advertised Gemini model each
returned Google HTTP 429. ExAPI persisted the latest result as
`failed / quota_exhausted` without changing scheduler state.

Before declaring that provider account usable, repeat a manual probe against a
currently advertised model. Do not treat a successful quota-metadata request as
proof that inference is available.

## Documentation update checklist

For each release or production change:

1. Update this page's date, release table, production state, validation, and
   known external conditions.
2. Keep reusable instructions version-neutral and digest-pinned in `deploy/`.
3. Update [`development.md`](../development.md) when active priorities or gates
   change.
4. Update feature documentation when behavior or API contracts change.
5. Preserve OpenSpec and other explicitly historical evidence as immutable
   records; link a superseding document instead of rewriting history.
6. Run `git diff --check`, the release-contract check, and relevant deployment
   contract tests before publishing documentation changes.
