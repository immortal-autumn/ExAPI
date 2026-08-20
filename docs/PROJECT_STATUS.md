# ExAPI project status

This is the canonical, dated status record for the ExAPI fork. Update it when a
release is published, production is promoted or rolled back, an operational
invariant changes, or a known external blocker is resolved. Generic procedures
belong in `deploy/`; this page records the currently reviewed facts.

## Current release

Last reviewed: **2026-08-20 (Europe/London)**

| Item | Current value |
|---|---|
| Product version | `0.2.5` |
| Git tag | `v0.2.5` |
| Release branch | `revision/exapi-v0.2.1` |
| Reviewed commit | `14a7c412a17971b160de356baaab7a3555fb90fa` |
| OCI image | `ghcr.io/immortal-autumn/sub2api2personal@sha256:2fc5b6c06ba8e118302ce00f1778064fd5ac253f46c60d90cffb51c350953cfe` |
| GitHub release | <https://github.com/immortal-autumn/Sub2API2Personal/releases/tag/v0.2.5> |
| Release workflow | <https://github.com/immortal-autumn/Sub2API2Personal/actions/runs/32358414952> |
| Upstream baseline | Sub2API `v0.1.171`, constrained by `upstream.lock.json` |

The release workflow passed the Go module-tidy check, backend unit and
integration suites, race detector, frontend tests/type checks/build audit,
multi-architecture image build, OCI label verification, SPDX SBOM generation,
and provenance/SBOM attestations.

## Production deployment

Production runs the digest above as Docker Compose project `sub2api` from
`/opt/sub2api`. The application container reports version `0.2.5`, the reviewed
commit, healthy status, and zero restarts at the time of this review.

The deployment keeps versioned promotion inputs:

- `/opt/sub2api/.env.v0.2.5`
- `/opt/sub2api/docker-compose.v0.2.5.yml`
- `/opt/sub2api/.env` and `docker-compose.local.yml` point to the same current
  configuration.
- The v0.2.4 environment and Compose snapshots remain available for rollback.

PostgreSQL and Redis were preserved during the application-only promotion and
remained healthy. The restored-data and isolated synthetic-provider canaries
also ran the v0.2.5 digest successfully before production promotion.

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

## v0.2.5 account-probe behavior

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
