# ExAPI project status

This is the canonical, dated status record for the ExAPI fork. Update it when a
release is published, production is promoted or rolled back, an operational
invariant changes, or a known external blocker is resolved. Generic procedures
belong in `deploy/`; this page records the currently reviewed facts.

## Current release

Last reviewed: **2026-08-24 (Europe/London)**

| Item | Current value |
|---|---|
| Product version | `0.2.6` |
| GitHub repository | `immortal-autumn/ExAPI` |
| Git tag | `v0.2.6` |
| Main branch | `main` (fast-forwarded to the reviewed commit) |
| Release branch | `revision/exapi-v0.2.1` |
| Reviewed commit | `8363e0decd68786e02c9620e616e17f1284e0ff2` |
| OCI image | `ghcr.io/immortal-autumn/sub2api2personal@sha256:5ef74f0df89989ae7922fa819ac67ea159c8769871173fba33548baf0a708b43` |
| GitHub release | <https://github.com/immortal-autumn/ExAPI/releases/tag/v0.2.6> |
| Release workflow | <https://github.com/immortal-autumn/ExAPI/actions/runs/32739586602> |
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

## Production deployment

Production runs the digest above as Docker Compose project `sub2api` from
`/opt/sub2api`. The application container reports version `0.2.6`, the reviewed
commit, healthy status, and zero restarts after promotion and observation.

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
