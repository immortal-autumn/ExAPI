# ExAPI Development Plan

This document records the active development direction for the ExAPI fork and
the mandatory quality gates between phases. The dated release/deployment
baseline lives in [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md).

## Current Goal

Build on the deployed v0.2.7 baseline while preserving runtime compatibility,
deployment compatibility, API compatibility, and private-control-plane
security. The review branch `revision/exapi-v0.2.1` is still non-production;
OPC remains on the attested v0.2.7 image until the recovery and rollout gates
below are complete.

The settings extraction described by the older Phase 1.2–1.7 plan has landed:
the reusable section shell, all major tab components, security/gateway/payment
panels, and focused tests now exist. `SettingsView.vue` is substantially smaller
but remains a coordinator with additional extraction opportunities. Do not
repeat completed phases.

Provider diagnostics, operator-only navigation, API-key lifecycle guards, and
the initial import hardening have landed on this branch. The immediate focus is
finishing the fail-closed migration evidence and release gates, while keeping
provider-vs-deployment failures easy to distinguish.

## Compatibility Boundaries

Do not rename or remove these during the refactor unless a separate migration plan is approved:

- Go module path: `github.com/Wei-Shaw/sub2api`
- Runtime/deployment paths such as `/opt/sub2api` and `/app/sub2api`
- Service/user/group/database/cache identifiers that intentionally still use `sub2api`
- Environment variable prefix `SUB2API_*`
- Existing API routes, gateway-compatible endpoints, and database schema semantics
- Private-control-plane behavior: public domains expose AI gateway endpoints only; admin/control UI stays localhost/WireGuard-only

## Next Development Phases

### Phase 2 status — completed on the review branch

The provider model-aware diagnostics, bounded/redacted probe state, API-key
idempotency contract, proxy update/import guards, and administrator frontend
boundary described in the Phase 2 criteria below are implemented and recorded
in [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md). Treat the Phase 2
sections as acceptance criteria and historical context; do not restart them
without a new regression finding.

### Phase 2.1 — Provider model-aware diagnostics

- Derive diagnostic model choices from a fresh provider capability/quota result
  when one exists.
- Keep an explicit operator-selected model authoritative.
- Distinguish unsupported/stale model IDs from real account-wide quota
  exhaustion when the provider supplies enough evidence.
- When a fresh Antigravity quota snapshot is already cached, use its
  recommended, non-exhausted text model for an implicit diagnostic; never
  trigger a quota refresh from the test path, and keep explicit `model_id`
  selections authoritative.
- Keep raw provider bodies out of account metadata, UI tooltips, and logs shown
  to ordinary operators.
- Add Antigravity, Gemini, Claude, and OpenAI-compatible regression cases.

### Phase 2.2 — Probe and quota observability

- Add bounded metrics for manual probe outcome/reason and forced-usage outcome.
- Preserve the separation between provider diagnostics and scheduler state.
- Document and test retry/backoff behavior so an operator refresh cannot create
  a provider retry storm.
- Keep account identity, token material, and raw upstream payloads out of
  metrics labels.
- Ensure periodic account-test workers claim due plans atomically with a
  database lease before provider calls; retain retry-after-lease-expiry and
  stale-worker protection tests so replica scaling cannot duplicate probes.

### Phase 2.3 — Finish coordinator reductions

- Continue reducing `SettingsView.vue` and other large coordinators only where
  behavior can be extracted behind focused tests.
- Do not rename settings keys, API fields, routes, or retained `sub2api`
  compatibility identifiers as part of structural work.
- Prefer one panel/composable extraction per reviewed commit.

### Phase 2.4 — Release and documentation discipline

- Update `docs/PROJECT_STATUS.md` for every release and production promotion.
- Keep deploy examples digest-pinned and keep mutable current-state facts out of
  generic procedures.
- Run the upstream-lock, release-contract, deployment-contract, backend, and
  frontend gates before tagging.
- Preserve historical OpenSpec evidence rather than rewriting it to match the
  current release.

### Phase 3 — Retired-surface and migration evidence hardening (completed checkpoint)

- Keep historical customer API prefixes on the explicit bilingual `410 Gone`
  contract without intercepting operator or gateway routes.
- Keep offline migration-report verification bounded and fail-closed on symlink,
  identity, size, mtime, inode, or content changes.
- Return structured proxy-import synchronization errors and matching failure
  counts for both reused and newly-created proxies.
- Keep operator batch-image functionality under `/admin/batch-images`; retain
  the legacy `/batch-image` bookmark as an explicit retirement page and keep
  route navigation/prefetch contracts covered by focused tests.
- Keep each Phase 3 change independently revertible, documented in English and
  Chinese, and green on the full GitHub CI/security workflow before proceeding.

Administrator API-key group writes are also guarded per key in the browser:
the group control is disabled until its request settles, preventing
out-of-order updates while preserving the operator-mode `group_id: 0` unbind
contract.

### Phase 4 — Recovery and private-cutover rehearsal (current; blocked on external inputs)

- Create an encrypted, versioned recovery set and restore it independently in a
  networkless disposable target.
- Verify the protected keyring, migration-report key, legacy-local-backup policy,
  zero batch-image rows/provider jobs, and immutable archive evidence.
