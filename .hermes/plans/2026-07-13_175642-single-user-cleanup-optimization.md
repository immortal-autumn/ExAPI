# ExAPI Single-User Cleanup and Optimization Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Turn the current ExAPI fork into a smaller, clearer, faster private single-user gateway control plane by removing inactive SaaS/customer surfaces, simplifying private-admin navigation and settings, unregistering unnecessary backend routes, and optimizing the remaining high-value workflows without breaking gateway compatibility.

**Architecture:** Use a phase-gated strangler cleanup. First lock down the desired product surface with tests, then remove active frontend references and background calls, then unregister unnecessary backend route families, and only after reference audits delete dead source files. Preserve database tables and internal routing-group semantics during this plan to avoid destructive migrations; optimize bundles and request paths only after functional cleanup establishes a reliable baseline.

**Tech Stack:** Vue 3, TypeScript, Pinia, Vue Router, Vite, Vitest; Go 1.26.5, Gin, Ent, PostgreSQL, Redis; Docker/Compose; curl/browser runtime checks.

---

## 1. Target Product in Plain Language

After this plan, ExAPI should feel like:

> A private control panel for managing upstream AI accounts, gateway keys, routing, usage, errors, security, and backups.

It should no longer feel like:

> A public SaaS platform for registering users, selling subscriptions, taking payments, issuing coupons, or running affiliates.

### Final primary navigation

```text
Dashboard
Ops
Accounts
API Keys
Usage
Gateway Settings
```

Conditional entries:

```text
Proxies           only if proxy support is actively configured
Channel Monitor   only if channel monitoring is actively used
Risk Control      only if content moderation is actively enabled
```

Settings should contain:

```text
General
Security
Gateway
Operator Alerts
Backup
```

### Gateway-critical domains that must remain

- Upstream account management.
- OpenAI/Gemini/Antigravity/Grok upstream OAuth account authorization.
- API key creation, editing, revocation, quota, concurrency, and IP restrictions.
- Routing groups used by API keys and accounts, including `openai-default`.
- Gateway routes and model compatibility.
- Usage records, errors, metrics, ops monitoring, and scheduled account tests.
- Proxy/TLS fingerprint/error-passthrough support where used.
- Backup/restore and operator security.

### SaaS/customer domains to remove from the active product

- Public registration and customer onboarding.
- User-management UI and customer profile flows.
- Customer subscriptions, balances, plans, orders, and payments.
- Redeem codes, promo codes, affiliates, and announcements.
- Customer lifecycle email panels.
- Public marketing, checkout, and customer portal routes.
- In-app update, rollback, and restart.

---

## 2. Safety Boundaries and Non-Goals

### Preserve during all phases

- Public domain exposes gateway endpoints only.
- Private control plane remains on localhost/WireGuard.
- API-key authentication remains required publicly.
- Local/WireGuard admin bypass remains private.
- Existing API keys continue to work.
- Existing upstream accounts and OAuth credentials remain intact.
- Existing group IDs and group records remain intact because groups are gateway routing primitives.
- Existing usage records remain readable.
- Go module path remains `github.com/Wei-Shaw/sub2api`.
- Runtime paths, `SUB2API_*` environment variables, PostgreSQL database, and Redis data remain compatible.

### Explicit non-goals for this plan

- Do not drop database tables or columns.
- Do not write destructive Ent migrations.
- Do not rename the Go module or runtime directories.
- Do not redesign gateway protocol behavior.
- Do not add user management, billing, payment, or self-update capabilities.
- Do not touch `precision` or CIV1/CIV2 configuration.
- Do not combine cleanup with a visual rebrand.

### Why schema deletion is deferred

Removing old UI/routes gives almost all security, maintenance, and bundle-size benefits without risking irreversible data loss. Database/schema deletion should be a separate optional plan after at least one stable release and a verified backup.

---

## 3. Phase Workflow

Each phase must follow this gate:

1. Write or update focused tests.
2. Run tests and observe expected failure where applicable.
3. Implement only that phase.
4. Review every changed function/component and side effect.
5. Run targeted tests.
6. Run frontend typecheck/build or backend tests as appropriate.
7. Run public/private route probes.
8. Stage only phase files.
9. Review staged diff and secret/security patterns.
10. Commit and push.
11. Stop for review before the next phase.

Never accumulate several phases into one large commit.

---

# Phase 0: Baseline and Acceptance Harness

## Task 0.1: Record the current source, image, and bundle baseline

**Objective:** Establish measurements so cleanup and optimization can be proven rather than assumed.

**Files:**
- Create: `.hermes/audits/single-user-cleanup-baseline.md`
- Reuse: `.hermes/scripts/audit-large-files.py` if present

**Step 1: Capture source and runtime identity**

Run:

```bash
cd /home/opc/src/sub2api
git status --short
git rev-parse HEAD
git log -10 --oneline
sudo docker inspect sub2api --format 'image={{.Config.Image}} image_id={{.Image}} health={{.State.Health.Status}} started={{.State.StartedAt}}'
```

Expected:

- Current commit and live image ID recorded.
- Existing untracked plan files documented but not staged accidentally.

**Step 2: Capture frontend bundle baseline**

Run:

```bash
cd /home/opc/src/sub2api/frontend
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build
find ../backend/internal/web/dist/assets -maxdepth 1 -type f -printf '%s %f\n' | sort -nr | head -30
```

Record at minimum:

- Main JS/CSS size.
- `AccountsView` chunk size.
- `SettingsView` chunk size.
- `OpsDashboard` chunk size.
- Whether payment/subscription/affiliate/redeem/announcement chunks exist.

**Step 3: Capture browser request baseline**

For a fresh load of `/admin/dashboard`, `/admin/accounts`, `/keys`, and `/admin/settings`, record:

- Number of XHR/fetch requests.
- Requests to payment/subscription/announcement APIs.
- Total transferred JS.
- Largest lazy chunk.

Use browser performance entries or DevTools network export. Do not record secrets.

**Step 4: Save baseline and commit**

```bash
git add -f .hermes/audits/single-user-cleanup-baseline.md
git commit -m "docs: capture single-user cleanup baseline"
```

---

## Task 0.2: Add a single-user product-surface manifest

**Objective:** Define one source of truth for allowed private UI and backend domains instead of scattering `hideInSimpleMode` checks.

**Files:**
- Create: `frontend/src/config/singleUserProduct.ts`
- Create: `frontend/src/config/__tests__/singleUserProduct.spec.ts`
- Modify: `frontend/src/router/singleUserGatewayMode.ts`

**Step 1: Write the failing test**

```ts
import { describe, expect, it } from 'vitest'
import {
  SINGLE_USER_ADMIN_ROUTES,
  SINGLE_USER_SETTINGS_TABS,
  isSingleUserLegacyPath,
} from '../singleUserProduct'

describe('single-user product surface', () => {
  it('contains only private gateway operator routes', () => {
    expect(SINGLE_USER_ADMIN_ROUTES).toEqual([
      '/admin/dashboard',
      '/admin/ops',
      '/admin/accounts',
      '/admin/api-keys',
      '/admin/usage',
      '/admin/settings',
    ])
  })

  it('keeps only operator-oriented settings tabs', () => {
    expect(SINGLE_USER_SETTINGS_TABS).toEqual([
      'general', 'security', 'gateway', 'email', 'backup',
    ])
  })

  it.each([
    '/admin/users',
    '/admin/orders',
    '/purchase',
    '/affiliate',
    '/redeem',
  ])('marks %s as legacy', (path) => {
    expect(isSingleUserLegacyPath(path)).toBe(true)
  })
})
```

**Step 2: Run RED**

```bash
cd /home/opc/src/sub2api/frontend
./node_modules/.bin/vitest run src/config/__tests__/singleUserProduct.spec.ts
```

Expected: fail because `singleUserProduct.ts` does not exist.

**Step 3: Implement the manifest**

```ts
export const SINGLE_USER_ADMIN_ROUTES = [
  '/admin/dashboard',
  '/admin/ops',
  '/admin/accounts',
  '/admin/api-keys',
  '/admin/usage',
  '/admin/settings',
] as const

export const SINGLE_USER_SETTINGS_TABS = [
  'general',
  'security',
  'gateway',
  'email',
  'backup',
] as const

export const SINGLE_USER_LEGACY_PREFIXES = [
  '/admin/users',
  '/admin/groups',
  '/admin/subscriptions',
  '/admin/redeem',
  '/admin/promo-codes',
  '/admin/announcements',
  '/admin/affiliates',
  '/admin/orders',
  '/subscriptions',
  '/purchase',
  '/orders',
  '/payment',
  '/redeem',
  '/affiliate',
  '/available-channels',
] as const

export function isSingleUserLegacyPath(path: string): boolean {
  return SINGLE_USER_LEGACY_PREFIXES.some(
    (prefix) => path === prefix || path.startsWith(`${prefix}/`),
  )
}
```

Update `singleUserGatewayMode.ts` to import/re-export the shared legacy list rather than defining a duplicate list.

**Step 4: Run GREEN**

```bash
./node_modules/.bin/vitest run \
  src/config/__tests__/singleUserProduct.spec.ts \
  src/router/__tests__/singleUserGatewayMode.spec.ts \
  src/router/__tests__/guards.spec.ts
```

Expected: all pass.

**Step 5: Commit**

```bash
git add frontend/src/config/singleUserProduct.ts \
  frontend/src/config/__tests__/singleUserProduct.spec.ts \
  frontend/src/router/singleUserGatewayMode.ts
git commit -m "refactor: centralize single-user product surface"
```

---

## Task 0.3: Add route-matrix regression tests

**Objective:** Make allowed and forbidden routes executable acceptance criteria.

**Files:**
- Create: `frontend/src/router/__tests__/singleUserRouteMatrix.spec.ts`
- Create: `backend/internal/server/routes/single_user_route_matrix_test.go`

**Frontend test cases:**

```text
/admin/dashboard       allowed
/admin/ops             allowed
/admin/accounts        allowed
/admin/api-keys        allowed
/admin/usage           allowed
/admin/settings        allowed
/admin/users           redirect /admin/dashboard
/admin/orders          redirect /admin/dashboard
/purchase              redirect /admin/dashboard
```

**Backend test cases in private mode:**

```text
Allowed/not-404 after auth middleware stub:
/api/v1/admin/dashboard/*
/api/v1/admin/accounts/*
/api/v1/admin/usage/*
/api/v1/admin/settings/*
/api/v1/admin/api-keys/*

Must be 404:
/api/v1/admin/users
/api/v1/admin/announcements
/api/v1/admin/redeem-codes
/api/v1/admin/promo-codes
/api/v1/admin/subscriptions
/api/v1/admin/affiliates
/api/v1/admin/payment/*
/api/v1/payment/*
```

Use a router registration helper with stub handlers/middleware; do not require production credentials.

**Verification:**

```bash
cd /home/opc/src/sub2api/frontend
./node_modules/.bin/vitest run src/router/__tests__/singleUserRouteMatrix.spec.ts

cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test ./internal/server/routes -run SingleUserRouteMatrix -v
```

**Commit:**

```bash
git add frontend/src/router/__tests__/singleUserRouteMatrix.spec.ts \
  backend/internal/server/routes/single_user_route_matrix_test.go
git commit -m "test: define single-user route matrix"
```

