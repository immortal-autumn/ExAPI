# ExAPI Rebrand and Design Refactor Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Rename the current Sub2API fork to **ExAPI** and refactor the product presentation/UI so the fork feels like a coherent single-user/private AI gateway rather than an upstream-branded SaaS clone.

**Architecture:** Do this as a staged rebrand with tests at each layer: first centralize brand constants, then update frontend UI strings/assets, then backend defaults/runtime strings, then deployment artifacts/docs. Avoid a blind repository-wide replace because `sub2api` appears in safe internal protocol/cache keys, generated Go import paths, database defaults, Docker service names, and compatibility docs. Treat **ExAPI** as the product/display name first; defer Go module path and service/user renames unless explicitly accepted because they create broad churn and deployment migration risk.

**Tech Stack:** Go 1.26 backend, Gin, Ent, Vue 3 + TypeScript + Pinia + Vue Router + vue-i18n, Vite/Vitest/vue-tsc, Docker Compose/nginx deployment.

---

## Current Context / Assumptions

- Repo path: `/home/opc/src/sub2api`.
- Current branch likely: `feat/local-admin-bypass`.
- Existing relevant files observed:
  - Backend module path: `backend/go.mod` currently `module github.com/Wei-Shaw/sub2api`.
  - Frontend package: `frontend/package.json` currently `"name": "sub2api-frontend"`.
  - Default frontend site name appears in:
    - `frontend/src/stores/app.ts`
    - `frontend/src/router/title.ts`
    - `frontend/src/views/auth/RegisterView.vue`
    - `frontend/src/views/auth/EmailVerifyView.vue`
    - `frontend/src/views/public/LegalDocumentView.vue`
    - `frontend/src/views/admin/SettingsView.vue`
    - `frontend/src/components/layout/AuthLayout.vue`
  - Backend runtime/product strings appear in:
    - `backend/cmd/server/main.go`
    - `backend/internal/web/embed_on.go`
    - `backend/internal/web/embed_test.go`
    - `backend/internal/setup/cli.go`
    - `backend/internal/setup/setup.go`
    - `backend/internal/pkg/logger/options.go`
  - Deploy/product strings appear in:
    - `deploy/Dockerfile`
    - `deploy/docker-compose.local.yml`
    - `deploy/config.example.yaml`
    - `deploy/sub2api.service`
    - `deploy/docker-deploy.sh`
    - `deploy/README.md`, `deploy/DOCKER.md`
  - Docs include heavy upstream sponsorship/reference content in `README.md`, `README_CN.md`, and `docs/*.md`.
- Existing public/private rework added a `SingleUserCockpitPanel.vue` and simplified sidebar on private control hosts. Keep and polish that direction.
- Do **not** change live `/opt/sub2api`, nginx, Docker runtime, GitHub PR state, or deployed server during this plan unless a later execution request explicitly asks for deployment.
- Recommended default: **product rebrand to ExAPI** while keeping compatibility identifiers (`sub2api` cache keys, DB names, Go module imports) unless a task explicitly says otherwise.

---

## Naming Policy

Use this policy before editing anything:

| Category | New value | Change now? | Rationale |
|---|---:|---:|---|
| Product/display name | `ExAPI` | Yes | User explicitly requested project name change. |
| Browser title fallback | `ExAPI` | Yes | Visible UI brand. |
| Default configured `site_name` | `ExAPI` | Yes | Fresh installs should show ExAPI. |
| Frontend package name | `exapi-frontend` | Yes | Low-risk source metadata. |
| Docker image label description | `ExAPI - AI API Gateway Platform` | Yes | Product metadata. |
| Binary name `/app/sub2api` | Keep initially | No | Renaming requires deploy/service migration. Add optional later phase. |
| Linux user/group `sub2api` | Keep initially | No | Host migration risk. |
| DB default `sub2api` | Keep initially | No | Existing data compatibility. |
| Redis/cache prefixes `sub2api:` | Keep initially | No | Existing cache/session compatibility. |
| Go module path `github.com/Wei-Shaw/sub2api` | Keep initially | No | Massive generated import churn; change only with repo rename. |
| External compatibility terms like `sub2apipay` | Keep with context | No | Historical compatibility docs/reference names. |
| User-facing sponsor/referral links | Remove or quarantine | Yes | ExAPI fork should not advertise upstream sponsors by default. |

---

## Phase 0: Baseline Audit and Safety

### Task 0.1: Capture exact rename inventory

**Objective:** Produce an implementation checklist of every remaining `Sub2API`/`sub2api` occurrence excluding generated/build/vendor files.

**Files:**
- Create: `.hermes/audits/exapi-rename-inventory.md`

**Step 1: Run read-only inventory commands**

```bash
cd /home/opc/src/sub2api
rg -n --hidden \
  --glob '!frontend/node_modules/**' \
  --glob '!backend/internal/web/dist/**' \
  --glob '!.git/**' \
  --glob '!frontend/tsconfig.tsbuildinfo' \
  'Sub2API|Sub2Api|sub2api|SUB2API' . \
  > /tmp/exapi-rename-rg.txt
```

Expected: command exits `0`; `/tmp/exapi-rename-rg.txt` contains source/docs/deploy occurrences.

**Step 2: Classify occurrences manually**

Create `.hermes/audits/exapi-rename-inventory.md` with sections:

