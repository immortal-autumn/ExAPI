# ExAPI project status

This is the canonical, dated status record for the ExAPI fork. Update it when a
release is published, production is promoted or rolled back, an operational
invariant changes, or a known external blocker is resolved. Generic procedures
belong in `deploy/`; this page records the currently reviewed facts.

## Current release

Last reviewed: **2026-09-01 (Europe/London)**

| Item | Current value |
|---|---|
| Product version | `0.2.7` |
| GitHub repository | `immortal-autumn/ExAPI` |
| Git tag | `v0.2.7` |
| Main branch | `main` (fast-forwarded to the reviewed commit) |
| Release branch | `revision/exapi-v0.2.1` |
| Reviewed commit | `a1c8bb6a7a4e49d67fbdb81aadc67de4ef12e7c1` |
| OCI image | `ghcr.io/immortal-autumn/sub2api2personal@sha256:628dbccd43e5348989ae83c6c7c494bcbe85227824a1acfedee10f77dd7f1795` |
| GitHub release | <https://github.com/immortal-autumn/ExAPI/releases/tag/v0.2.7> |
| Release workflow | <https://github.com/immortal-autumn/ExAPI/actions/runs/33308484734> |
| Upstream baseline | Sub2API `v0.1.171`, constrained by `upstream.lock.json` |

The GitHub repository was renamed from `Sub2API2Personal` to `ExAPI` on
2026-08-20. The existing v0.2.5 GHCR package path remains
`sub2api2personal` for image and deployment compatibility; future release work
must keep that compatibility decision explicit or publish a separately
verified package migration.

The release workflow passed the Go module-tidy check, backend unit and
integration suites, race detector, frontend tests/type checks/build audit,
multi-architecture image build, OCI label verification, SPDX SBOM generation,
and provenance/SBOM attestations. The attested manifest digest is the same
digest recorded above; the release also passed `gh attestation verify`.

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

The review branch also contains work hardening proxy partial updates: omitted
lifecycle and credential fields are preserved, while explicit null/empty values
clear nullable settings. This work is not part of the production release above
and has not been deployed. Production remains on the reviewed v0.2.7 digest
until a separately validated release is promoted under the rollout procedure.

## Production deployment

Production runs the digest above as Docker Compose project `sub2api` from
`/opt/sub2api`. The application container reports version `0.2.7`, the reviewed
commit, healthy status, and zero restarts at the 2026-09-01 read-only check.

The deployment keeps versioned promotion inputs:

- `/opt/sub2api/.env.v0.2.6`
- `/opt/sub2api/docker-compose.v0.2.6.yml`
- `/opt/sub2api/.env` and `docker-compose.local.yml` retain the legacy local
  provenance and v0.2.5 rollback digest; promotion uses the versioned files
  above explicitly.
- The v0.2.5 environment, Compose file, and digest remain available for
  application-only rollback.

PostgreSQL and Redis were preserved during the application-only promotion and
remained healthy; their container IDs and start times did not change. The
isolated v0.2.6 synthetic-provider canary passed readiness, provider gateway
smoke, internal-network/egress checks, and disposable private migration before
promotion.

The v0.2.6 recovery set was created under rollout
`exapi-v026-production-20260824a`: the encrypted logical dump and physical
snapshot were retained off-host and independently restored into networkless
disposable targets. Evidence is under the checkout-local `tmp/rollouts/`
directory on OPC.

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

## v0.2.6 account-probe behavior

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