---

# Phase 1: Simplify Navigation and Route Ownership

## Task 1.1: Create a proper admin API-key route

**Objective:** Remove the conceptual and practical mismatch where the private admin sidebar points to the user route `/keys`.

**Files:**
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/router/__tests__/singleUserRouteMatrix.spec.ts`
- Reuse initially: `frontend/src/views/user/KeysView.vue`

**Step 1: Update the failing expectation**

Assert:

```ts
expect(resolvePrivateAdminRoute('/admin/api-keys')).toBeAllowed()
expect(resolvePrivateAdminRoute('/keys')).toRedirect('/admin/api-keys')
```

Use the project’s existing router-test helpers rather than inventing a new assertion API if one already exists.

**Step 2: Add the route**

```ts
{
  path: '/admin/api-keys',
  name: 'AdminAPIKeys',
  component: () => import('@/views/user/KeysView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: true,
    title: 'API Keys',
    titleKey: 'keys.title',
    descriptionKey: 'keys.description',
  },
},
```

Make `/keys` a compatibility redirect to `/admin/api-keys` when in private single-user mode. Do not expose it publicly.

**Step 3: Update sidebar**

Replace:

```ts
filtered.push({ path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon })
```

with:

```ts
filtered.push({ path: '/admin/api-keys', label: t('nav.apiKeys'), icon: KeyIcon })
```

Update onboarding/data-tour selectors accordingly.

**Step 4: Verify**

```bash
cd /home/opc/src/sub2api/frontend
./node_modules/.bin/vitest run \
  src/router/__tests__/singleUserRouteMatrix.spec.ts \
  src/router/__tests__/guards.spec.ts
./node_modules/.bin/vue-tsc -b
```

Browser acceptance:

- Click “API 密钥” from local admin sidebar.
- URL becomes `/admin/api-keys`.
- Page loads without a login loop.
- Existing key list/create/edit/delete behavior still works.

**Step 5: Commit**

```bash
git add frontend/src/router/index.ts \
  frontend/src/components/layout/AppSidebar.vue \
  frontend/src/router/__tests__/singleUserRouteMatrix.spec.ts
git commit -m "fix: give private admin API keys an admin route"
```

---

## Task 1.2: Replace per-feature legacy routes with one compatibility redirect

**Objective:** Remove dozens of dead SaaS route records while preserving a friendly redirect for old bookmarks.

**Files:**
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/router/singleUserGatewayMode.ts`
- Test: `frontend/src/router/__tests__/singleUserRouteMatrix.spec.ts`
- Delete later, not yet: `frontend/src/views/SingleUserGatewayRedirectView.vue`

**Implementation approach:**

- Keep the router guard that checks `isSingleUserLegacyPath(to.path)`.
- Redirect legacy paths to `/admin/dashboard` for admin or `/dashboard` for non-admin.
- Remove explicit route records whose only component is `SingleUserGatewayRedirectView.vue`.
- Let the guard handle legacy URL compatibility before the catch-all route.

**Route records to remove:**

```text
/redeem
/affiliate
/available-channels
/subscriptions
/purchase
/orders
/payment/qrcode
/payment/result
/payment/stripe
/payment/airwallex
/payment/stripe-popup
/admin/users
/admin/groups
/admin/subscriptions
/admin/announcements
/admin/redeem
/admin/promo-codes
/admin/affiliates/*
/admin/orders/*
```

**Verification:**

```bash
./node_modules/.bin/vitest run \
  src/config/__tests__/singleUserProduct.spec.ts \
  src/router/__tests__/singleUserGatewayMode.spec.ts \
  src/router/__tests__/singleUserRouteMatrix.spec.ts \
  src/router/__tests__/guards.spec.ts
```

Browser-check at least:

```text
/admin/users       -> /admin/dashboard
/admin/orders      -> /admin/dashboard
/purchase          -> /admin/dashboard
```

**Commit:**

```bash
git add frontend/src/router/index.ts \
  frontend/src/router/singleUserGatewayMode.ts \
  frontend/src/router/__tests__
git commit -m "refactor: collapse legacy SaaS route redirects"
```

---

## Task 1.3: Replace the sidebar’s hidden SaaS menu tree with a direct single-user menu

**Objective:** Stop constructing and filtering a large SaaS menu only to hide most of it.

**Files:**
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Create: `frontend/src/components/layout/__tests__/AppSidebar.singleUser.spec.ts`

**Step 1: Write behavior test**

Mount sidebar in private admin mode and assert visible route labels/targets are exactly:

```text
/admin/dashboard
/admin/ops
/admin/accounts
/admin/api-keys
/admin/usage
/admin/settings
```

Assert absent:

```text
/admin/users
/admin/groups
/admin/subscriptions
/admin/announcements
/admin/redeem
/admin/promo-codes
/admin/affiliates
/admin/orders
```

**Step 2: Implement direct menu builder**

```ts
function buildSingleUserAdminNavItems(): NavItem[] {
  return [
    { path: '/admin/dashboard', label: t('nav.dashboard'), icon: DashboardIcon },
    { path: '/admin/ops', label: t('nav.ops'), icon: ChartIcon, featureFlag: flagOpsMonitoring },
    { path: '/admin/accounts', label: t('nav.accounts'), icon: GlobeIcon },
    { path: '/admin/api-keys', label: t('nav.apiKeys'), icon: KeyIcon },
    { path: '/admin/usage', label: t('nav.usage'), icon: ChartIcon },
    { path: '/admin/settings', label: t('nav.settings'), icon: CogIcon },
  ]
}
```

Return this list immediately in private mode instead of building the SaaS list and filtering it.

Afterward remove imports, icons, flags, and branches used only by the deleted private-mode menu.