```markdown
# ExAPI Rename Inventory

## Replace now: product/display strings
- `frontend/src/stores/app.ts`: fallback `Sub2API` -> `ExAPI`
- ...

## Keep for compatibility
- `backend/go.mod`: module path `github.com/Wei-Shaw/sub2api`
- `frontend/src/i18n/index.ts`: localStorage key `sub2api_locale`
- ...

## Needs explicit migration approval
- `deploy/sub2api.service`
- Docker service/container/user names
- Database default dbname/user
```

**Step 3: Commit audit artifact**

```bash
git add .hermes/audits/exapi-rename-inventory.md
git commit -m "docs: inventory Sub2API to ExAPI rename scope"
```

---

### Task 0.2: Add a source-only brand regression script

**Objective:** Add a small script that fails when user-facing `Sub2API` strings remain outside the allowlist.

**Files:**
- Create: `scripts/check-exapi-brand.sh`

**Step 1: Write failing script**

```bash
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

rg -n --hidden \
  --glob '!frontend/node_modules/**' \
  --glob '!backend/internal/web/dist/**' \
  --glob '!.git/**' \
  --glob '!frontend/tsconfig.tsbuildinfo' \
  --glob '!scripts/check-exapi-brand.sh' \
  --glob '!README*.md' \
  --glob '!docs/**' \
  'Sub2API|Sub2Api|SUB2API' \
  frontend/src backend/cmd backend/internal deploy \
  | rg -v 'sub2apipay|Wei-Shaw/sub2api|github.com/Wei-Shaw/sub2api|otpauth://totp/Sub2API|referrer", "sub2api"' \
  && {
    echo "Found unapproved user-facing Sub2API branding. Replace with ExAPI or add a justified allowlist entry." >&2
    exit 1
  }

exit 0
```

**Step 2: Run to verify failure before refactor**

```bash
chmod +x scripts/check-exapi-brand.sh
./scripts/check-exapi-brand.sh
```

Expected: FAIL — lists current user-facing branding occurrences.

**Step 3: Commit script only after later tasks pass**

Do not commit this task until Phase 1/2 replacements make the script pass, unless the team accepts a temporarily failing quality gate.

---

## Phase 1: Centralize Frontend Brand Defaults

### Task 1.1: Create frontend brand constants

**Objective:** Replace scattered fallback literals with a single typed brand config.

**Files:**
- Create: `frontend/src/config/brand.ts`
- Test: `frontend/src/config/__tests__/brand.spec.ts`

**Step 1: Write failing test**

Create `frontend/src/config/__tests__/brand.spec.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { BRAND, getDefaultSiteName, getDefaultPaymentProductPrefix } from '../brand'

describe('brand defaults', () => {
  it('uses ExAPI as the product name', () => {
    expect(BRAND.productName).toBe('ExAPI')
    expect(getDefaultSiteName()).toBe('ExAPI')
  })

  it('uses ExAPI for payment product prefixes', () => {
    expect(getDefaultPaymentProductPrefix()).toBe('ExAPI')
  })
})
```

**Step 2: Run test to verify failure**

```bash
cd frontend
./node_modules/.bin/vitest run src/config/__tests__/brand.spec.ts
```

Expected: FAIL — module `../brand` does not exist.

**Step 3: Implement minimal config**

Create `frontend/src/config/brand.ts`:

```ts
export const BRAND = {
  productName: 'ExAPI',
  productTagline: 'Private AI API Gateway',
  githubRepo: 'immortal-autumn/sub2api',
  publicGatewayBasePath: '/v1',
} as const

export function getDefaultSiteName(): string {
  return BRAND.productName
}

export function getDefaultPaymentProductPrefix(): string {
  return BRAND.productName
}
```

**Step 4: Run test to verify pass**

```bash
cd frontend
./node_modules/.bin/vitest run src/config/__tests__/brand.spec.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/config/brand.ts frontend/src/config/__tests__/brand.spec.ts
git commit -m "feat: add ExAPI frontend brand constants"
```

---

### Task 1.2: Use brand constants in app store and router title

**Objective:** Ensure default site name and document titles fall back to ExAPI.

**Files:**
- Modify: `frontend/src/stores/app.ts`
- Modify: `frontend/src/router/title.ts`
- Modify: `frontend/src/router/__tests__/title.spec.ts`
- Test: `frontend/src/stores/__tests__/app.spec.ts`

**Step 1: Update tests first**

In `frontend/src/router/__tests__/title.spec.ts`, change expectations:

```ts
expect(resolveDocumentTitle('Dashboard', '')).toBe('Dashboard - ExAPI')
expect(resolveDocumentTitle(undefined, '   ')).toBe('ExAPI')
```

In `frontend/src/stores/__tests__/app.spec.ts`, add or update a default-name assertion:

```ts
it('defaults site name to ExAPI', () => {
  const store = useAppStore()
  expect(store.siteName).toBe('ExAPI')
})
```

**Step 2: Run tests to verify failure**

```bash
cd frontend
./node_modules/.bin/vitest run src/router/__tests__/title.spec.ts src/stores/__tests__/app.spec.ts
```

Expected: FAIL — current fallback is `Sub2API`.

**Step 3: Replace fallback literals**

In `frontend/src/stores/app.ts`:

```ts
import { getDefaultSiteName } from '@/config/brand'

const siteName = ref<string>(getDefaultSiteName())
// ...
siteName.value = config.site_name || getDefaultSiteName()
```

In `frontend/src/router/title.ts`:

```ts
import { getDefaultSiteName } from '@/config/brand'

const normalizedSiteName = typeof siteName === 'string' && siteName.trim() ? siteName.trim() : getDefaultSiteName()
```

