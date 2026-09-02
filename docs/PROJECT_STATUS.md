# ExAPI project status

This is the canonical, dated status record for the ExAPI fork. Update it when a
release is published, production is promoted or rolled back, an operational
invariant changes, or a known external blocker is resolved. Generic procedures
belong in `deploy/`; this page records the currently reviewed facts.

## Current release

Last reviewed: **2026-09-02 (Europe/London)**

| Item | Current value |
|---|---|
| Product version | `0.2.10` (released and deployed to OPC on 2026-09-02) |
| GitHub repository | `immortal-autumn/ExAPI` |
| Git tag | `v0.2.10` |
| Main branch | `main` (currently `f79ef301b`; release branch remains separate) |
| Release branch | `revision/exapi-v0.2.1` |
| Reviewed commit | `f8cee085c3e7d815d5144659d3a92720f9aa8e95` |
| OCI image | `ghcr.io/immortal-autumn/sub2api2personal@sha256:c56c876f70c49d3f05dffc7fc80417807b043da1d1261d1e3fc29b7a0daaeaa8` |
| GitHub release | <https://github.com/immortal-autumn/ExAPI/releases/tag/v0.2.10> |
| Release workflow | <https://github.com/immortal-autumn/ExAPI/actions/runs/33584593736> |
| Upstream baseline | Sub2API `v0.1.171`, constrained by `upstream.lock.json` |

The GitHub repository was renamed from `Sub2API2Personal` to `ExAPI` on
2026-08-20. The existing v0.2.5 GHCR package path remains
`sub2api2personal` for image and deployment compatibility; future release work
must keep that compatibility decision explicit or publish a separately
verified package migration.

The v0.2.10 release workflow passed the Go module-tidy check, backend unit and
integration suites, race detector, frontend tests/type checks/build audit,
multi-architecture image build, OCI label verification, SPDX SBOM generation,
and provenance/SBOM attestations. The attested manifest digest is the same
digest recorded above, and `gh attestation verify` passed. The v0.2.9 release
remains available as the preceding reviewed candidate. The prior v0.2.8 tag
remains immutable but was never published because its commit still contained
`backend/cmd/server/VERSION=0.2.7`; it must not be retagged.

## Administrator hardening review branch

The non-production review branch `revision/exapi-v0.2.1` has continued
administrator-surface hardening through 2026-09-01. The following independently
revertible commits have passed their local focused/full checks and GitHub CI and
security scans:

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
clear nullable settings. This work is not part of the production release above
and has not been deployed. Those review-only changes remain excluded from the
v0.2.10 production image.

The latest review-only commits add a bilingual administrator-only product
surface contract (`bdd4329e3`), make all supported deployment examples default
to `RUN_MODE=simple` while preserving the backend's legacy `standard` fallback,
and derive OAuth/setup-token account display names from an explicit name,
validated email, or provider fallback (`8b25ba36c`). Focused account/private-route
tests, frontend typecheck/build, deployment bind/security/rollout contracts, and
the release contract passed. These commits are on
`revision/exapi-v0.2.1` only and have **not** been deployed to OPC or included in
the v0.2.10 image.

The latest review-only commit `da3e56788` exposes the retained private security
boundary in the administrator settings view, prevents a read-only security tab
from triggering a global settings save, and localizes the affected security,
OAuth, pagination, and 404 copy in English and Simplified Chinese. It adds
regression coverage for the security tab and retained-tab contract; the full
frontend suite (267 files, 1,435 tests), typecheck, lint, production build,
bundle budgets, and deployment contracts passed. This commit is on
`revision/exapi-v0.2.1` only and has **not** been deployed to OPC or included in
the v0.2.10 image.

## Production deployment

Production is deployed as Docker Compose project `sub2api` from `/opt/sub2api`
using the versioned inputs `/opt/sub2api/.env.v0.2.10` and
`/opt/sub2api/docker-compose.v0.2.10.yml`. The application is pinned to
`ghcr.io/immortal-autumn/sub2api2personal@sha256:c56c876f70c49d3f05dffc7fc80417807b043da1d1261d1e3fc29b7a0daaeaa8`,
revision `f8cee085c3e7d815d5144659d3a92720f9aa8e95`, and reports version
`0.2.10`. App, PostgreSQL, and Redis are healthy; PostgreSQL/Redis were not
recreated and all three services have zero restarts in the final observation.

The pre-cutover recovery set `exapi-v0210-recovery-20260902e` is retained in
encrypted, versioned off-host objects. Its logical object version is
`a236405f-4138-4860-8d2c-42eee5f98a0b`, snapshot ID is
`54b97de2-1731-449f-b339-11fcdead04a7`, and secrets object version is
`2d39e24e-f329-46fd-9254-029d68bc10af`; retention is through 2027-09-03.
The restored-data canary is explicitly bound to the independently restored
`exapi-v0210-recovery-20260902c` volume (logical version
`13f467e3-26df-4059-ba4b-ea4dd7a9d5c3`); the e set is the fresh refresh taken
immediately before cutover and is the rollback snapshot/keyroot source.
The completed private cutover archive is verified, encrypted, and immutable at
`tmp/rollouts/exapi-v0210-cutover-20260902a/private-cutover-resume/private-migration-archive.json`
on OPC.
The snapshot-evidence adapter quiescence/checkpoint fix used by this rollout is
tracked as commit `56691bdb5` on the release branch.

The 2026-09-02 external prerequisite run refreshed the off-host readiness
monitor and synthetic-provider proof. A critical `/ready` transition returned
503 and was delivered at 06:06:50Z; recovery returned 200 and was delivered at
06:07:12Z. No credentials or database contents are stored in this repository.

The final rollout manifest is recorded locally at
`tmp/rollout-manifest-v0.2.10.json` (SHA-256
`bf4b22b86f1abf1f69840d7a639f88ce646215a51012481f84fb24ac259860fd`) and was
validated and signed on OPC with the protected cosign key. The signed record is
retained under `s3://exapi-rollout-records/exapi-v0210-production-20260902e`
with COMPLIANCE retention through 2027-09-03. Object versions are:
`manifest.json` `a49e2e47-7cd8-498c-aa1d-e865f515044d`, checksum
`544c19b0-db1e-4314-9877-a35284929e73`, signature bundle
`eaecb12c-7674-4f8c-b436-ad826455a0fa`, and provenance evidence
`7a3b55dc-c324-432b-aed6-bc16d24341a6`.

Post-publication local gates remain green: frontend Vitest `266` files/
`1432` tests passed, `vue-tsc` typecheck passed, both bundle budgets passed,
the snapshot-evidence unit suite passed (`5` tests), and the production-rollout
contract passed.

The production observation ran for 60 minutes with 120 readiness probes:

- readiness failures: `0`; restarts: `0`; unexpected 5xx: `0`;
- error rate: `0.0`; p95: `1.0 ms` versus a `4.852 ms` baseline;
- new P0/P1 alerts: `0`; production topology and dependency identities: verified.

The final allowlisted peer was `100.97.17.2`: control `/ready`,
`/api/v1/operator/me`, and a read-only account-list request returned 200. Public
`/ready` remained 200, while the public root and public control route returned
404.

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

## v0.2.10 account-probe behavior

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
