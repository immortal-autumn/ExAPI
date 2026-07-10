# ExAPI Whole-Project Refactor Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** 对 ExAPI/Sub2API fork 做一次保守、可验证、分阶段的全项目重构：降低超大文件复杂度，拆清 UI 设置页/导航/网关服务职责，保留现有 API、部署路径、数据库、`SUB2API_*` 环境变量与私有控制面安全语义。

**Architecture:** 采用“先立护栏、再拆大文件、最后清理边界”的 strangler/refactor-in-place 策略。每一步只移动或提取代码，不改变外部行为；所有 public routes、Docker/systemd/runtime identifiers、Go module path 与数据库结构默认保持兼容。前端优先拆分 `SettingsView.vue` 与大型管理组件；后端优先拆分 gateway/auth/setting/service 中的纯函数、策略对象与接口边界。

**Tech Stack:** Vue 3 + TypeScript + Pinia + Vue Router + Vite；Go + Gin + Ent + PostgreSQL + Redis；Docker/Compose；Vitest、vue-tsc、Go tests。

---

## 当前上下文 / 约束

### 必须保持不变

- Go module path 继续为 `github.com/Wei-Shaw/sub2api`，除非另开迁移计划。
- 运行时路径继续兼容：`/opt/sub2api`、`/app/sub2api`、`sub2api.service`、Docker user/group `sub2api`。
- DB/cache/env 默认继续兼容：`sub2api` 数据库名、`SUB2API_*` 环境变量、旧 localStorage/cache key。
- 现有 live 安全语义不能倒退：
  - public domain 只暴露 AI gateway endpoints；
  - 控制台仅 localhost/WireGuard 私有访问；
  - local/WireGuard admin bypass 不得暴露到公网。
- 不做一键全局 rename，不碰 live `/opt/sub2api` 部署，除非用户另行要求部署。
- 每个 task 独立提交、独立测试；大 refactor 禁止一次性改 50 个文件。

### 当前审计指标

- `frontend/src/views/admin/SettingsView.vue`: 11,072 行 / 450 KB。
- `frontend/src/components/account/CreateAccountModal.vue`: 6,005 行。
- `frontend/src/views/admin/GroupsView.vue`: 5,172 行。
- `frontend/src/components/account/EditAccountModal.vue`: 4,510 行。
- `backend/internal/service/gemini_messages_compat_service.go`: 3,494 行。
- `backend/internal/config/config.go`: 3,154 行。
- `backend/internal/handler/admin/account_handler.go`: 2,751 行。
- `backend/internal/service/account.go`: 2,674 行。
- `backend/internal/handler/openai_gateway_handler.go`: 2,514 行。
- `backend/internal/service/gateway_scheduling.go`: 2,459 行。
- `backend/internal/service/gateway_service.go`: 1,289 行。
- `backend/internal/handler/auth_handler.go`: 849 行。

Generated Ent files are large but should not be manually refactored.

### Baseline commands

Run before and after each phase:

```bash
cd /home/opc/src/sub2api/frontend
./node_modules/.bin/vitest run \
  src/i18n/__tests__/zh-only.spec.ts \
  src/config/__tests__/brand.spec.ts \
  src/i18n/__tests__/brand-copy.spec.ts \
  src/router/__tests__/title.spec.ts \
  src/stores/__tests__/app.spec.ts \
  src/utils/__tests__/singleUserCockpit.spec.ts \
  src/views/__tests__/KeyUsageView.spec.ts
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build
```

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test ./internal/brand ./internal/setup ./internal/service ./internal/server ./internal/repository ./internal/payment/provider
GOTOOLCHAIN=auto go test -tags embed ./internal/web
GOTOOLCHAIN=auto go test -tags unit ./internal/server/middleware ./internal/server/routes ./internal/handler
```

```bash
cd /home/opc/src/sub2api
.hermes/scripts/check-exapi-brand.sh
git status --short
```

---

## Phase 0: Refactor safety harness

### Task 0.1: Add file-size / complexity audit script

**Objective:** 给后续重构建立可重复的“大文件清单”质量门。

**Files:**
- Create: `.hermes/scripts/audit-large-files.py`
- Create: `.hermes/audits/refactor-baseline.md`

**Step 1: Create script**

```python
#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SKIP_PARTS = {'.git', 'node_modules', 'dist', 'build', '.pnpm-store'}
EXTS = {'.vue', '.ts', '.tsx', '.js', '.go', '.md', '.yaml', '.yml', '.json', '.css'}