**Step 4: Run tests to verify pass**

```bash
cd frontend
./node_modules/.bin/vitest run src/router/__tests__/title.spec.ts src/stores/__tests__/app.spec.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/stores/app.ts frontend/src/router/title.ts frontend/src/router/__tests__/title.spec.ts frontend/src/stores/__tests__/app.spec.ts
git commit -m "feat: default frontend branding to ExAPI"
```

---

### Task 1.3: Rebrand auth/legal fallback screens

**Objective:** Replace visible unauthenticated fallback branding with ExAPI.

**Files:**
- Modify: `frontend/src/views/auth/RegisterView.vue`
- Modify: `frontend/src/views/auth/EmailVerifyView.vue`
- Modify: `frontend/src/views/public/LegalDocumentView.vue`
- Modify: `frontend/src/components/layout/AuthLayout.vue`
- Tests:
  - `frontend/src/views/auth/__tests__/EmailVerifyView.spec.ts`
  - any existing auth layout tests if present

**Step 1: Update tests**

In `frontend/src/views/auth/__tests__/EmailVerifyView.spec.ts`, replace mocked default site names:

```ts
site_name: 'ExAPI'
```

Update expectation helper fallback:

```ts
return `Account created for ${params?.siteName ?? 'ExAPI'}`
```

**Step 2: Run tests to verify failure**

```bash
cd frontend
./node_modules/.bin/vitest run src/views/auth/__tests__/EmailVerifyView.spec.ts
```

Expected: FAIL where components still emit `Sub2API`.

**Step 3: Replace literals with constants**

Use:

```ts
import { getDefaultSiteName } from '@/config/brand'
```

Then replace:

```ts
const siteName = ref<string>('Sub2API')
settings.site_name || 'Sub2API'
appStore.siteName || 'Sub2API'
```

with:

```ts
const siteName = ref<string>(getDefaultSiteName())
settings.site_name || getDefaultSiteName()
appStore.siteName || getDefaultSiteName()
```

**Step 4: Run tests to verify pass**

```bash
cd frontend
./node_modules/.bin/vitest run src/views/auth/__tests__/EmailVerifyView.spec.ts
./node_modules/.bin/vue-tsc -b
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/auth/RegisterView.vue frontend/src/views/auth/EmailVerifyView.vue frontend/src/views/public/LegalDocumentView.vue frontend/src/components/layout/AuthLayout.vue frontend/src/views/auth/__tests__/EmailVerifyView.spec.ts
git commit -m "feat: rebrand auth and legal screens to ExAPI"
```

---

## Phase 2: Rebrand Frontend UI Copy and Settings

### Task 2.1: Rebrand i18n onboarding/setup copy

**Objective:** Replace visible onboarding/setup product mentions with ExAPI in English and Chinese locale files.

**Files:**
- Modify: `frontend/src/i18n/locales/en/misc.ts`
- Modify: `frontend/src/i18n/locales/zh/misc.ts`
- Modify: `frontend/src/i18n/locales/en/landing.ts`
- Modify: `frontend/src/i18n/locales/zh/landing.ts`
- Modify: `frontend/src/i18n/locales/en/admin/settings.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/settings.ts`

**Step 1: Write/extend test for locale strings**

Create `frontend/src/i18n/__tests__/brand-copy.spec.ts`:

```ts
import { describe, expect, it } from 'vitest'
import enMisc from '../locales/en/misc'
import zhMisc from '../locales/zh/misc'
import enLanding from '../locales/en/landing'
import zhLanding from '../locales/zh/landing'

function stringify(value: unknown): string {
  return JSON.stringify(value)
}

describe('i18n brand copy', () => {
  it('does not expose Sub2API in primary onboarding/setup copy', () => {
    const text = [enMisc, zhMisc, enLanding, zhLanding].map(stringify).join('\n')
    expect(text).not.toContain('Sub2API')
    expect(text).toContain('ExAPI')
  })
})
```

**Step 2: Run test to verify failure**

```bash
cd frontend
./node_modules/.bin/vitest run src/i18n/__tests__/brand-copy.spec.ts
```

Expected: FAIL — current locale copy contains Sub2API.

**Step 3: Replace visible strings**

Examples:

```ts
title: '👋 Welcome to ExAPI'
description: '<div ...>ExAPI is a private AI service gateway...</div>'
```

Chinese examples:

```ts
title: '👋 欢迎使用 ExAPI'
description: '<div ...>ExAPI 是一个私有 AI 服务网关...</div>'
```

Keep compatibility-specific lowercase `sub2api` mentions in technical hints only when they refer to another upstream Sub2API instance. If kept, add them to test allowlist instead of blanket replacing.

**Step 4: Run test/typecheck**

```bash
cd frontend
./node_modules/.bin/vitest run src/i18n/__tests__/brand-copy.spec.ts
./node_modules/.bin/vue-tsc -b
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/i18n frontend/src/i18n/__tests__/brand-copy.spec.ts
git commit -m "feat: rebrand onboarding copy to ExAPI"
```

---

### Task 2.2: Rebrand settings defaults and payment display names

**Objective:** Replace admin settings placeholders/defaults so new installs configure ExAPI.

