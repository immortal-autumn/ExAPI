# Private operator router

This directory defines the browser route contract for ExAPI. The product has
one mode: an administrator-only control plane reached through the private
control listener. English is the default UI language; every operator-facing
route and retirement message has English and Simplified Chinese locale keys.

本目录定义 ExAPI 的浏览器路由契约。产品只有一种模式：通过私有控制监听器
访问的管理员专用控制面。界面默认使用英文；所有管理员路由和退役提示都提供
英文及简体中文文案。

## Route contract

### Compatibility redirects

| Path | Destination | Purpose |
| --- | --- | --- |
| `/`, `/home`, `/admin` | `/admin/dashboard` | Stable entry points |
| `/keys` | `/admin/api-keys` | Legacy API-key bookmark |
| `/admin/channels` | `/admin/channels/pricing` | Legacy channel bookmark |

### Operator routes

All routes below require the private control-plane boundary and administrator
identity. The backend remains authoritative; a browser route is not an access
control mechanism.

| Path | View | Purpose |
| --- | --- | --- |
| `/admin/dashboard` | `admin/DashboardView.vue` | Cockpit overview |
| `/admin/api-keys` | `admin/AdminAPIKeysView.vue` | Operator API-key management |
| `/batch-image` | `user/BatchImageGuideView.vue` | Operator batch-image jobs |
| `/admin/accounts` | `admin/AccountsView.vue` | Provider accounts and OAuth imports |
| `/admin/groups` | `admin/GroupsView.vue` | Routing groups and pricing |
| `/admin/proxies` | `admin/ProxiesView.vue` | Proxy lifecycle and diagnostics |
| `/admin/channels/pricing` | `admin/ChannelsView.vue` | Channel pricing and mappings |
| `/admin/channels/monitor` | `admin/ChannelMonitorView.vue` | Channel health monitoring |
| `/admin/ops` | `admin/ops/OpsDashboard.vue` | Runtime operations and alerts |
| `/admin/usage` | `admin/UsageView.vue` | Gateway usage records |
| `/admin/audit-logs` | `admin/AuditLogView.vue` | Immutable operator audit view |
| `/admin/risk-control` | `admin/RiskControlView.vue` | Content/risk controls |
| `/admin/prompt-audit` | `features/prompt-audit/PromptAuditView.vue` | Prompt input audit |
| `/admin/settings` | `admin/PrivateSettingsView.vue` | Private gateway settings |

### Retired customer routes

Former customer entry points such as `/register`, `/dashboard`, `/profile`,
`/payment/*`, `/subscriptions/*`, `/redeem/*`, and `/affiliate/*` are retained
only as an explicit retirement page for bookmarked browser URLs. They do not
load customer components or customer API clients. The backend returns
`410 Gone` with code `CUSTOMER_SURFACE_RETIRED` for retired customer API roots.

旧客户入口（如 `/register`、`/dashboard`、`/profile`、`/payment/*`、
`/subscriptions/*`、`/redeem/*` 和 `/affiliate/*`）仅为书签兼容保留退役页面。
它们不会加载普通用户组件或客户 API 客户端。后端对已退役的客户 API 根路径
返回带有 `CUSTOMER_SURFACE_RETIRED` 代码的 `410 Gone`。

Unknown paths use `NotFoundView.vue`. A retired route and an unknown route are
intentionally different: retirement communicates a deliberate product removal,
while 404 communicates that no route exists.

## Navigation lifecycle

`createAppRouter` in `index.ts` performs the following lifecycle:

1. Establishes the peer-authenticated operator identity with `authStore.checkAuth()`.
2. Resolves the localized document title.
3. Loads risk-control settings before risk-control routes and redirects only on an
   explicit, successfully loaded disable flag.
4. Starts and ends navigation-loading state and prefetches the next route.
5. Retries one dynamic chunk load after a deployment update, then reports a
   persistent cache error without an infinite reload loop.

`requiresAuth` and `requiresAdmin` metadata documents the control-plane contract
for shared components. The private listener, operator middleware, and backend
handlers enforce the actual boundary.

`index.ts` 中的 `createAppRouter` 生命周期如下：

1. 通过 `authStore.checkAuth()` 建立对等节点认证的管理员身份。
2. 解析本地化页面标题。
3. 在进入风控路由前加载风控设置，只有成功加载且明确关闭时才重定向。
4. 管理导航加载状态，并预取下一路由。
5. 部署更新导致动态 chunk 加载失败时最多重试一次，避免无限刷新。

`requiresAuth` 与 `requiresAdmin` 元数据用于记录共享组件契约；真正的边界由
私有监听器、管理员中间件和后端 handler 强制执行。

## Lazy loading and security

All operator views use dynamic imports. Keep customer/payment/auth modules out of
the private initial graph; `frontend/check-private-bundle.mjs` rejects forbidden
chunks and API strings during the production build. Never treat route hiding as
authorization, and never add secrets, tokens, environment files, or database
exports to the frontend bundle.

所有管理员视图都使用动态导入。应将客户、支付、认证模块排除在私有初始图之外；
生产构建期间 `frontend/check-private-bundle.mjs` 会拒绝禁用 chunk 和 API 字符串。
路由隐藏不能替代授权，前端 bundle 也不得包含密钥、token、环境文件或数据库导出。

## Testing

Run from `frontend/`:

```bash
pnpm vitest run src/router/__tests__/singleUserRouteMatrix.spec.ts \
  src/router/__tests__/singleUserGatewayRoutes.spec.ts \
  src/components/layout/__tests__/AppSidebar.spec.ts
pnpm vue-tsc --noEmit
```

The route matrix must contain only the private operator contract, compatibility
redirects, explicit retirement routes, and the catch-all 404 route. When a route
changes, update `privateRoutes.ts`, `singleUserProduct.ts`, the relevant locale
keys, and this document in the same reviewed change.

从 `frontend/` 目录运行：

```bash
pnpm vitest run src/router/__tests__/singleUserRouteMatrix.spec.ts \
  src/router/__tests__/singleUserGatewayRoutes.spec.ts \
  src/components/layout/__tests__/AppSidebar.spec.ts
pnpm vue-tsc --noEmit
```

路由矩阵只能包含私有管理员契约、兼容重定向、明确的退役页面以及 404 兜底路由。
修改路由时，应在同一个评审变更中同步更新 `privateRoutes.ts`、
`singleUserProduct.ts`、相关本地化文案和本文档。
