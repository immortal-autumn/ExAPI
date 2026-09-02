# ExAPI 管理员专用基线

[English](ADMIN_ONLY_BASELINE.md) | 简体中文（默认文档语言为 English）

这是管理员专用产品审查的 Phase 0 非敏感基线，记录已审阅源码、OPC 黑盒结果以及路由/功能边界。
密钥、服务商凭据、原始响应、数据库行和私有监听地址均有意排除。

## 审查身份

| 项目 | 值 |
|---|---|
| 审查日期 | 2026-09-02（Europe/London） |
| 审查分支 | `review/admin-only-baseline` |
| 审阅源码 | `f43c4c150cb0a779b4e5766989617d98d304bba7` |
| 生产版本 | v0.2.14 |
| 生产源码 revision | `4b0352fa87720425bf4fb5c23aa91e2c0e212c9e` |
| 生产应用镜像 | `ghcr.io/immortal-autumn/sub2api2personal@sha256:e8a6d161a1acb5d454a13526ef2914533d077fd5aefae7a412bc45f58513857d` |
| 生产 Compose provenance | `docker-compose.v0.2.14.yml` / `.env.v0.2.14` |
| 生产状态 | 应用、PostgreSQL、Redis 均 healthy；重启次数均为 0 |

审查源码尚未创建 release tag，也没有发布 OCI 镜像。在为该精确源码 revision 构建并验证 digest
之前，不得将其替换到生产环境。

## OPC 黑盒验收

2026-09-02 通过授权的操作员网络路径执行检查，只保留状态码和有界摘要：

| 表面 | 结果 |
|---|---|
| 公网 `/health` | HTTP 200，`{"status":"ok"}` |
| 公网 `/ready` | HTTP 200，`{"status":"ready"}` |
| 无 Key 访问公网 `/v1/models` | HTTP 401，机器可读的 API Key 缺失错误 |
| 公网控制 UI/API 路径 | HTTP 404 |
| 私有控制 `/health`、`/ready` | HTTP 200 |
| 带 control marker 的私有 `/api/v1/operator/me` | HTTP 200 |
| 带 control marker 的私有 Cockpit 摘要 | HTTP 200 |

检查遵守私有控制请求 marker 和同源 mutation 策略；未带 marker 的控制 API 请求被
`CONTROL_REQUEST_REQUIRED` 拒绝。

## 已启用服务商探测

通过私有管理员 API 获取账号和模型列表，未刷新凭据；随后每个已配置服务商执行一次最小手动推理探测。
探测端点只写入 [`ACCOUNT_PROBES.md`](ACCOUNT_PROBES.md) 描述的有界手动测试快照，不改变调度资格。

| 服务商 | 账号 | 非刷新模型列表 | 最小推理探测 |
|---|---:|---:|---|
| Antigravity | 1 个 active/schedulable | 28 个模型 | HTTP 200 SSE；`gemini-2.5-flash` 收到服务商 HTTP 429 `quota_exhausted`；账号仍 active/schedulable |
| Grok | 2 个 active/schedulable | 每个账号 13 个模型 | HTTP 200 SSE；`grok-4.5` 成功完成（`reason=ok`） |

Antigravity 结果是服务商额度状态，而不是路由或认证失败。应在服务商额度重置后重新探测，才能声称该账号可推理。

## 管理员路由和 handler 清单

### 浏览器导航

[`frontend/src/config/singleUserProduct.ts`](../frontend/src/config/singleUserProduct.ts) 中的浏览器白名单包含 14 个管理员页面：

`/admin/dashboard`、`/admin/ops`、`/admin/accounts`、`/admin/batch-images`、`/admin/groups`、
`/admin/api-keys`、`/admin/channels/pricing`、`/admin/channels/monitor`、`/admin/proxies`、
`/admin/risk-control`、`/admin/usage`、`/admin/settings`、`/admin/audit-logs`、`/admin/prompt-audit`。

没有公网浏览器产品路由。兼容重定向为 `/`、`/home`、`/admin`、`/admin/channels`、`/keys`。
注册、客户 dashboard、支付、订阅、兑换、推广、资料和旧 batch-image 路径进入明确的 retired-feature 页面。

### 后端活动命名空间

私有 listener 注册以下 operator 命名空间：

- `/api/v1/operator/me`、`/api/v1/settings/public`；
- `/api/v1/keys`、`/api/v1/groups`、`/api/v1/usage`、`/api/v1/operator/batch-images`、`/api/v1/channel-monitors`；
- `/api/v1/admin/cockpit-summary`，以及 dashboard、groups、accounts、OpenAI/Gemini/Antigravity/Grok OAuth、proxies、settings、data-management、backups、ops、system、usage、error-passthrough、TLS fingerprint、channels、channel monitors、monitor templates、risk-control、prompt-audit、audit-log 命名空间。

公网 listener 只保留 health/readiness 和 API Key gateway 系列：`/v1`、`/v1beta`、`/responses`、`/chat/completions`、
`/embeddings`、图片/视频、`/backend-api/codex` 和 `/antigravity`。Gateway 请求必须携带 API Key，不能暴露管理员 UI。

### Handler 归属

活动管理员 handler 为 Dashboard、Group、Account、OAuth、OpenAIOAuth、GeminiOAuth、AntigravityOAuth、GrokOAuth、Proxy、
Setting、DataManagement、Backup、Ops、System、Usage、ErrorPassthrough、TLSFingerprintProfile、AdminAPIKey、ScheduledTest、
Channel、ChannelMonitor、ChannelMonitorRequestTemplate、ContentModeration、PromptAudit、AuditLog、Compliance。
User、Announcement、Redeem、Promo、Subscription、UserAttribute、Affiliate、Payment 字段仅作为源码兼容接口保留；私有模式不注册客户路由。

旧客户 API 前缀由后端稳定的 `CUSTOMER_SURFACE_RETIRED` 410 合约覆盖；公网反向代理在请求到达应用前将其隐藏为 404。

## 语言和质量基线

- English 是 UI、安装器、release、无后缀文档的默认语言；简体中文以 `.zh-CN.md` 或 `zh-CN` locale 配对。
- locale 编译、键完整性、生产文案、Cockpit 双语和套餐有效期测试均通过。
- 当前本地质量运行通过 267 个前端测试文件 / 1,436 个测试、完整 Go 测试、前端 typecheck/lint/build/bundle 门禁及全部部署契约测试。

## 证据和后续门禁

本基线的命令级证据存放在 checkout 内的 `tmp/usability-review/current/phase-0-baseline-2026-09-02.md`，该目录被 Git 忽略；本提交摘要不含凭据或原始服务商数据。

下一门禁是等待 review 源码的 GitHub CI 和 Security Scan，然后创建新版本、签名 SBOM/provenance、刷新恢复数据和 synthetic-provider canary，并执行私有 cutover dry-run。在所有门禁通过前，生产继续运行 v0.2.14。