**Files:**
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/views/admin/__tests__/SettingsView.spec.ts`

**Step 1: Update tests**

In `SettingsView.spec.ts`, replace default `site_name: "Sub2API"` with:

```ts
site_name: 'ExAPI'
```

Add assertion if absent:

```ts
expect(wrapper.text()).toContain('ExAPI')
```

**Step 2: Run test to verify failure**

```bash
cd frontend
./node_modules/.bin/vitest run src/views/admin/__tests__/SettingsView.spec.ts
```

Expected: FAIL — placeholders/defaults still use Sub2API.

**Step 3: Update component**

In `SettingsView.vue`, import:

```ts
import { getDefaultSiteName, getDefaultPaymentProductPrefix } from '@/config/brand'
```

Replace:

```ts
placeholder="Sub2API"
(form.payment_product_name_prefix || "Sub2API")
site_name: "Sub2API"
```

with:

```vue
:placeholder="getDefaultSiteName()"
```

and:

```ts
(form.payment_product_name_prefix || getDefaultPaymentProductPrefix())
site_name: getDefaultSiteName()
```

**Step 4: Run tests/typecheck**

```bash
cd frontend
./node_modules/.bin/vitest run src/views/admin/__tests__/SettingsView.spec.ts
./node_modules/.bin/vue-tsc -b
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/SettingsView.vue frontend/src/views/admin/__tests__/SettingsView.spec.ts
git commit -m "feat: default admin settings to ExAPI"
```

---

### Task 2.3: Rebrand user-facing file export names safely

**Objective:** Rename downloaded export filenames from `sub2api-*` to `exapi-*` where this is purely cosmetic.

**Files:**
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Modify: `frontend/src/views/admin/ProxiesView.vue`
- Modify: `frontend/src/views/user/BatchImageGuideView.vue`
- Modify tests if present for export filenames.

**Step 1: Search exact filename templates**

```bash
cd /home/opc/src/sub2api
rg -n 'sub2api-.*\$\{timestamp\}|sub2api-batch-image|sub2api-ui' frontend/src
```

Expected: list includes export/cache/request-id occurrences.

**Step 2: Replace only download filenames**

Replace:

```ts
const filename = `sub2api-account-${timestamp}.json`
const filename = `sub2api-proxy-${timestamp}.json`
```

with:

```ts
const filename = `exapi-account-${timestamp}.json`
const filename = `exapi-proxy-${timestamp}.json`
```

Do **not** rename cache/request IDs yet:

```ts
sub2api-batch-image-preview-cache
sub2api-ui-...
sub2api-ui-retry-...
```

These are internal browser compatibility keys; changing them can invalidate client-side cache and complicate debugging.

**Step 3: Run targeted checks**

```bash
cd frontend
./node_modules/.bin/vue-tsc -b
```

Expected: PASS.

**Step 4: Commit**

```bash
git add frontend/src/views/admin/AccountsView.vue frontend/src/views/admin/ProxiesView.vue
git commit -m "feat: rebrand exported filenames to ExAPI"
```

---

## Phase 3: Product Design/UI Refactor

### Task 3.1: Define ExAPI visual language in a design note

**Objective:** Establish a concise UI design direction before modifying many views.

**Files:**
- Create: `docs/design/EXAPI_UI_DIRECTION.md`

**Step 1: Write design direction doc**

```markdown
# ExAPI UI Direction

## Product position
ExAPI is a private AI API gateway for a technical operator: fast status, quota awareness, account switching, and local integration are first-class.

## Navigation
Primary private-control nav:
1. Dashboard
2. Accounts
3. API Keys
4. Usage
5. Proxies
6. Ops
7. Settings

SaaS/admin-commerce pages are secondary or hidden in simple/private mode.

## Visual principles
- Dense but calm: cards with clear numbers, not marketing hero panels.
- Operator-first: health/quota/rate-limit indicators above revenue/user growth.
- Local integration: copyable URLs and client snippets visible on dashboard.
- Keep bilingual support; avoid sponsor/ad content in default fork UI.
```

**Step 2: Commit**

```bash
git add docs/design/EXAPI_UI_DIRECTION.md
git commit -m "docs: define ExAPI UI direction"
```

---

### Task 3.2: Extract single-user dashboard copy into i18n

**Objective:** Remove hardcoded English from `SingleUserCockpitPanel.vue` and prepare UI for bilingual operation.

**Files:**
- Modify: `frontend/src/views/admin/components/SingleUserCockpitPanel.vue`
- Modify: `frontend/src/i18n/locales/en/admin/overview.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/overview.ts`
- Test: `frontend/src/views/admin/components/__tests__/SingleUserCockpitPanel.spec.ts`

**Step 1: Write failing component test**

Create `frontend/src/views/admin/components/__tests__/SingleUserCockpitPanel.spec.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import SingleUserCockpitPanel from '../SingleUserCockpitPanel.vue'

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: vi.fn().mockResolvedValue({ items: [] }),
    },
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