def should_skip(path: Path) -> bool:
    return any(part in SKIP_PARTS for part in path.parts)

rows = []
for path in ROOT.rglob('*'):
    if not path.is_file() or should_skip(path) or path.suffix not in EXTS:
        continue
    try:
        lines = sum(1 for _ in path.open('r', encoding='utf-8', errors='ignore'))
    except OSError:
        continue
    rows.append((lines, path.relative_to(ROOT)))

for lines, path in sorted(rows, reverse=True)[:50]:
    print(f'{lines:6d} {path}')
```

**Step 2: Run script**

```bash
cd /home/opc/src/sub2api
python3 .hermes/scripts/audit-large-files.py > .hermes/audits/refactor-baseline.md
```

Expected: top entries include `frontend/src/views/admin/SettingsView.vue` and large account/group components.

**Step 3: Commit**

```bash
git add -f .hermes/scripts/audit-large-files.py .hermes/audits/refactor-baseline.md
git commit -m "chore: add refactor audit baseline"
```

---

### Task 0.2: Add route/security regression checklist

**Objective:** 固化私有控制面不可回归检查。

**Files:**
- Create: `.hermes/audits/refactor-regression-checklist.md`

**Content:**

```markdown
# ExAPI Refactor Regression Checklist

## Public routes must stay hidden
- `/`
- `/home`
- `/login`
- `/admin/dashboard`
- `/api/v1/auth/login`
- `/api/v1/auth/local-admin`
- `/api/v1/admin/dashboard`
- `/api/v1/user/profile`

Expected public status: 404.

## Public AI endpoints stay reachable but protected
- `/v1/models`
- `/v1beta/models`
- `/backend-api/codex/models`
- `/antigravity/models`

Expected public unauthenticated status: 401 JSON.

## Private control plane stays reachable
- `http://127.0.0.1:8027/admin/dashboard`
- `http://100.97.17.1:8027/admin/dashboard`

## Compatibility identifiers intentionally retained
- Go module path: `github.com/Wei-Shaw/sub2api`
- Runtime path: `/opt/sub2api`
- Env vars: `SUB2API_*`
```

**Commit:**

```bash
git add -f .hermes/audits/refactor-regression-checklist.md
git commit -m "docs: document refactor regression checklist"
```

---

## Phase 1: Frontend settings page decomposition

### Design target

`SettingsView.vue` should become a coordinator with:

```text
frontend/src/views/admin/SettingsView.vue
frontend/src/views/admin/settings/
  SettingsTabs.vue
  SettingsSectionCard.vue
  types.ts
  useSettingsTabs.ts
  tabs/
    GeneralSettingsTab.vue
    AgreementSettingsTab.vue
    FeatureSettingsTab.vue
    SecuritySettingsTab.vue
    UserSettingsTab.vue
    GatewaySettingsTab.vue
    PaymentSettingsTab.vue
    EmailSettingsTab.vue
    BackupSettingsTab.vue
```

Keep current route `/admin/settings`, translations, API calls, and form state semantics.

---

### Task 1.1: Extract settings tab metadata and keyboard navigation

**Objective:** 把 tab key、icon、keyboard navigation 从 11k 行主文件中抽出，先不移动业务表单。

**Files:**
- Create: `frontend/src/views/admin/settings/types.ts`
- Create: `frontend/src/views/admin/settings/useSettingsTabs.ts`
- Test: `frontend/src/views/admin/settings/__tests__/useSettingsTabs.spec.ts`
- Modify: `frontend/src/views/admin/SettingsView.vue`

**Step 1: Write test**

```ts
import { describe, expect, it, vi } from 'vitest'
import { SETTINGS_TABS, getNextSettingsTab } from '../useSettingsTabs'