**Step 3: Verify and commit**

```bash
./node_modules/.bin/vitest run src/components/layout/__tests__/AppSidebar.singleUser.spec.ts
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build
```

```bash
git add frontend/src/components/layout/AppSidebar.vue \
  frontend/src/components/layout/__tests__/AppSidebar.singleUser.spec.ts
git commit -m "refactor: simplify private admin navigation"
```

---

# Phase 2: Remove Hidden Background Work and Shared SaaS Widgets

## Task 2.1: Eliminate duplicate sidebar settings fetches

**Objective:** Remove the current duplicate `adminSettingsStore.fetch()` calls from both an immediate watcher and `onMounted`.

**Files:**
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/components/layout/__tests__/AppSidebar.singleUser.spec.ts`

**Test:** Mount as admin and assert `adminSettingsStore.fetch` runs once.

**Implementation:** Keep the immediate watcher or `onMounted`, not both. Prefer the immediate watcher because it also handles role changes.

**Expected improvement:** One fewer startup request/state transition per sidebar mount.

**Commit:**

```bash
git commit -m "perf: deduplicate sidebar settings fetch"
```

---

## Task 2.2: Remove SaaS header widgets and their request paths in private mode

**Objective:** Ensure announcements, subscriptions, balances, and customer docs do not load or issue requests.

**Files:**
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/components/layout/AppLayout.vue` if related initialization exists
- Test: `frontend/src/components/layout/__tests__/AppHeader.singleUser.spec.ts`

**Assertions:**

- `AnnouncementBell` is not mounted.
- `SubscriptionProgressMini` is not mounted.
- Balance/customer purchase widgets are not mounted.
- No announcement/subscription/payment API method is called.
- User menu and operator identity still render.

Prefer not importing these widgets into the private build path at all. If the project still supports a non-private mode in source, use async imports behind the product-mode branch.

**Runtime verification:** A fresh `/admin/dashboard` load must contain no requests matching:

```text
/announcements
/subscriptions
/payment
/orders
```

**Commit:**

```bash
git commit -m "perf: remove SaaS header work from private mode"
```

---

## Task 2.3: Remove inactive subscription/payment stores from app initialization

**Objective:** Stop inactive Pinia stores and pollers from entering the private runtime graph.

**Files:**
- Modify: `frontend/src/stores/index.ts`
- Modify any private-runtime imports of:
  - `frontend/src/stores/subscriptions.ts`
  - `frontend/src/stores/payment.ts`
- Test: `frontend/src/stores/__tests__/privateRuntimeStores.spec.ts`

**Test:** Import the private app bootstrap and assert payment/subscription APIs are not called and subscription polling is not started.

Do not delete store files in this task. First remove all active references and prove the private build no longer includes them.

**Commit:**

```bash
git commit -m "perf: detach SaaS stores from private runtime"
```

---

# Phase 3: Simplify Settings for One Operator

## Task 3.1: Finalize the settings tab type and order

**Objective:** Remove stale `users` and `payment` types and remove the legal-agreement tab from the operator settings navigation.

**Files:**
- Modify: `frontend/src/views/admin/settings/types.ts`
- Modify: `frontend/src/views/admin/settings/useSettingsTabs.ts`
- Modify: `frontend/src/views/admin/settings/__tests__/useSettingsTabs.spec.ts`

**Target:**

```ts
export type SettingsTab =
  | 'general'
  | 'security'
  | 'gateway'
  | 'email'
  | 'backup'

export const SETTINGS_TABS: SettingsTabMeta[] = [
  { key: 'general', icon: 'home' },
  { key: 'security', icon: 'shield' },
  { key: 'gateway', icon: 'server' },
  { key: 'email', icon: 'mail' },
  { key: 'backup', icon: 'database' },
]
```

Before removing `features`, inventory every feature toggle. Move gateway-relevant toggles into `GatewaySettingsTab` and security-relevant toggles into `SecuritySettingsTab`; delete customer/SaaS toggles.

Before removing `agreement`, preserve only legally required static text outside operator settings if necessary.

**Verification:** Keyboard navigation tests must wrap across five tabs.

**Commit:**

```bash
git commit -m "refactor: reduce settings to operator domains"
```

---

## Task 3.2: Slim security settings to private operator controls

**Objective:** Keep operator access controls while removing customer registration and social-login setup.

**Files:**
- Modify: `frontend/src/views/admin/settings/tabs/SecuritySettingsTab.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Delete after reference audit:
  - `frontend/src/views/admin/settings/panels/RegistrationSettingsPanel.vue`
  - `frontend/src/views/admin/settings/panels/LinuxDoOAuthPanel.vue`
  - `frontend/src/views/admin/settings/panels/EmailOAuthPanel.vue`
  - `frontend/src/views/admin/settings/panels/WeChatConnectPanel.vue`
  - `frontend/src/views/admin/settings/panels/DingTalkConnectPanel.vue`
  - `frontend/src/views/admin/settings/panels/OidcConnectPanel.vue`
- Keep:
  - `SecurityAccessControlsPanel.vue`
  - `SecurityAdminApiKeyPanel.vue`

**Focused test:**

- Admin API-key create/regenerate/delete callbacks still work.
- Access-control fields still bind to parent state.
- Registration/social login controls are absent.

Do not confuse application login OAuth with upstream AI account OAuth. The latter remains in account management.

**Commit:**

```bash
git commit -m "refactor: focus security settings on operator access"
```

---

## Task 3.3: Convert email settings into operator alerts only

**Objective:** Keep SMTP/test email and useful operator alerts; remove customer lifecycle messaging.

**Files:**
- Modify: `frontend/src/views/admin/settings/tabs/EmailSettingsTab.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Keep:
  - `frontend/src/views/admin/settings/email/SmtpSettingsPanel.vue`
  - `frontend/src/views/admin/settings/email/TestEmailPanel.vue`
  - `frontend/src/views/admin/settings/email/AccountQuotaNotificationPanel.vue` if it alerts the operator about upstream account quota
