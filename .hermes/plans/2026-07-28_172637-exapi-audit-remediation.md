# ExAPI Audit Remediation Campaign

> **For Hermes:** Implement with phase-gated TDD. Each phase must have RED, GREEN, focused/broad gates, scoped staging, independent exact-SHA review, and a separate commit. Do not deploy, restart services, or mutate production.

**Goal:** Remediate every evidence-backed issue from the exact-SHA review of `9421fa762de50013526aec9241b6fde9553e0818` while preserving ExAPI's private, single-user, Chinese-only operator gateway.

**Architecture:** Security boundaries are fixed first, followed by credential-storage foundations, worker lifecycle, product-mode cleanup/localization, release provenance/CI, and operational readiness. Persisted-secret changes remain additive and dual-read until a separately authorized production migration; no live schema or data change is part of this campaign.

**Tech stack:** Go 1.26.5, Gin, Ent/PostgreSQL, Redis, Vue 3, TypeScript, Vitest, pnpm, Docker/Compose, GitHub Actions.

---

## Baseline and safety sentinel

- Captured: `2026-07-28T17:26:37Z`
- Baseline HEAD: `9421fa762de50013526aec9241b6fde9553e0818`
- Branch: `feat/local-admin-bypass`
- Tracking: `origin/feat/local-admin-bypass`
- Modified/staged/untracked: `0/0/0`
- Backend/frontend/deploy-area changes: `0/0/0`
- Runtime sentinel: **BLOCKED** — prior read-only container inspection was denied by the execution environment. Do not retry the denied production-inspection command or substitute config/secret reads. Source work must remain no-deploy/no-restart; final report must state runtime sentinel unavailable rather than claim it unchanged.
- Forbidden: production database/Redis access, applying migrations to `/opt/sub2api`, container/service restart, image deployment, reading or printing secret values.
- Disposable test infrastructure is allowed only under `/tmp` with synthetic credentials.

## Disposition vocabulary

- `FIXED`: acceptance tests and required gates pass at a reviewed exact SHA.
- `REJECTED`: deterministic evidence disproves the finding, with rationale.
- `DEFERRED-BLOCKED`: requires unavailable platform or explicitly unauthorized production action.
- `ACCEPTED-RISK`: explicit user acceptance only; never inferred.

## Issue ledger and phases

### Phase A — Outbound security boundary

- **EXAPI-SEC-01: Content-moderation authenticated SSRF — OPEN**
  - RED: loopback/RFC1918/link-local/IPv6 ULA, redirect-to-private, and DNS-rebinding tests.
  - GREEN: save-time URL policy plus dial-time IP pinning and redirect revalidation; explicit private CIDR opt-in only if required.
  - Gates: focused service tests; `go test ./internal/service ./internal/repository`; `go vet`.
- **EXAPI-SEC-02: Generic upstream DNS validation/dial TOCTOU — OPEN**
  - RED: resolver changes public validation answer to private dial answer.
  - GREEN: safe transport pins validated addresses and revalidates redirects/proxy behavior.

### Phase B — Persisted credential protection foundation

- **EXAPI-CRED-01: Admin API key plaintext — OPEN**
  - Add dedicated HMAC-verifier keyring/purpose, nullable verifier and display-prefix storage, new-write/dual-read behavior, one-time disclosure, value-free preflight/backfill command, rotation tests, and migration documentation.
- **EXAPI-CRED-02: Moderation credentials plaintext — OPEN**
  - Encrypt with a dedicated purpose-bound external keyring; support legacy dual read and explicit backfill.
- **EXAPI-CRED-03: Proxy passwords plaintext — OPEN**
  - Add row-bound authenticated envelopes and transaction-safe create/update/bulk paths; redact non-operational projections.
  - Migration is additive only. Do not erase live legacy values or apply schema in production.
  - Gates: crypto unit tests, repository/service tests, disposable PostgreSQL mixed-state/rollback/rotation tests, generated Ent/Wire checks, broad backend suite, independent crypto review.

### Phase C — Worker lifecycle and shutdown

- **EXAPI-LIFE-01: Unjoined content-moderation workers — OPEN**
- **EXAPI-LIFE-02: Unjoined concurrency cleanup — OPEN**
- **EXAPI-LIFE-03: Missing timing-wheel/deferred/user-message cleanup — OPEN**
  - RED: initialized workers remain alive after cleanup.
  - GREEN: cancel + join for every process-lifetime goroutine; add ordered bounded cleanup entries before Redis/Ent.
  - Gates: focused cleanup tests, repeated `-race`, command/server suite.