describe('useSettingsTabs', () => {
  it('keeps the expected tab order', () => {
    expect(SETTINGS_TABS.map((tab) => tab.key)).toEqual([
      'general', 'agreement', 'features', 'security', 'users', 'gateway', 'payment', 'email', 'backup'
    ])
  })

  it('wraps keyboard navigation across tabs', () => {
    expect(getNextSettingsTab('general', 'ArrowLeft')).toBe('backup')
    expect(getNextSettingsTab('general', 'ArrowRight')).toBe('agreement')
    expect(getNextSettingsTab('gateway', 'Home')).toBe('general')
    expect(getNextSettingsTab('gateway', 'End')).toBe('backup')
  })
})
```

**Step 2: Run RED**

```bash
cd /home/opc/src/sub2api/frontend
./node_modules/.bin/vitest run src/views/admin/settings/__tests__/useSettingsTabs.spec.ts
```

Expected: fail because module does not exist.

**Step 3: Implement module**

```ts
// frontend/src/views/admin/settings/types.ts
export type SettingsTab =
  | 'general'
  | 'agreement'
  | 'features'
  | 'security'
  | 'users'
  | 'gateway'
  | 'payment'
  | 'email'
  | 'backup'

export interface SettingsTabMeta {
  key: SettingsTab
  icon: string
}
```

```ts
// frontend/src/views/admin/settings/useSettingsTabs.ts
import type { SettingsTab, SettingsTabMeta } from './types'

export const SETTINGS_TABS: SettingsTabMeta[] = [
  { key: 'general', icon: 'home' },
  { key: 'agreement', icon: 'document' },
  { key: 'features', icon: 'bolt' },
  { key: 'security', icon: 'shield' },
  { key: 'users', icon: 'user' },
  { key: 'gateway', icon: 'server' },
  { key: 'payment', icon: 'creditCard' },
  { key: 'email', icon: 'mail' },
  { key: 'backup', icon: 'database' },
]

export function getNextSettingsTab(current: SettingsTab, key: string): SettingsTab | null {
  const index = SETTINGS_TABS.findIndex((tab) => tab.key === current)
  if (index < 0) return null
  if (key === 'Home') return SETTINGS_TABS[0].key
  if (key === 'End') return SETTINGS_TABS[SETTINGS_TABS.length - 1].key
  const direction = key === 'ArrowLeft' || key === 'ArrowUp' ? -1 : key === 'ArrowRight' || key === 'ArrowDown' ? 1 : 0
  if (direction === 0) return null
  return SETTINGS_TABS[(index + direction + SETTINGS_TABS.length) % SETTINGS_TABS.length].key
}
```

**Step 4: Patch `SettingsView.vue`**

- Import `SETTINGS_TABS`, `getNextSettingsTab`, `type SettingsTab`.
- Replace local `settingsTabs` with `SETTINGS_TABS`.
- Replace keyboard action map with `getNextSettingsTab`.

**Step 5: Verify**

```bash
./node_modules/.bin/vitest run src/views/admin/settings/__tests__/useSettingsTabs.spec.ts src/views/admin/__tests__/SettingsView.spec.ts
./node_modules/.bin/vue-tsc -b
```

**Commit:**

```bash
git add frontend/src/views/admin/settings/types.ts frontend/src/views/admin/settings/useSettingsTabs.ts frontend/src/views/admin/settings/__tests__/useSettingsTabs.spec.ts frontend/src/views/admin/SettingsView.vue
git commit -m "refactor: extract settings tab metadata"
```

---

### Task 1.2: Extract reusable settings card shell

**Objective:** 减少 settings tab 中重复 card/header/description markup。

**Files:**
- Create: `frontend/src/views/admin/settings/SettingsSectionCard.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue` only for 1-2 low-risk sections first.

**Component:**

```vue
<template>
  <section class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        <slot name="title" />
      </h2>
      <p v-if="$slots.description" class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        <slot name="description" />
      </p>
    </div>
    <div class="space-y-4 p-6">
      <slot />
    </div>
  </section>