describe('SingleUserCockpitPanel', () => {
  it('renders ExAPI local integration controls', async () => {
    const wrapper = mount(SingleUserCockpitPanel, {
      global: {
        stubs: { Icon: true },
      },
    })

    expect(wrapper.text()).toContain('Local integration')
    expect(wrapper.text()).toContain('Public AI base URL')
  })
})
```

**Step 2: Run test**

```bash
cd frontend
./node_modules/.bin/vitest run src/views/admin/components/__tests__/SingleUserCockpitPanel.spec.ts
```

Expected: PASS initially, but it documents current behavior.

**Step 3: Add i18n keys**

In `frontend/src/i18n/locales/en/admin/overview.ts`, add under an appropriate key:

```ts
singleUserCockpit: {
  title: 'ExAPI cockpit',
  subtitle: 'Private control plane for quota monitoring, wakeup readiness, and multi-account switching.',
  refresh: 'Refresh',
  accounts: 'Accounts',
  quotaMonitoring: 'Quota monitoring',
  multiAccountManagement: 'Multi-account management',
  localIntegration: 'Local integration',
  publicGatewayUrl: 'Public AI base URL',
  wireGuardControlPanel: 'WireGuard control panel',
  localControlPanel: 'Local control panel',
}
```

In Chinese:

```ts
singleUserCockpit: {
  title: 'ExAPI 控制台',
  subtitle: '用于配额监控、唤醒就绪和多账号切换的私有控制面。',
  refresh: '刷新',
  accounts: '账号',
  quotaMonitoring: '配额监控',
  multiAccountManagement: '多账号管理',
  localIntegration: '本地集成',
  publicGatewayUrl: '公网 AI 基础 URL',
  wireGuardControlPanel: 'WireGuard 控制面板',
  localControlPanel: '本机控制面板',
}
```

**Step 4: Refactor component to use `t()`**

In `SingleUserCockpitPanel.vue`:

```ts
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
```

Replace hardcoded labels:

```vue
<h2>{{ t('admin.overview.singleUserCockpit.title') }}</h2>
<p>{{ t('admin.overview.singleUserCockpit.subtitle') }}</p>
```

**Step 5: Run checks**

```bash
cd frontend
./node_modules/.bin/vitest run src/views/admin/components/__tests__/SingleUserCockpitPanel.spec.ts
./node_modules/.bin/vue-tsc -b
```

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/views/admin/components/SingleUserCockpitPanel.vue frontend/src/i18n/locales/en/admin/overview.ts frontend/src/i18n/locales/zh/admin/overview.ts frontend/src/views/admin/components/__tests__/SingleUserCockpitPanel.spec.ts
git commit -m "refactor: localize ExAPI cockpit copy"
```

---

### Task 3.3: Reorder private-mode sidebar around ExAPI operator workflows

**Objective:** Make the private-control sidebar match the ExAPI design direction.

**Files:**
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`

**Step 1: Add test for private-host sidebar ordering**

In `AppSidebar.spec.ts`, add a source-level assertion if the existing test style is source-based:

```ts
it('places Accounts and API Keys high in private single-user mode', () => {
  expect(componentSource).toContain("filtered.push({ path: '/keys'")
  expect(componentSource.indexOf("path: '/admin/accounts'")).toBeLessThan(componentSource.indexOf("path: '/admin/proxies'"))
})
```

If the test mounts the component, mock `window.location.hostname = '100.97.17.1'` and assert rendered link order.

**Step 2: Run test to verify current state**

```bash
cd frontend
./node_modules/.bin/vitest run src/components/layout/__tests__/AppSidebar.spec.ts
```

Expected: PASS or FAIL depending current order. If PASS, keep as regression.

**Step 3: Adjust private mode item order**

In `adminNavItems` private/simple branch, target order:

```ts
Dashboard
Accounts
API Keys
Usage
Proxies
Ops
Announcements
Settings
```

Implementation idea: create a dedicated function instead of mutating filtered items:

```ts
function buildPrivateControlNavItems(visible: NavItem[]): NavItem[] {
  const byPath = new Map(visible.map(item => [item.path, item]))
  return [
    byPath.get('/admin/dashboard'),
    byPath.get('/admin/accounts'),
    { path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon },
    byPath.get('/admin/usage'),
    byPath.get('/admin/proxies'),
    byPath.get('/admin/ops'),
    byPath.get('/admin/announcements'),
    { path: '/admin/settings', label: t('nav.settings'), icon: CogIcon },
  ].filter(Boolean) as NavItem[]
}
```

Use this when `singleUserPrivateControlPlane.value` is true.

**Step 4: Run checks**

```bash
cd frontend
./node_modules/.bin/vitest run src/components/layout/__tests__/AppSidebar.spec.ts
./node_modules/.bin/vue-tsc -b
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/layout/AppSidebar.vue frontend/src/components/layout/__tests__/AppSidebar.spec.ts
git commit -m "refactor: align private sidebar with ExAPI workflows"
```

---

## Phase 4: Backend Product Defaults

### Task 4.1: Add backend brand constants

**Objective:** Centralize backend product names used in logs/setup/web title injection.

**Files:**
- Create: `backend/internal/brand/brand.go`
- Create: `backend/internal/brand/brand_test.go`

**Step 1: Write failing test**

```go
package brand

import "testing"

func TestDefaults(t *testing.T) {
	if ProductName != "ExAPI" {
		t.Fatalf("ProductName=%q, want ExAPI", ProductName)
	}
	if DefaultAdminEmail != "admin@exapi.local" {
		t.Fatalf("DefaultAdminEmail=%q", DefaultAdminEmail)
	}
}
```

**Step 2: Run test to verify failure**

```bash
cd backend
GOTOOLCHAIN=auto go test ./internal/brand
```

Expected: FAIL — package missing.

**Step 3: Implement constants**

```go
package brand