- Delete after reference audit:
  - `BalanceLowNotificationPanel.vue`
  - `SubscriptionExpiryNotificationPanel.vue`
  - customer email template editor paths not used by ops/account alerts

Rename the tab label from generic customer “Email” wording to “Operator Alerts” where appropriate, without changing persisted setting keys in the first pass.

**Focused tests:**

- SMTP save wiring remains.
- Test email wiring remains.
- Upstream account quota alert recipients remain.
- Balance/subscription customer panels are absent.

**Commit:**

```bash
git commit -m "refactor: keep operator email alerts only"
```

---

## Task 3.4: Split settings tabs into lazy chunks

**Objective:** Reduce the initial `/admin/settings` chunk by loading only the active tab.

**Files:**
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Create: `frontend/src/views/admin/settings/SettingsTabHost.vue`
- Test: `frontend/src/views/admin/settings/__tests__/SettingsTabHost.spec.ts`

Use `defineAsyncComponent` or route/query-driven lazy imports for the five retained tabs. Keep form state/API payload ownership in `SettingsView.vue` during the first extraction.

**Acceptance:**

- Opening settings initially loads only shell + active tab.
- Switching tab loads one additional chunk.
- Save behavior remains unchanged.
- Settings chunk is materially smaller than Phase 0 baseline.

**Commit:**

```bash
git commit -m "perf: lazy-load operator settings tabs"
```

---

# Phase 4: Unregister Unneeded Backend Route Families

## Task 4.1: Centralize backend private-mode detection

**Objective:** Remove duplicate environment parsing and provide a testable product-mode helper.

**Files:**
- Create: `backend/internal/config/product_mode.go`
- Create: `backend/internal/config/product_mode_test.go`
- Modify:
  - `backend/internal/server/middleware/public_control_plane_guard.go`
  - `backend/internal/server/routes/admin.go`

**Implementation:**

```go
package config

import (
    "os"
    "strings"
)

func SingleUserPrivateControlPlaneEnabled() bool {
    switch strings.ToLower(strings.TrimSpace(os.Getenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE"))) {
    case "1", "true", "yes", "on":
        return true
    default:
        return false
    }
}
```

Tests must cover true values, false values, whitespace, and mixed case. Use `t.Setenv`.

**Commit:**

```bash
git commit -m "refactor: centralize private product mode detection"
```

---

## Task 4.2: Reduce admin route registration in private mode

**Objective:** Register only operator/gateway admin APIs in private mode.

**Files:**
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/server/routes/single_user_route_matrix_test.go`

**Keep registered in private mode:**

```text
dashboard
accounts
upstream OAuth routes
proxies
gateway/system settings required by retained tabs
data export/import used by accounts
backup
ops
version only from system routes
usage
error passthrough
TLS fingerprint profiles
admin API keys
scheduled tests
channels/channel monitors if retained
risk control only if feature remains enabled
```

**Do not register in private mode:**

```text
user management
customer group management UI APIs
announcements
redeem codes
promo codes
subscriptions
user attributes used only for SaaS
affiliate routes
payment administration
```

Important: keep the minimum group lookup/update APIs required by account and API-key routing. Do not delete the group model or group repository.

Prefer two explicit functions:

```go
func registerPrivateGatewayAdminRoutes(admin *gin.RouterGroup, h *handler.Handlers)
func registerSaaSAdminRoutes(admin *gin.RouterGroup, h *handler.Handlers)
```

Then choose once at registration time. Avoid dozens of inline `if` checks.

**Verification:** Route-matrix tests plus relevant account/API-key tests.

**Commit:**

```bash
git commit -m "refactor: register only gateway admin routes in private mode"
```

---

## Task 4.3: Reduce authenticated user routes to gateway essentials

**Objective:** Keep only APIs still used by the private admin/API-key UI.

**Files:**
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/internal/server/routes/single_user_route_matrix_test.go`

**Keep in private mode:**

```text
/keys/*
/groups/available
/groups/rates
/usage/* if still used by retained views
/channel-monitors/* if retained
minimal /user/profile or session identity endpoint only if required by auth store
TOTP endpoints only if retained as operator security
```

**Remove from private registration:**

```text
/user/aff
/user/aff/transfer
/customer identity binding endpoints not used by operator auth
/customer notify-email endpoints
/channels/available if only customer-facing
/announcements
/redeem
/subscriptions
```

Before removing each route, search frontend imports and browser network logs. A route is not removed merely because it sounds like SaaS; prove it is unused by retained screens.

**Commit:**

```bash
git commit -m "refactor: reduce private user APIs to gateway essentials"
```

---

## Task 4.4: Disable all payment route registration in private mode

**Objective:** Ensure payment APIs and webhooks do not exist in the private gateway process.

**Files:**
- Modify: `backend/internal/server/router.go`
- Modify: `backend/internal/server/routes/single_user_route_matrix_test.go`

**Implementation:**

```go
if !config.SingleUserPrivateControlPlaneEnabled() {
    routes.RegisterPaymentRoutes(
        v1,
        h.Payment,
        h.PaymentWebhook,
        h.Admin.Payment,
        jwtAuth,
        adminAuth,
        settingService,
    )
}
```

**Tests:** Assert all `/api/v1/payment/*` and `/api/v1/admin/payment/*` paths return 404 in private mode.

**Runtime verification:** Public and local payment probes return 404.

**Commit:**

```bash
git commit -m "refactor: omit payment routes from private gateway"
```

---