</template>
```

**Verification:**

```bash
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vitest run src/views/admin/__tests__/SettingsView.spec.ts
```

**Commit:**

```bash
git add frontend/src/views/admin/settings/SettingsSectionCard.vue frontend/src/views/admin/SettingsView.vue
git commit -m "refactor: add reusable settings section card"
```

---

### Task 1.3: Extract General settings tab

**Objective:** 把站点名称、logo、文档 URL、主页内容等 general settings 移到子组件。

**Files:**
- Create: `frontend/src/views/admin/settings/tabs/GeneralSettingsTab.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Test: existing `frontend/src/views/admin/__tests__/SettingsView.spec.ts`

**Approach:**

- Keep `form` state in parent initially.
- Child receives `form` via `defineModel` or explicit prop + emits.
- Do not change API payload shape.

**Skeleton:**

```vue
<script setup lang="ts">
import SettingsSectionCard from '../SettingsSectionCard.vue'
import type { SettingsForm } from '../types'

const form = defineModel<SettingsForm>('form', { required: true })
</script>

<template>
  <div class="space-y-6">
    <SettingsSectionCard>
      <template #title>{{ $t('admin.settings.general.siteIdentity') }}</template>
      <template #description>{{ $t('admin.settings.general.siteIdentityDesc') }}</template>
      <!-- move exact existing fields here, unchanged -->
    </SettingsSectionCard>
  </div>
</template>
```

If `SettingsForm` type is too large, first extract only the exact subset needed:

```ts
export interface GeneralSettingsFormSlice {
  site_name: string
  site_logo: string
  site_subtitle: string
  doc_url: string
  home_content: string
}
```

**Verification:**

```bash
./node_modules/.bin/vitest run src/views/admin/__tests__/SettingsView.spec.ts
./node_modules/.bin/vue-tsc -b
```

**Commit:**

```bash
git add frontend/src/views/admin/SettingsView.vue frontend/src/views/admin/settings/tabs/GeneralSettingsTab.vue frontend/src/views/admin/settings/types.ts
git commit -m "refactor: extract general settings tab"
```

---

### Task 1.4: Extract Agreement and Feature settings tabs

**Objective:** 继续拆低风险 tab，避免一次性拆 security/gateway。

**Files:**
- Create: `frontend/src/views/admin/settings/tabs/AgreementSettingsTab.vue`
- Create: `frontend/src/views/admin/settings/tabs/FeatureSettingsTab.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`

**Verification:**

```bash
./node_modules/.bin/vitest run src/views/admin/__tests__/SettingsView.spec.ts src/i18n/__tests__/zh-only.spec.ts
./node_modules/.bin/vue-tsc -b
```

**Commit:**

```bash
git add frontend/src/views/admin/SettingsView.vue frontend/src/views/admin/settings/tabs/AgreementSettingsTab.vue frontend/src/views/admin/settings/tabs/FeatureSettingsTab.vue
git commit -m "refactor: extract agreement and feature settings tabs"
```

---

### Task 1.5: Split Security tab into sub-panels without changing tab key

**Objective:** `security` tab 当前包含 Admin API Key + 注册/验证 + CAPTCHA + OAuth/OIDC 等，先拆成 panels，tab key 不变。

**Files:**
- Create: `frontend/src/views/admin/settings/security/AdminApiKeyPanel.vue`
- Create: `frontend/src/views/admin/settings/security/RegistrationSecurityPanel.vue`
- Create: `frontend/src/views/admin/settings/security/ThirdPartyAuthPanel.vue`
- Create: `frontend/src/views/admin/settings/tabs/SecuritySettingsTab.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`

**Rule:**

- Do not rename settings API fields.
- Do not change toggle behavior.
- Do not change validation logic.

**Verification:**

```bash
./node_modules/.bin/vitest run src/views/admin/__tests__/SettingsView.spec.ts src/api/__tests__/settings.authSourceDefaults.spec.ts
./node_modules/.bin/vue-tsc -b
```

**Commit:**