const (
	ProductName       = "ExAPI"
	ProductDescription = "ExAPI - AI API Gateway Platform"
	DefaultAdminEmail = "admin@exapi.local"
	DefaultSiteTitle  = "ExAPI - AI API Gateway"
)
```

**Step 4: Run test to verify pass**

```bash
cd backend
GOTOOLCHAIN=auto go test ./internal/brand
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/brand/brand.go backend/internal/brand/brand_test.go
git commit -m "feat: add backend ExAPI brand constants"
```

---

### Task 4.2: Rebrand backend startup/setup visible strings

**Objective:** Replace startup/setup wizard visible strings with ExAPI while keeping binary/service paths stable.

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/setup/cli.go`
- Modify: `backend/internal/setup/setup.go`
- Modify: `backend/internal/setup/setup_test.go`

**Step 1: Update tests**

In `setup_test.go`, update expected default admin email if tested:

```go
if got := cfg.Admin.Email; got != "admin@exapi.local" { ... }
```

Do **not** change DB defaults from `sub2api` in this task.

**Step 2: Run tests to verify failure**

```bash
cd backend
GOTOOLCHAIN=auto go test -tags unit ./internal/setup ./cmd/server
```

Expected: FAIL where old defaults are used.

**Step 3: Update code**

In `backend/cmd/server/main.go`:

```go
log.Printf("%s %s (commit: %s, built: %s)\n", brand.ProductName, Version, Commit, Date)
log.Printf("Complete the setup wizard to configure %s", brand.ProductName)
```

In `backend/internal/setup/cli.go`:

```go
fmt.Println("║        ExAPI Installation Wizard          ║")
```

Keep this line unchanged for binary compatibility unless optional artifact rename is approved:

```go
fmt.Println("  ./sub2api")
```

In `backend/internal/setup/setup.go`:

```go
Email: getEnvOrDefault("ADMIN_EMAIL", brand.DefaultAdminEmail),
```

**Step 4: Run tests**

```bash
cd backend
GOTOOLCHAIN=auto go test -tags unit ./internal/setup ./cmd/server
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/cmd/server/main.go backend/internal/setup/cli.go backend/internal/setup/setup.go backend/internal/setup/setup_test.go
git commit -m "feat: rebrand backend setup to ExAPI"
```

---

### Task 4.3: Rebrand embedded web title tests

**Objective:** Make backend title injection tests use ExAPI as the bundled title fallback.

**Files:**
- Modify: `backend/internal/web/embed_test.go`
- Modify: `backend/internal/web/embed_on.go` only if needed

**Step 1: Update test HTML fixtures**

Replace fixture titles like:

```go
<title>Sub2API - AI API Gateway</title>
<title>Sub2API</title>
```

with:

```go
<title>ExAPI - AI API Gateway</title>
<title>ExAPI</title>
```

Update assertion:

```go
assert.NotContains(t, string(result), "ExAPI")
```

where the test verifies replacement with custom site name.

**Step 2: Run tests**

```bash
cd backend
GOTOOLCHAIN=auto go test -tags unit ./internal/web
```

Expected: PASS. If FAIL, update `embed_on.go` to use brand constants in generated fallback title behavior.

**Step 3: Commit**

```bash
git add backend/internal/web/embed_test.go backend/internal/web/embed_on.go
git commit -m "test: rebrand embedded web title fixtures"
```

---

## Phase 5: Deployment Metadata and Docs

### Task 5.1: Rebrand Docker metadata without changing runtime names

**Objective:** Update human-readable Docker labels and comments to ExAPI while keeping paths/users/service names stable.

**Files:**
- Modify: `deploy/Dockerfile`
- Modify: `deploy/DOCKER.md`
- Modify: `deploy/build_image.sh` only if image tag rename is approved

**Step 1: Update metadata**

In `deploy/Dockerfile`:

```dockerfile
# ExAPI Multi-Stage Dockerfile
LABEL description="ExAPI - AI API Gateway Platform"
LABEL org.opencontainers.image.title="ExAPI"
```

Keep for now:

```dockerfile
-o /app/sub2api
CMD ["/app/sub2api"]
RUN addgroup ... sub2api
```

**Step 2: Run Dockerfile syntax/build smoke**

```bash
cd /home/opc/src/sub2api
sudo docker build -f deploy/Dockerfile -t exapi:metadata-smoke .
```

Expected: build succeeds.

**Step 3: Commit**

```bash
git add deploy/Dockerfile deploy/DOCKER.md
git commit -m "docs: rebrand Docker metadata to ExAPI"
```

---

### Task 5.2: Rebrand deploy examples and config comments

**Objective:** Make fresh-install docs/config examples present ExAPI while retaining compatibility defaults where necessary.

**Files:**
- Modify: `deploy/config.example.yaml`
- Modify: `deploy/README.md`
- Modify: `deploy/docker-deploy.sh`
- Modify: `deploy/sub2api.service`

**Step 1: Edit display text only**

Examples:

```yaml
# ExAPI Configuration File
# Documentation: https://github.com/immortal-autumn/sub2api
logging:
  service_name: "exapi"
```

For systemd:

```ini
Description=ExAPI - AI API Gateway Platform
```

Keep until optional migration:

```ini
User=sub2api
Group=sub2api
WorkingDirectory=/opt/sub2api
ExecStart=/opt/sub2api/sub2api
SyslogIdentifier=sub2api
```

**Step 2: Validate no accidental runtime rename**

```bash
rg -n 'User=|Group=|ExecStart=|WorkingDirectory=|DATABASE_DBNAME|dbname:' deploy/sub2api.service deploy/config.example.yaml
```

Expected: runtime paths/users remain intentional.

**Step 3: Commit**