### Phase D — Private product and Chinese-only frontend

- **EXAPI-PROD-01: Dormant Home Content control/public injection/CSP side effect — OPEN**
  - Remove private control, deny save field, clear/omit anonymous response/injection, ignore stale value in CSP, keep `/home` compatibility redirect.
- **EXAPI-PROD-02: Dormant SaaS imports/dialogs in retained Settings root — OPEN**
  - Remove private build-root imports/mounts without deleting gateway primitives.
- **EXAPI-L10N-01: Server-injected English title — OPEN**
- **EXAPI-L10N-02: Raw role and account-test English prose — OPEN**
- **EXAPI-L10N-03: Usage/setup English fallbacks — OPEN**
  - RED/GREEN per reachable surface; technical identifiers remain unchanged.
  - Gates: mounted component tests, server-rendered embed tests, private public-settings schema tests, route/network/build graph tests, full affected specs, typecheck, build, bundle inspection.

### Phase E — Test and dependency gates

- **EXAPI-TEST-01: Isolated frontend specs fail — OPEN**
  - Repair tests to drive public UI behavior or intentionally exposed stable test interfaces; no order dependence.
- **EXAPI-TEST-02: Coverage thresholds not enforced — OPEN**
  - Correct Vitest coverage configuration; add a failure-path contract test.
- **EXAPI-TEST-03: Gin global-mode test race — OPEN**
  - Set mode once before parallel tests and require scoped race CI.
- **EXAPI-DEP-01: Unexcepted PostCSS advisories — OPEN**
  - Upgrade lock to `>=8.5.18`; do not create an exception unless remediation is impossible and explicitly accepted.
- **EXAPI-DEP-02: Xlsx accepted advisories expire 2026-10-06 — OPEN**
  - Replace or isolate dependency; retain explicit documented risk only until replacement passes.
- **EXAPI-HYGIENE-01: `go mod tidy -diff` — OPEN**
  - Remove stale checksums and enforce tidy check.

### Phase F — CI and artifact provenance

- **EXAPI-CI-01: CI runs only six frontend tests — OPEN**
- **EXAPI-CI-02: CI omits production build/bundle gate — OPEN**
- **EXAPI-CI-03: Release can bypass validation/full gates — OPEN**
- **EXAPI-CI-04: Mutable action/scanner versions — OPEN**
- **EXAPI-REL-01: Deployment paths reference upstream/mutable artifacts — OPEN**
- **EXAPI-REL-02: Reviewed SHA absent from remote — DEFERRED-BLOCKED until authenticated push authorization/path is available**
  - Source fixes: ExAPI-owned repository variables, immutable SHA/digest contracts, version metadata, exact artifact checks, full required release gate, pinned Actions/scanners.
  - Never push or publish without explicit user authorization if credentials/remote side effects are required.

### Phase G — Operational readiness

- **EXAPI-OPS-01: Static liveness used as readiness — OPEN**
  - Add separate bounded PostgreSQL/Redis readiness endpoint and Compose health contract.
- **EXAPI-OPS-02: Binary-only rollback lacks schema compatibility proof — DEFERRED-BLOCKED for live data; source/docs/test harness OPEN**
  - Add compatibility metadata/docs and disposable previous-version upgrade/use/rollback drill.
- **EXAPI-OPS-03: Migration/restore drill — DEFERRED-BLOCKED for production; disposable drill OPEN**
- **EXAPI-OPS-04: Metrics/tracing, SBOM/signatures/reproducibility — OPEN as release hardening**
- **EXAPI-SETUP-01: Setup probes permitted to broad admitted CIDRs — OPEN**
  - Add one-time setup authorization and/or explicit destination policy plus rate limiting without blocking legitimate remote DB/Redis setup.

## Mandatory phase log

For every phase append:

- candidate base and resulting exact SHA;
- issue dispositions;
- RED command/result;
- GREEN command/result;
- changed and staged files;
- focused and broad gate results;
- static secret/dangerous-code scan;
- independent reviewer verdict for exact staged/committed diff;
- source cleanliness;
- deployment status: always `NOT DEPLOYED` unless the user separately authorizes it.

## Campaign closure

Release-ready means all non-blocked issues are `FIXED` or explicitly `ACCEPTED-RISK`, full default/tagged/race/frontend/build/security gates pass from a disposable exact-SHA checkout, all artifacts identify and derive from that SHA, an independent exact-SHA review approves, and production deployment remains a separate explicit authorization step.