```bash
git add frontend/src/views/admin/SettingsView.vue frontend/src/views/admin/settings/security frontend/src/views/admin/settings/tabs/SecuritySettingsTab.vue
git commit -m "refactor: split security settings panels"
```

---

### Task 1.6: Split Gateway tab into runtime/protocol/scheduler panels

**Objective:** `gateway` tab 当前包含 cooldown、rate limit、stream timeout、rectifier、Claude Code、scheduler 等；按职责拆分。

**Files:**
- Create: `frontend/src/views/admin/settings/gateway/GatewayRuntimePanel.vue`
- Create: `frontend/src/views/admin/settings/gateway/GatewayProtocolPanel.vue`
- Create: `frontend/src/views/admin/settings/gateway/GatewaySchedulerPanel.vue`
- Create: `frontend/src/views/admin/settings/tabs/GatewaySettingsTab.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`

**Verification:**

```bash
./node_modules/.bin/vitest run src/views/admin/__tests__/SettingsView.spec.ts
./node_modules/.bin/vue-tsc -b
```

**Commit:**

```bash
git add frontend/src/views/admin/SettingsView.vue frontend/src/views/admin/settings/gateway frontend/src/views/admin/settings/tabs/GatewaySettingsTab.vue
git commit -m "refactor: split gateway settings panels"
```

---

### Task 1.7: Extract remaining settings tabs and enforce file-size target

**Objective:** 将 SettingsView 降到 < 1,000 行，业务 panels 独立维护。

**Files:**
- Create:
  - `frontend/src/views/admin/settings/tabs/UserSettingsTab.vue`
  - `frontend/src/views/admin/settings/tabs/PaymentSettingsTab.vue`
  - `frontend/src/views/admin/settings/tabs/EmailSettingsTab.vue`
  - `frontend/src/views/admin/settings/tabs/BackupSettingsTab.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`

**Verification:**

```bash
python3 .hermes/scripts/audit-large-files.py | head -20
./node_modules/.bin/vitest run src/views/admin/__tests__/SettingsView.spec.ts src/api/__tests__/settings.paymentVisibleMethods.spec.ts src/api/__tests__/settings.wechatConnect.spec.ts
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build
```

Expected:

- `SettingsView.vue` no longer appears as 11k-line file.
- Build chunk may still be large, but source maintainability is improved.

**Commit:**

```bash
git add frontend/src/views/admin/SettingsView.vue frontend/src/views/admin/settings/tabs .hermes/audits/refactor-baseline.md
git commit -m "refactor: decompose settings view tabs"
```

---

## Phase 2: Frontend navigation and admin component cleanup

### Task 2.1: Extract sidebar nav configuration

**Objective:** 将 `AppSidebar.vue` 的 nav item arrays、feature-flag filtering、private-control-mode logic 拆出。

**Files:**
- Create: `frontend/src/components/layout/sidebar/types.ts`
- Create: `frontend/src/components/layout/sidebar/navItems.ts`
- Create: `frontend/src/components/layout/sidebar/useSidebarMode.ts`
- Test: `frontend/src/components/layout/sidebar/__tests__/useSidebarMode.spec.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`

**Test:**

```ts
import { describe, expect, it } from 'vitest'
import { isPrivateControlHost } from '../useSidebarMode'

describe('isPrivateControlHost', () => {
  it.each(['127.0.0.1', 'localhost', '::1', '100.97.17.1'])('treats %s as private', (host) => {
    expect(isPrivateControlHost(host)).toBe(true)
  })

  it('does not treat public hosts as private', () => {
    expect(isPrivateControlHost('sub2api.research.for-immortal.cn')).toBe(false)
  })
})
```

**Verification:**

```bash
./node_modules/.bin/vitest run src/components/layout/sidebar/__tests__/useSidebarMode.spec.ts
./node_modules/.bin/vue-tsc -b
```

**Commit:**

```bash
git add frontend/src/components/layout/AppSidebar.vue frontend/src/components/layout/sidebar
git commit -m "refactor: extract sidebar navigation configuration"
```

---

### Task 2.2: Extract account modal composables