## Task 4.5: Reduce application-auth routes in private mode

**Objective:** Keep only private operator session bootstrap/recovery and remove customer onboarding endpoints.

**Files:**
- Modify: `backend/internal/server/routes/auth.go`
- Modify/add auth route-matrix tests

**Keep only what the current private admin bootstrap requires:**

- Local/WireGuard admin bootstrap/bypass endpoint.
- Current-session/me endpoint.
- Logout.
- One explicit emergency login path if desired and already configured.
- TOTP verification if operator TOTP is retained.

**Do not register in private mode:**

```text
register
validate promo/invitation code
forgot/reset password email workflow
customer OAuth login providers
WeChat payment OAuth
pending customer OAuth account creation
```

Important: do not remove upstream OpenAI/Gemini/Grok account OAuth routes under `/admin`; these are gateway account credentials, not application user login.

**Acceptance:**

- Private local admin access still works.
- Public auth endpoints remain 404.
- No customer registration route exists locally in private mode.

**Commit:**

```bash
git commit -m "refactor: remove customer auth routes from private mode"
```

---

# Phase 5: Delete Proven-Dead Frontend Code

## Task 5.1: Generate a dead-file candidate report

**Objective:** Delete only files with no retained imports, routes, tests, or dynamic references.

**Files:**
- Create: `.hermes/audits/single-user-dead-files.md`

For each candidate domain, search:

```bash
rg -n "UsersView|GroupsView|PaymentView|SubscriptionsView|AffiliateView|RedeemView|AnnouncementsView" frontend/src
```

Classify each file:

```text
DELETE      no retained references
KEEP        gateway dependency
MIGRATE     contains mixed gateway and SaaS logic
UNKNOWN     dynamic/string reference needs investigation
```

Domains:

- Payment views/components/API/store/types.
- Subscription views/components/API/store/types.
- Affiliate views/components/API/types.
- Redeem/promo views/components/API/types.
- Announcement views/components/API/types.
- Customer user/group management views.
- Removed auth-provider pages/panels.

Commit the audit before deletion.

---

## Task 5.2: Delete one frontend domain per commit

**Objective:** Keep deletions reviewable and bisectable.

Recommended commit order:

1. Announcements.
2. Affiliates.
3. Redeem/promo.
4. Payments/orders.
5. Customer subscriptions.
6. Customer user-management views.
7. Customer auth pages/panels.
8. `SingleUserGatewayRedirectView.vue` after router no longer imports it.

For each domain:

1. Delete only audit-approved files.
2. Search domain symbols again.
3. Run targeted router/layout tests.
4. Run `vue-tsc -b`.
5. Run `vite build`.
6. Inspect bundle for removed chunk names.
7. Commit that domain only.

Example commit:

```bash
git commit -m "refactor: remove inactive payment frontend"
```

Do not delete shared types/utilities used by usage accounting or gateway cost display merely because they contain words such as `payment` or `subscription`.

---

# Phase 6: Delete Proven-Dead Backend Code Without Schema Changes

## Task 6.1: Generate backend handler/service dependency report

**Objective:** Identify code that remains initialized even after routes are unregistered.

**Files:**
- Create: `.hermes/audits/single-user-backend-dead-code.md`

For each domain, trace:

```text
route -> handler -> service -> repository -> Ent schema
```

Classify:

- Route-only dead.
- Handler dead but service shared.
- Service dead but repository shared.
- Fully dead above schema.
- Schema retained for compatibility.

Do not delete Ent-generated files manually.

---

## Task 6.2: Remove dead handler wiring one domain at a time

**Objective:** Reduce startup complexity and binary size after routes are gone.

Likely files to inspect/modify:

```text
backend/internal/handler/handlers.go
backend/internal/wire/*
backend/internal/server/router.go
backend/internal/handler/payment*
backend/internal/handler/*subscription*
backend/internal/handler/*affiliate*
backend/internal/handler/*announcement*
backend/internal/handler/*redeem*
```