```bash
git add deploy/config.example.yaml deploy/README.md deploy/docker-deploy.sh deploy/sub2api.service
git commit -m "docs: rebrand deployment docs to ExAPI"
```

---

### Task 5.3: Rewrite README as ExAPI fork documentation

**Objective:** Replace upstream sponsor-heavy README with concise ExAPI-specific docs.

**Files:**
- Modify: `README.md`
- Modify: `README_CN.md`
- Optional create: `docs/UPSTREAM_COMPATIBILITY.md`

**Step 1: Replace top-level README structure**

Use this outline:

```markdown
# ExAPI

ExAPI is a private AI API gateway for routing local tools and agents through managed upstream AI accounts.

## Highlights
- OpenAI-compatible `/v1` gateway
- Claude/Codex/Gemini/Antigravity compatibility routes
- Private WireGuard/localhost control plane
- Account pool, quota monitoring, scheduled account tests
- API key management and usage logs

## Deployment modes
### Private single-user mode
Public domain exposes only AI gateway routes; control plane is localhost/WireGuard only.

### Standard multi-user mode
Optional SaaS-style user/subscription/payment features inherited from upstream.

## Compatibility with upstream Sub2API
This fork is derived from Wei-Shaw/sub2api. Some internal names, database defaults, and compatibility docs still use `sub2api` to avoid migration risk.
```

**Step 2: Move sponsor/referral content out**

Do not keep upstream sponsor tables in the default README. If preservation is desired, move them to:

```text
docs/upstream/SPONSORS_ORIGINAL.md
```

with a header noting it is upstream historical content, not ExAPI endorsement.

**Step 3: Commit**

```bash
git add README.md README_CN.md docs/UPSTREAM_COMPATIBILITY.md
git commit -m "docs: rewrite README for ExAPI fork"
```

---

## Phase 6: Optional Full Artifact Rename — Decision Gate

Do this phase **only if explicitly approved** after Phase 1–5 pass.

### Task 6.1: Decide whether to rename Go module path

**Objective:** Avoid accidental massive generated-code churn.

**Options:**

1. **Recommended:** Keep `module github.com/Wei-Shaw/sub2api` until GitHub repo is renamed and imports are regenerated intentionally.
2. **Full fork rename:** Change to `module github.com/immortal-autumn/exapi` or `github.com/immortal-autumn/sub2api` and update all imports.

**If choosing option 2, exact steps:**

```bash
cd backend
go mod edit -module github.com/immortal-autumn/exapi
gofmt -w $(find . -name '*.go' -not -path './vendor/*')
rg -l 'github.com/Wei-Shaw/sub2api' . | xargs perl -0pi -e 's#github\.com/Wei-Shaw/sub2api#github.com/immortal-autumn/exapi#g'
GOTOOLCHAIN=auto go test -tags unit ./...
```

Expected: may require Ent/Wire regeneration and broad test fixes.

**Commit:**

```bash
git add backend/go.mod backend/go.sum backend/**/*.go
git commit -m "refactor: rename Go module path to ExAPI fork"
```

---

### Task 6.2: Decide whether to rename binary/service/container names

**Objective:** Migrate runtime artifacts from `sub2api` to `exapi` safely.

**Only proceed after backup and deployment migration plan.**

Potential changes:

```text
deploy/sub2api.service -> deploy/exapi.service
/app/sub2api -> /app/exapi
Docker service: sub2api -> exapi
Linux user/group: sub2api -> exapi
Data dir: /opt/sub2api -> /opt/exapi
DB name: sub2api -> exapi
Redis prefix: sub2api: -> exapi:
```

**Risk:** high. Existing deployments, volumes, databases, logs, systemd units, and scripts will break without migration.

**Recommendation:** do not include in first rebrand PR. Create a separate migration PR with rollback instructions.

---

## Phase 7: Final Quality Gate

### Task 7.1: Run frontend checks

**Objective:** Verify ExAPI frontend passes tests, typecheck, and production build.

**Commands:**

```bash
cd /home/opc/src/sub2api/frontend
./node_modules/.bin/vitest run \
  src/config/__tests__/brand.spec.ts \
  src/router/__tests__/title.spec.ts \
  src/i18n/__tests__/brand-copy.spec.ts \
  src/views/auth/__tests__/EmailVerifyView.spec.ts \
  src/views/admin/__tests__/SettingsView.spec.ts \
  src/views/admin/components/__tests__/SingleUserCockpitPanel.spec.ts \
  src/components/layout/__tests__/AppSidebar.spec.ts

./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build
```

Expected:

```text
Test Files passed
vue-tsc exits 0
vite build exits 0
```

**Commit any final fixes:**

```bash
git add frontend
git commit -m "test: stabilize ExAPI frontend rebrand"
```

---

### Task 7.2: Run backend checks

**Objective:** Verify backend product defaults did not break routing/setup/web embed tests.

**Commands:**

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test -tags unit ./internal/brand ./internal/setup ./internal/web ./internal/server/middleware ./internal/server/routes
```

Expected: PASS.

If time allows, run broader suite:

```bash
GOTOOLCHAIN=auto go test -tags unit ./...
```

Expected: PASS or document known pre-existing failures.

**Commit any final fixes:**

```bash
git add backend
git commit -m "test: stabilize ExAPI backend rebrand"
```

---

### Task 7.3: Run brand quality gate

**Objective:** Ensure no unintended user-facing Sub2API branding remains.

**Commands:**

```bash
cd /home/opc/src/sub2api
./scripts/check-exapi-brand.sh
```

Expected: PASS.

Then inspect all remaining occurrences:

```bash
rg -n --hidden \
  --glob '!frontend/node_modules/**' \
  --glob '!backend/internal/web/dist/**' \
  --glob '!.git/**' \
  'Sub2API|Sub2Api|sub2api|SUB2API' .