**Objective:** `CreateAccountModal.vue` / `EditAccountModal.vue` 过大，先抽平台字段、quota form、OAuth helpers。

**Files:**
- Create: `frontend/src/components/account/composables/useAccountQuotaForm.ts`
- Create: `frontend/src/components/account/composables/useAccountPlatformFields.ts`
- Create: `frontend/src/components/account/composables/useAccountOAuthHints.ts`
- Test: corresponding `__tests__/*.spec.ts`
- Modify: `CreateAccountModal.vue`, `EditAccountModal.vue`

**Verification:**

```bash
./node_modules/.bin/vitest run src/components/account/**/*.spec.ts
./node_modules/.bin/vue-tsc -b
```

**Commit:**

```bash
git add frontend/src/components/account
git commit -m "refactor: extract account modal composables"
```

---

### Task 2.3: Extract GroupsView table and form modules

**Objective:** `GroupsView.vue` > 5k 行；拆 table、form dialog、rate rules editor。

**Files:**
- Create: `frontend/src/views/admin/groups/GroupsTable.vue`
- Create: `frontend/src/views/admin/groups/GroupFormDialog.vue`
- Create: `frontend/src/views/admin/groups/GroupRateRulesEditor.vue`
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Test: `frontend/src/views/admin/__tests__/GroupsView.columnSettings.spec.ts`

**Verification:**

```bash
./node_modules/.bin/vitest run src/views/admin/__tests__/GroupsView.columnSettings.spec.ts
./node_modules/.bin/vue-tsc -b
```

**Commit:**

```bash
git add frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/groups
git commit -m "refactor: split admin groups view"
```

---

## Phase 3: Backend service and handler boundary cleanup

### Task 3.1: Extract local/private admin bypass policy

**Objective:** 将 `auth_handler.go` 中 local/WireGuard bypass 判断抽为纯 policy，便于测试和复用。

**Files:**
- Create: `backend/internal/handler/auth/local_admin_bypass_policy.go`
- Create: `backend/internal/handler/auth/local_admin_bypass_policy_test.go`
- Modify: `backend/internal/handler/auth_handler.go`
- Keep existing: `backend/internal/handler/auth_handler_local_admin_test.go`

**Policy skeleton:**

```go
package auth

import (
    "net"
    "net/http"
    "strings"
)

type LocalAdminBypassPolicy struct {
    Enabled bool
    TrustedCIDRs []*net.IPNet
}

func (p LocalAdminBypassPolicy) Allows(r *http.Request) bool {
    if !p.Enabled || r == nil {
        return false
    }
    host, _, err := net.SplitHostPort(r.Host)
    if err != nil {
        host = r.Host
    }
    host = strings.Trim(host, "[]")
    if host == "127.0.0.1" || host == "localhost" || host == "::1" {
        return true
    }
    remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        remoteHost = r.RemoteAddr
    }
    ip := net.ParseIP(remoteHost)
    if ip == nil {
        return false
    }
    for _, cidr := range p.TrustedCIDRs {
        if cidr.Contains(ip) {
            return true
        }
    }
    return false
}
```