- Run `deploy/ops/run-private-cutover.sh --dry-run` against the exact retained
  OPC Compose project and record its target digest without changing production.
- Do not perform the destructive private-only transaction until every external
  archive, snapshot, monitoring, and provider-cleanup adapter listed in
  `deploy/PRODUCTION_ROLLOUT.md` is available and independently verified.

### Phase 5 — Immutable release and isolated canary

- Tag only a commit that has passed the complete backend/frontend/deployment
  gates; build the multi-architecture OCI image by digest with SBOM and
  provenance attestations.
- Run the restored-data and synthetic-provider canary using the same image,
  database schema, control bind, and egress policy intended for production.
- Verify public/control listener separation, readiness, account/key routing, and
  rollback inputs before any production stop.

### Phase 6 — Controlled production promotion and observation

- Bind the real cutover to the reviewed dry-run target hash, exact image digest,
  Compose/environment files, WireGuard identities, and one-time confirmation
  token; keep PostgreSQL and Redis untouched.
- Independently archive and verify the signed report, verifier evidence, and key
  with immutable retention before starting the reviewed application container.
- Observe the stated readiness, restart, error-rate, p95, alert, topology, and
  provider smoke thresholds for the full window; retain rollback evidence.

## Mandatory Gate Between Every Phase

Between every phase, perform all of the following before starting the next phase.

### 1. Review the full diff

Run:

```bash
git diff --stat
git diff --check
git diff
```

Review for:

- Accidental behavior changes
- Renamed API fields or settings keys
- Deleted validation paths
- Lost loading/error/disabled states
- Lost accessibility attributes
- Lost i18n keys or Chinese-only UI assumptions
- Any change to deployment/runtime compatibility boundaries

### 2. Review every modified function/component

For each changed file, inspect all modified functions, components, props, emits, watchers, computed values, and side effects.

Minimum checks:

- Does the function/component still receive the same data?
- Does it still emit/save the same payload?
- Are validation and error messages preserved?
- Are loading, disabled, and optimistic update states preserved?
- Are secrets still masked and never logged?
- Are public/private control-plane checks unchanged?

### 3. Run targeted tests for changed functionality

Run tests directly related to the modified files. For SettingsView refactors, include at least:

```bash
cd frontend
./node_modules/.bin/vitest run \
  src/views/admin/settings/__tests__/useSettingsTabs.spec.ts \
  src/i18n/__tests__/zh-only.spec.ts \
  src/config/__tests__/brand.spec.ts \
  src/i18n/__tests__/brand-copy.spec.ts \
  src/router/__tests__/title.spec.ts \
  src/stores/__tests__/app.spec.ts \
  src/utils/__tests__/singleUserCockpit.spec.ts \
  src/views/__tests__/KeyUsageView.spec.ts
```

Add new tests for any extracted composable, helper, or behavior-bearing component.

### 4. Run frontend typecheck and build

Run:

```bash
cd frontend
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build
```

The Vite chunk-size warning is acceptable during early extraction, but type errors or build failures are blockers.

### 5. Run project safety checks

Run from repository root:

```bash
.hermes/scripts/check-exapi-brand.sh
python3 .hermes/scripts/audit-large-files.py | head -30
```

Confirm the large-file audit is moving in the expected direction and no brand/compatibility guard fails.

### 6. Run backend checks if backend or shared API contracts changed

If a phase touches backend code, shared API types, generated API clients, routing, auth, payment, settings payloads, or embedded frontend output, run relevant backend checks:

```bash
cd backend
GOTOOLCHAIN=auto go test ./internal/brand ./internal/setup ./internal/service ./internal/server ./internal/repository ./internal/payment/provider
GOTOOLCHAIN=auto go test -tags embed ./internal/web
```

For middleware/routes/auth changes, also run:

```bash
GOTOOLCHAIN=auto go test -tags unit ./internal/server/middleware ./internal/server/routes ./internal/handler
```

### 7. Commit only after the gate passes

Each phase should be committed separately after review and verification:

```bash
git add <phase files>
git commit -m "refactor: <specific phase summary>"
```

Do not batch multiple phases into one commit unless the user explicitly asks.

### 8. Push only verified commits

After committing a verified phase:

```bash
git push
```

Do not push unverified code.

## Definition of Done for a Phase

A phase is done only when:

- The intended extraction/refactor is complete.
- No public API, deployment, database, environment variable, or private-control-plane behavior changed unintentionally.
- All modified files and functions were reviewed.
- Targeted tests passed.
- Typecheck passed.
- Build passed when frontend is touched.
- Brand/refactor audit checks passed.
- The phase has a focused commit.
- The branch has been pushed if the work is intended for the active PR.

## Current Recommended Next Step

1. Keep `72aa62eea` as the reviewed Phase 3 checkpoint after its successful
   GitHub CI and security scan; do not deploy the review branch directly.
2. Resolve the external prerequisites in Phase 4 and execute the documented
   dry-run against `/opt/sub2api` before creating a release tag.
3. If a canary or operator review finds a new defect, add a failing focused test
   and a separate revertible commit, then repeat the mandatory phase gate.
4. Update `docs/PROJECT_STATUS.md` and its Chinese mirror for every release,
   promotion, rollback, or changed operational invariant.
