# ExAPI Development Plan

This document records the active refactor direction for the ExAPI fork and the mandatory quality gates between phases.

## Current Goal

Continue the conservative whole-project refactor while preserving runtime compatibility, deployment compatibility, API compatibility, and private-control-plane security.

The immediate focus is `frontend/src/views/admin/SettingsView.vue`, currently the largest hand-maintained frontend file. The refactor should turn it into a coordinator component while moving tab content, repeated section shells, and panel logic into smaller tested modules.

## Compatibility Boundaries

Do not rename or remove these during the refactor unless a separate migration plan is approved:

- Go module path: `github.com/Wei-Shaw/sub2api`
- Runtime/deployment paths such as `/opt/sub2api` and `/app/sub2api`
- Service/user/group/database/cache identifiers that intentionally still use `sub2api`
- Environment variable prefix `SUB2API_*`
- Existing API routes, gateway-compatible endpoints, and database schema semantics
- Private-control-plane behavior: public domains expose AI gateway endpoints only; admin/control UI stays localhost/WireGuard-only

## Next Refactor Phases

### Phase 1.2 — Extract reusable settings card shell

Create a reusable settings section wrapper:

- `frontend/src/views/admin/settings/SettingsSectionCard.vue`

Use it first on only one or two low-risk settings sections. Do not perform a broad markup rewrite in one commit.

### Phase 1.3 — Extract General settings tab

Create:

- `frontend/src/views/admin/settings/tabs/GeneralSettingsTab.vue`

Move general/site-identity settings out of `SettingsView.vue`, including site name, logo, documentation URL, subtitle, and home content fields.

Keep parent form state and save payload unchanged at first. This phase is a structural extraction, not a behavior change.

### Phase 1.4 — Extract Agreement and Feature settings tabs

Create:

- `frontend/src/views/admin/settings/tabs/AgreementSettingsTab.vue`
- `frontend/src/views/admin/settings/tabs/FeatureSettingsTab.vue`

Move existing markup and wiring only. Do not rename settings keys or alter API payload structure.

### Phase 1.5 — Split Security tab

Split the existing `security` tab into smaller panels while keeping the visible tab key unchanged:

- Admin API key panel
- Registration / email verification / invitation settings panel
- CAPTCHA / Turnstile panel
- Third-party login / OAuth / OIDC panel

Suggested paths:

- `frontend/src/views/admin/settings/security/AdminApiKeyPanel.vue`
- `frontend/src/views/admin/settings/security/RegistrationSecurityPanel.vue`
- `frontend/src/views/admin/settings/security/ThirdPartyAuthPanel.vue`
- `frontend/src/views/admin/settings/tabs/SecuritySettingsTab.vue`

### Phase 1.6 — Split Gateway tab

Split the existing `gateway` tab into smaller panels while keeping the visible tab key unchanged:

- Gateway runtime behavior, cooldowns, timeout, and retry settings
- Protocol/client compatibility options such as Claude/Codex/OpenAI behavior
- Scheduler and routing settings

Suggested paths:

- `frontend/src/views/admin/settings/gateway/GatewayRuntimePanel.vue`
- `frontend/src/views/admin/settings/gateway/GatewayProtocolPanel.vue`
- `frontend/src/views/admin/settings/gateway/GatewaySchedulerPanel.vue`
- `frontend/src/views/admin/settings/tabs/GatewaySettingsTab.vue`

### Phase 1.7 — Extract remaining settings tabs

Extract the remaining tabs after the heavier security/gateway work is stable:

- `UserSettingsTab.vue`
- `PaymentSettingsTab.vue`
- `EmailSettingsTab.vue`
- `BackupSettingsTab.vue`

The target is for `SettingsView.vue` to become a small coordinator component rather than an 11k-line implementation file.

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

Proceed with **Phase 1.2**:

1. Add `SettingsSectionCard.vue`.
2. Replace only one or two obvious repeated settings cards.
3. Review the diff.
4. Run the mandatory phase gate.
5. Commit and push.

Then continue to Phase 1.3 only after the gate passes.