Use one domain per commit. After each domain:

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test ./internal/handler ./internal/server ./internal/service ./internal/repository
GOTOOLCHAIN=auto go test -tags embed ./internal/web
```

If local Go cannot satisfy `go.mod`, use the Docker build as the compile gate.

Do not remove services used by API-key group routing, usage cost computation, account scheduling, or gateway compatibility.

---

## Task 6.3: Retain old tables but stop background SaaS jobs

**Objective:** Remove CPU/DB work for disabled domains without destructive migrations.

Search scheduler/job registration for:

```text
subscription expiry
payment reconciliation
order cleanup
affiliate settlement
announcement scheduling
customer balance notifications
```

In private mode, do not register those jobs. Keep:

```text
account health/scheduled tests
gateway usage aggregation
ops retention/alerts
backup schedules if configured
upstream credential/quota monitoring
```

Add scheduler-registration tests proving SaaS jobs are absent in private mode.

---

# Phase 7: Optimize the Remaining High-Value Screens

## Task 7.1: Split the large Accounts page by on-demand tools

**Objective:** Reduce the current large `AccountsView` chunk and initial parse cost.

**Files:**
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Convert heavy modals/tools to async components, especially:
  - create/edit/re-auth account modals
  - import/export dialogs
  - CRS sync modal
  - TLS fingerprint profiles modal
  - error passthrough modal
  - scheduled-test panels
- Add: `frontend/src/views/admin/__tests__/AccountsView.lazyTools.spec.ts`

Use `defineAsyncComponent` so these tools load only when opened.

**Acceptance:**

- Account table/search/filter loads normally.
- Opening each modal still works.
- Initial `AccountsView` chunk decreases materially from baseline.
- No API request is triggered merely by importing a closed modal.

**Commit:**

```bash
git commit -m "perf: lazy-load account management tools"
```

---

## Task 7.2: Simplify account table actions for one operator

**Objective:** Make common actions obvious and move rare maintenance actions behind “More.”

Primary actions:

```text
Add account
Refresh
Search/filter
Edit
Enable/disable scheduling
Re-auth
Test account
```

Secondary actions:

```text
Import/export
CRS sync
TLS fingerprints
error passthrough
bulk maintenance
```

Do not remove functionality in this task; reorganize it. Add stable `data-test` hooks and interaction tests.

---

## Task 7.3: Optimize API-key management for the private admin

**Objective:** Make key creation and safe copying the shortest path.

**Files:**
- Modify: `frontend/src/views/user/KeysView.vue` or rename to `frontend/src/views/admin/APIKeysView.vue` after route consolidation
- Update imports/router accordingly
- Add focused tests under `frontend/src/views/admin/__tests__/`

Desired behavior:

- Create key from one prominent button.
- Default group is explicit and safe.
- Secret is shown only at creation/copy interaction.
- Existing table masks key by default.
- Revoke/disable requires confirmation.
- Quota, concurrency, and IP restrictions are available but not visually dominant.
- No customer billing language.

If renaming the view, do it in its own structural commit before behavior changes.

---

## Task 7.4: Reduce dashboard request fan-out

**Objective:** Make the dashboard load only data needed for the single-user cockpit.

**Files:**
- Inspect/modify:
  - `frontend/src/views/admin/DashboardView.vue`
  - `frontend/src/views/admin/components/SingleUserCockpitPanel.vue`
  - corresponding admin dashboard APIs/handlers
- Add request-count tests or mocked API-call tests

Target dashboard data:

```text
service health
active/schedulable upstream accounts
quota/expiry warnings
recent request/error rate
API-key count
recent usage
```

Remove customer/user/subscription/revenue widgets and calls. If several retained calls query the same tables, consider one read-only cockpit summary endpoint only after measuring current request fan-out.

Do not create a new endpoint merely for aesthetic consolidation; require evidence of redundant queries or latency.

---

## Task 7.5: Add bundle and request budgets

**Objective:** Prevent SaaS code and startup calls from creeping back.

**Files:**
- Create: `.hermes/scripts/check-single-user-bundle.py`
- Create: `.hermes/scripts/check-single-user-surface.sh`
- Add tests or CI invocation if CI exists

Bundle check should fail when forbidden chunk names appear:

```text
UsersView
GroupsView
Payment
Orders
Redeem
Affiliate
Announcements
SubscriptionsView
```

Set initial warning budgets from post-cleanup measurements rather than arbitrary numbers. Suggested starting goals:

- No forbidden SaaS chunks.
- `AccountsView` initial chunk at least 20% smaller than Phase 0 baseline.
- `SettingsView` initial chunk at least 30% smaller than Phase 0 baseline.
- Fresh dashboard load has zero announcement/subscription/payment requests.

**Commit:**

```bash
git commit -m "test: enforce private bundle and request budgets"
```

---

# Phase 8: Runtime and Database Optimization Based on Evidence

## Task 8.1: Measure before backend optimization

**Objective:** Avoid speculative performance changes.

Collect for dashboard, accounts, keys, usage, and one gateway request:

- p50/p95 response latency from existing ops data.
- SQL query count if safely observable.
- Redis calls if instrumented.
- CPU/memory at idle and during a small smoke test.
- Container startup time.

Do not expose pprof publicly. If profiling is needed, bind it to localhost temporarily and remove it after the review, or use existing internal metrics.

---

## Task 8.2: Optimize only measured hot queries

**Objective:** Add indexes or query changes only where `EXPLAIN (ANALYZE, BUFFERS)` demonstrates need.

Likely tables to inspect:

```text
usage_logs
request_error_logs
api_keys
accounts
scheduled_tests
```

Rules:

- Never add an index based only on column names.
- Capture query, plan before, change, and plan after.
- Use concurrent index creation where PostgreSQL/version/deployment permits.
- Put schema/index work in its own reversible migration plan if needed.

---

## Task 8.3: Verify idle background activity

**Objective:** Confirm cleanup reduces unnecessary polling/jobs.

After deployment, observe ten idle minutes and record:

- Browser background requests.
- App log request volume.
- DB connections/query rate.
- Redis command rate if available.
- Container CPU.

Expected:

- No payment/subscription/announcement polling.
- No customer email/background jobs.
- Only operator/gateway health tasks remain.

---

# Phase 9: Final Verification and Deployment

## Task 9.1: Run the complete source gate

**Frontend:**

```bash
cd /home/opc/src/sub2api/frontend
./node_modules/.bin/vitest run \
  src/config/__tests__/singleUserProduct.spec.ts \
  src/router/__tests__/singleUserGatewayMode.spec.ts \
  src/router/__tests__/singleUserRouteMatrix.spec.ts \
  src/router/__tests__/guards.spec.ts \
  src/components/layout/__tests__/AppSidebar.singleUser.spec.ts \
  src/components/layout/__tests__/AppHeader.singleUser.spec.ts \
  src/views/admin/settings/__tests__/useSettingsTabs.spec.ts
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build
```

Run retained page tests for Accounts, API Keys, Usage, Ops, and Settings.

**Backend:**

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test ./internal/config ./internal/server/middleware ./internal/server/routes
GOTOOLCHAIN=auto go test ./internal/handler ./internal/service ./internal/repository
GOTOOLCHAIN=auto go test -tags embed ./internal/web
```

**Project checks:**

```bash
cd /home/opc/src/sub2api
.hermes/scripts/check-exapi-brand.sh
python3 .hermes/scripts/audit-large-files.py | head -30
python3 .hermes/scripts/check-single-user-bundle.py
.hermes/scripts/check-single-user-surface.sh
```