```

Expected: remaining occurrences are either:

- compatibility allowlisted,
- upstream attribution/history,
- internal module/import paths,
- deployment runtime identifiers intentionally deferred.

**Commit:**

```bash
git add scripts/check-exapi-brand.sh .hermes/audits/exapi-rename-inventory.md
git commit -m "test: add ExAPI branding quality gate"
```

---

### Task 7.4: Docker build smoke

**Objective:** Prove the refactored project builds into a deployable image.

**Command:**

```bash
cd /home/opc/src/sub2api
sudo docker build -t exapi:rebrand-smoke .
```

Expected: image build succeeds.

Optional inspect:

```bash
sudo docker image inspect exapi:rebrand-smoke --format '{{json .Config.Labels}}'
```

Expected: labels include ExAPI description/title where configured.

---

## Files Likely to Change

### Frontend

```text
frontend/package.json
frontend/src/config/brand.ts
frontend/src/config/__tests__/brand.spec.ts
frontend/src/stores/app.ts
frontend/src/stores/__tests__/app.spec.ts
frontend/src/router/title.ts
frontend/src/router/__tests__/title.spec.ts
frontend/src/views/auth/RegisterView.vue
frontend/src/views/auth/EmailVerifyView.vue
frontend/src/views/auth/__tests__/EmailVerifyView.spec.ts
frontend/src/views/public/LegalDocumentView.vue
frontend/src/components/layout/AuthLayout.vue
frontend/src/components/layout/AppSidebar.vue
frontend/src/components/layout/__tests__/AppSidebar.spec.ts
frontend/src/views/admin/SettingsView.vue
frontend/src/views/admin/__tests__/SettingsView.spec.ts
frontend/src/views/admin/components/SingleUserCockpitPanel.vue
frontend/src/views/admin/components/__tests__/SingleUserCockpitPanel.spec.ts
frontend/src/i18n/locales/en/misc.ts
frontend/src/i18n/locales/zh/misc.ts
frontend/src/i18n/locales/en/landing.ts
frontend/src/i18n/locales/zh/landing.ts
frontend/src/i18n/locales/en/admin/settings.ts
frontend/src/i18n/locales/zh/admin/settings.ts
frontend/src/i18n/locales/en/admin/overview.ts
frontend/src/i18n/locales/zh/admin/overview.ts
frontend/src/i18n/__tests__/brand-copy.spec.ts
frontend/src/views/admin/AccountsView.vue
frontend/src/views/admin/ProxiesView.vue
```

### Backend

```text
backend/internal/brand/brand.go
backend/internal/brand/brand_test.go
backend/cmd/server/main.go
backend/internal/setup/cli.go
backend/internal/setup/setup.go
backend/internal/setup/setup_test.go
backend/internal/web/embed_test.go
backend/internal/web/embed_on.go
```

### Deployment/docs/scripts

```text
scripts/check-exapi-brand.sh
.hermes/audits/exapi-rename-inventory.md
docs/design/EXAPI_UI_DIRECTION.md
README.md
README_CN.md
docs/UPSTREAM_COMPATIBILITY.md
deploy/Dockerfile
deploy/DOCKER.md
deploy/config.example.yaml
deploy/README.md
deploy/docker-deploy.sh
deploy/sub2api.service
```

---

## Risks / Tradeoffs

1. **Blind global replace is unsafe.** `sub2api` is present in Go imports, DB defaults, Docker users, cache keys, websocket subprotocols, and upstream compatibility text.
2. **Go module rename is high churn.** It touches generated Ent files and every internal import; defer unless the GitHub repo is renamed and CI accepts the churn.
3. **Runtime artifact rename can break live deployments.** `/opt/sub2api`, `sub2api.service`, Docker service/container names, DB names, and Redis prefixes should be migrated in a separate deploy plan.
4. **Tests may encode upstream branding intentionally.** Update visible UI tests; allowlist compatibility tests like `sub2apipay` or OAuth referrer behavior only with comments.
5. **Docs sponsorship cleanup is semantic, not just rename.** Remove or quarantine upstream referral/sponsor content so ExAPI docs are not misleading.
6. **Existing user settings can override defaults.** If a live database already has `site_name=Sub2API`, code changes alone may not change the deployed UI; execution should include an explicit DB/settings migration only if approved.

---

## Open Questions Before Optional Full Rename

1. Should the GitHub repository eventually become `immortal-autumn/exapi`, or remain `immortal-autumn/sub2api` with ExAPI as product name?
2. Should live deployment paths move from `/opt/sub2api` to `/opt/exapi`, or remain stable?
3. Should the binary/image/service names become `exapi`, or is product/UI branding enough for now?
4. Should `site_name` in the current live database be updated to `ExAPI` during deployment, or only default new installs?
5. Should public domain change from `sub2api.research.for-immortal.cn` to an ExAPI-branded hostname?

---

## Recommended Execution Approach

Implement Phase 0–5 as one PR branch, with small commits after each task. Stop before Phase 6 unless the optional full artifact rename is explicitly approved. After the PR passes tests and Docker build, deploy separately with backups and verify public gateway/private control behavior again.