**Verification:**

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test -tags unit ./internal/handler -run 'TestLocalAdminBypass|TestLocalAdminBypassPolicy' -count=1
```

**Commit:**

```bash
git add backend/internal/handler/auth_handler.go backend/internal/handler/auth backend/internal/handler/auth_handler_local_admin_test.go
git commit -m "refactor: extract local admin bypass policy"
```

---

### Task 3.2: Extract public control-plane guard path policy

**Objective:** 将 middleware 中 public/private host/path 判断变成纯函数，避免 nginx/app guard 语义散落。

**Files:**
- Create: `backend/internal/server/middleware/public_control_plane_policy.go`
- Modify: `backend/internal/server/middleware/public_control_plane_guard.go`
- Test: `backend/internal/server/middleware/public_control_plane_guard_test.go`

**Verification:**

```bash
GOTOOLCHAIN=auto go test -tags unit ./internal/server/middleware -run PublicControlPlaneGuard -count=1
```

**Commit:**

```bash
git add backend/internal/server/middleware/public_control_plane_*.go
git commit -m "refactor: extract public control-plane policy"
```

---

### Task 3.3: Split gateway request debug configuration

**Objective:** `gateway_service.go` 中 env debug flags、debug body file、model routing debug 等应独立。

**Files:**
- Create: `backend/internal/service/gateway_debug.go`
- Create: `backend/internal/service/gateway_debug_test.go`
- Modify: `backend/internal/service/gateway_service.go`

**Rules:**

- Keep env names: `SUB2API_DEBUG_GATEWAY_BODY`, `SUB2API_DEBUG_MODEL_ROUTING`, `SUB2API_DEBUG_CLAUDE_MIMIC`.
- Do not change debug output path semantics.

**Verification:**

```bash
GOTOOLCHAIN=auto go test ./internal/service -run 'Test.*Debug|TestGateway' -count=1
```

**Commit:**

```bash
git add backend/internal/service/gateway_debug.go backend/internal/service/gateway_debug_test.go backend/internal/service/gateway_service.go
git commit -m "refactor: extract gateway debug configuration"
```

---

### Task 3.4: Split account service responsibilities

**Objective:** `backend/internal/service/account.go` 与 admin account handler 过大；优先抽 quota/profile/platform helpers。

**Files:**
- Create: `backend/internal/service/account_quota.go`
- Create: `backend/internal/service/account_platform.go`
- Create: `backend/internal/service/account_profile.go`
- Move tests or add focused tests.
- Modify: `backend/internal/service/account.go`

**Verification:**

```bash
GOTOOLCHAIN=auto go test ./internal/service -run 'Test.*Account|Test.*Quota|Test.*Platform' -count=1
GOTOOLCHAIN=auto go test ./internal/handler/admin -run Account -count=1
```

If `./internal/handler/admin` has no isolated tests, run:

```bash
GOTOOLCHAIN=auto go test ./internal/handler/... -count=1
```

**Commit:**

```bash
git add backend/internal/service/account*.go backend/internal/handler/admin/account_handler.go
git commit -m "refactor: split account service helpers"
```

---

### Task 3.5: Split config into domain files without changing config schema

**Objective:** `backend/internal/config/config.go` > 3k 行；按 config domain 拆文件，同 package 保持 API 不变。

**Files:**
- Create:
  - `backend/internal/config/server.go`
  - `backend/internal/config/database.go`
  - `backend/internal/config/security.go`
  - `backend/internal/config/gateway.go`
  - `backend/internal/config/logging.go`
- Modify: `backend/internal/config/config.go`

**Rule:** same package `config`; move declarations only, no behavior change.

**Verification:**

```bash
GOTOOLCHAIN=auto go test ./internal/config ./internal/setup ./internal/server ./cmd/server -count=1
```

**Commit:**

```bash
git add backend/internal/config
git commit -m "refactor: split backend config package files"
```

---

## Phase 4: Design-system cleanup

### Task 4.1: Create admin page shell components

**Objective:** Normalize admin page headers, tab shells, card shells.

**Files:**
- Create: `frontend/src/components/admin/layout/AdminPageShell.vue`
- Create: `frontend/src/components/admin/layout/AdminSectionCard.vue`
- Modify 1-2 pages only first, e.g. `DashboardView.vue`, `SettingsView.vue`.

**Verification:**

```bash
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vitest run src/router/__tests__/title.spec.ts src/stores/__tests__/app.spec.ts
```

**Commit:**

```bash
git add frontend/src/components/admin/layout frontend/src/views/admin/DashboardView.vue frontend/src/views/admin/SettingsView.vue
git commit -m "refactor: add admin layout primitives"
```

---

### Task 4.2: Enforce Chinese-only UI contract in docs and tests

**Objective:** 当前 UI 已改为中文-only；将该选择记录为产品约束，避免未来误加语言切换。

**Files:**
- Modify: `docs/design/EXAPI_UI_DIRECTION.md`
- Modify: `frontend/src/i18n/__tests__/zh-only.spec.ts`
- Optional remove unused `LocaleSwitcher.vue` only after proving no dynamic imports.

**Check:**

```bash
rg -n '<LocaleSwitcher|import LocaleSwitcher|availableLocales = \[\s*\{ code: .en' frontend/src
```

Expected: no results except possibly dead component export.

**Commit:**

```bash
git add docs/design/EXAPI_UI_DIRECTION.md frontend/src/i18n/__tests__/zh-only.spec.ts
git commit -m "docs: document Chinese-only UI contract"
```

---

## Phase 5: Quality gates and PR hygiene

### Task 5.1: Run full targeted gates

**Commands:**

```bash
cd /home/opc/src/sub2api/frontend
./node_modules/.bin/vitest run \
  src/i18n/__tests__/zh-only.spec.ts \
  src/config/__tests__/brand.spec.ts \
  src/i18n/__tests__/brand-copy.spec.ts \
  src/router/__tests__/title.spec.ts \
  src/stores/__tests__/app.spec.ts \
  src/utils/__tests__/singleUserCockpit.spec.ts \
  src/views/__tests__/KeyUsageView.spec.ts \
  src/views/admin/__tests__/SettingsView.spec.ts \
  src/views/admin/__tests__/GroupsView.columnSettings.spec.ts
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build
```

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test ./internal/brand ./internal/setup ./internal/service ./internal/server ./internal/repository ./internal/payment/provider ./internal/config
GOTOOLCHAIN=auto go test -tags embed ./internal/web
GOTOOLCHAIN=auto go test -tags unit ./internal/server/middleware ./internal/server/routes ./internal/handler
```