---

## Task 9.2: Build production image

```bash
cd /home/opc/src/sub2api
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
sudo docker build \
  --build-arg COMMIT="$COMMIT" \
  --build-arg DATE="$DATE" \
  -t sub2api:single-user-private-control \
  -t "sub2api:single-user-private-control-$COMMIT" \
  .
```

Expected: complete frontend and Go build succeeds.

---

## Task 9.3: Deploy reversibly

Before deployment:

- Record old image ID/tag.
- Verify PostgreSQL backup exists.
- Do not run destructive migrations.

Deploy:

```bash
sudo docker compose -f /opt/sub2api/docker-compose.local.yml up -d sub2api
```

Wait for healthy status. If health or smoke tests fail, redeploy the previous image tag.

---

## Task 9.4: Run live acceptance matrix

### Public domain

Expected:

```text
/                                       404
/login                                  404
/admin/dashboard                        404
/api/v1/admin/accounts                  404
/api/v1/payment/config                  404
/v1/models without key                  401
/v1/models with valid key               200
```

### Private control plane

Expected:

```text
/admin/dashboard                        200
/admin/ops                              200
/admin/accounts                         200
/admin/api-keys                         200
/admin/usage                            200
/admin/settings                         200
```

Legacy private URLs should redirect to `/admin/dashboard`:

```text
/admin/users
/admin/orders
/admin/affiliates/invites
/purchase
/redeem
```

Removed backend APIs should return 404 locally and publicly:

```text
/api/v1/admin/users
/api/v1/admin/subscriptions
/api/v1/admin/announcements
/api/v1/admin/redeem-codes
/api/v1/admin/promo-codes
/api/v1/admin/affiliates
/api/v1/payment/*
/api/v1/admin/payment/*
/api/v1/admin/system/update
/api/v1/admin/system/rollback
/api/v1/admin/system/restart
```

### Gateway smoke test

Using a valid key without printing it:

- `GET /v1/models` returns 200.
- One `codex-auto-review` chat completion returns 200.
- Usage log increments.
- No raw key/token appears in logs.

---

## Task 9.5: Compare before and after

Update `.hermes/audits/single-user-cleanup-baseline.md` with a final comparison:

| Metric | Before | After | Change |
|---|---:|---:|---:|
| Total frontend JS | | | |
| Accounts initial chunk | | | |
| Settings initial chunk | | | |
| Fresh dashboard API calls | | | |
| Forbidden SaaS chunks | | | |
| Idle browser requests/10m | | | |
| Container idle CPU | | | |
| Container memory | | | |
| Private registered route count | | | |

Do not claim an optimization unless the measurement improved or the product surface became meaningfully simpler.

---

# 4. Recommended Commit Sequence

Use this order so every commit is independently reviewable:

```text
docs: capture single-user cleanup baseline
refactor: centralize single-user product surface
test: define single-user route matrix
fix: give private admin API keys an admin route
refactor: collapse legacy SaaS route redirects
refactor: simplify private admin navigation
perf: deduplicate sidebar settings fetch
perf: remove SaaS header work from private mode
perf: detach SaaS stores from private runtime
refactor: reduce settings to operator domains
refactor: focus security settings on operator access
refactor: keep operator email alerts only
perf: lazy-load operator settings tabs
refactor: centralize private product mode detection
refactor: register only gateway admin routes in private mode
refactor: reduce private user APIs to gateway essentials
refactor: omit payment routes from private gateway
refactor: remove customer auth routes from private mode
refactor: remove inactive <domain> frontend
refactor: remove inactive <domain> backend wiring
perf: lazy-load account management tools
perf: streamline single-user dashboard requests
test: enforce private bundle and request budgets
```

---

# 5. Decision Points Requiring User Review

Stop after the relevant phase and ask before proceeding if any of these are unclear:

1. **Proxies:** Keep visible only if proxies are actively used.
2. **Channel management/monitoring:** Keep only if needed for gateway observability.
3. **Risk control/content moderation:** Keep only if actively configured.
4. **Emergency login:** Decide whether to retain one password/TOTP fallback in addition to local admin bypass.
5. **Operator email alerts:** Keep SMTP/test email and upstream quota/ops alerts; confirm whether any email notification is still desired.
6. **Custom pages:** Remove unless used for private documentation.
7. **Batch image guide:** Remove unless this gateway actively serves batch-image workflows.
8. **Database tables:** This plan retains them. Any later table deletion requires a separate backup/migration plan.

Default conservative choice: hide/unregister uncertain features first; do not delete data or schema.

---

# 6. Definition of Done

The cleanup is complete when:

- [ ] Sidebar contains only operator gateway workflows.
- [ ] API keys use `/admin/api-keys` and load correctly through private admin access.
- [ ] No active frontend route imports customer payment/subscription/affiliate/redeem/announcement views.
- [ ] No private-page load calls payment/subscription/announcement APIs.
- [ ] Settings contain only General, Security, Gateway, Operator Alerts, and Backup.
- [ ] Application-login customer registration/OAuth flows are absent in private mode.
- [ ] Payment, subscription, affiliate, redeem, promo, announcement, and customer-management backend routes return 404 in private mode.
- [ ] Routing groups remain functional for accounts and API keys.
- [ ] Upstream account OAuth remains functional.
- [ ] Public gateway authentication and model/completion requests still work.
- [ ] In-app update/rollback/restart remains unavailable.
- [ ] No database tables were dropped.
- [ ] Targeted tests, typecheck, Go tests, Docker build, and live route matrix pass.
- [ ] Bundle and request counts are compared against baseline.
- [ ] No raw credentials appear in tests, logs, or review artifacts.
