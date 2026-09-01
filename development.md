# ExAPI Development Plan

This document records the active development direction for the ExAPI fork and
the mandatory quality gates between phases. The dated release/deployment
baseline lives in [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md).

## Current Goal

Build on the deployed v0.2.6 baseline while preserving runtime compatibility,
deployment compatibility, API compatibility, and private-control-plane
security.

The settings extraction described by the older Phase 1.2–1.7 plan has landed:
the reusable section shell, all major tab components, security/gateway/payment
panels, and focused tests now exist. `SettingsView.vue` is substantially smaller
but remains a coordinator with additional extraction opportunities. Do not
repeat completed phases.

The immediate focus is provider diagnostics and operator clarity: keep manual
probe state independent from scheduling, make live usage queries truthfully
reach providers, select currently advertised models for diagnostics, and make
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

Proceed with **Phase 2.1**:

1. Reproduce the case where forced Antigravity quota metadata succeeds but
   inference for an advertised model returns 429.
2. Capture only sanitized evidence and determine whether the provider exposes a
   reliable unsupported-model vs quota-exhausted discriminator.
3. Write failing focused tests before changing classification or model
   selection.
4. Run the mandatory phase gate and independent review.
5. Update `docs/ACCOUNT_PROBES.md` and `docs/PROJECT_STATUS.md` with any changed
   contract before committing.