```bash
cd /home/opc/src/sub2api
.hermes/scripts/check-exapi-brand.sh
python3 .hermes/scripts/audit-large-files.py | head -30
sudo docker build -t exapi:refactor-smoke .
```

### Task 5.2: Push and update PR

```bash
git push
gh pr edit 1 --repo immortal-autumn/sub2api \
  --title "refactor: modularize ExAPI UI and service boundaries" \
  --body-file /tmp/exapi-refactor-pr-body.md
```

PR body should include:

- Summary by phase.
- Explicit compatibility retained.
- Test matrix output.
- Known deferred items.

---

## Risks / tradeoffs

1. **SettingsView extraction risk:** large file with many reactive dependencies; mitigate by moving one tab/panel per commit and keeping form state in parent initially.
2. **Backend service split risk:** moving methods can accidentally change package visibility or imports; mitigate by same-package extraction first.
3. **Generated Ent files:** do not edit; exclude from file-size target.
4. **Compatibility risk:** do not rename `sub2api` runtime identifiers in this refactor.
5. **Live deployment risk:** do not deploy until PR build passes and user explicitly asks.
6. **UI Chinese-only vs email/payment locale:** product UI is Chinese-only, but some payment/email/provider flows may still carry Accept-Language or template locale data. Do not remove backend locale fields without a separate audit.

---

## Open questions

1. 是否把 PR title 从当前 ExAPI rebrand 改成 broader refactor，还是新建单独分支/PR？建议新分支：`refactor/exapi-modularize`，避免 PR #1 过大。
2. 是否删除 dead `LocaleSwitcher.vue` 与英文 locale files？建议第二阶段再做，先确认邮件模板、法律文档、支付 SDK 是否仍依赖英文内容。
3. 是否保留 SaaS/multi-user功能？当前计划保留，只在 simple/private mode 中隐藏或弱化。
4. 是否要部署到 live `/opt/sub2api`？建议不在 refactor PR 自动部署。

---

## Recommended execution order

1. Phase 0: safety harness.
2. Phase 1.1–1.4: low-risk settings extraction.
3. Run tests and push intermediate branch.
4. Phase 1.5–1.7: security/gateway/heavy tabs.
5. Phase 2: sidebar/account/group cleanup.
6. Phase 3: backend same-package splits.
7. Phase 4: design-system primitives.
8. Phase 5: full gates + Docker smoke.

If time is limited, execute only Phase 0 + Phase 1 first. That yields the largest maintainability gain with the lowest backend risk.
