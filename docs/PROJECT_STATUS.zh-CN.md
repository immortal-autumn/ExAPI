# ExAPI 项目状态

这是 ExAPI fork 的标准双语状态记录。发布、生产部署、回滚或运维不变量变化时，
请同步更新本文件和英文版 [`PROJECT_STATUS.md`](PROJECT_STATUS.md)。英文文档仍是
默认入口；本文件提供简体中文对应内容。

## 当前发布

最后审阅：**2026-09-02（Europe/London）**

| 项目 | 当前值 |
|---|---|
| 产品版本 | `0.2.7` |
| GitHub 仓库 | `immortal-autumn/ExAPI` |
| Git tag | `v0.2.7` |
| 主分支 | `main`（已快进到审阅提交） |
| 发布分支 | `revision/exapi-v0.2.1` |
| 审阅提交 | `a1c8bb6a7a4e49d67fbdb81aadc67de4ef12e7c1` |
| OCI 镜像 | `ghcr.io/immortal-autumn/sub2api2personal@sha256:628dbccd43e5348989ae83c6c7c494bcbe85227824a1acfedee10f77dd7f1795` |
| GitHub Release | <https://github.com/immortal-autumn/ExAPI/releases/tag/v0.2.7> |
| Release workflow | <https://github.com/immortal-autumn/ExAPI/actions/runs/33308484734> |

镜像经过多架构构建、OCI 标签、SPDX SBOM、SLSA/SBOM attestations 和
`gh attestation verify` 验证。生产环境只使用上述 immutable digest，不使用
`latest` 等可变标签。GHCR 包名仍保留 `sub2api2personal` 以兼容现有部署。

## 管理员专用化审查分支

非生产审查分支 `revision/exapi-v0.2.1` 截至 2026-09-01 已继续完成管理员界面加固。
以下可独立回滚的提交均已通过各自的本地重点/完整检查，以及 GitHub CI 和安全扫描：

- `30f5dc180`：日常代理凭据最小暴露；
- `3453be8cf`：代理破坏性操作的界面防护；
- `faca8d3e0`：代理创建/更新请求的进行中防重复提交保护；
- `99cfb8a64`：代理 JSON 导入请求的进行中防重复提交保护。
- `18d0b7390`：代理部分更新契约；省略字段保留现值，显式可空值可清除已存设置。
- `27f6b9697` 与 `4cb556a42`：新鲜 Antigravity 能力诊断、有限探测原因和 provider body
  脱敏；两次提交均已通过 GitHub CI 与安全扫描，但尚未 promotion 到生产。
- `403a6a928`：用户端及运维错误详情只显示有界的脱敏 provider 响应元数据；同时修复
  用户详情弹窗首次打开时未稳定加载所选记录的问题。重点/完整前端测试、类型检查、构建
  和 bundle 门禁均通过。
- `e2ebfda27`：从管理员控制面构建中移除 Driver.js 新手引导运行时、全局引导样式、引导
  store/composable/步骤定义及过期的中英文引导文案；稳定工作流钩子统一为
  `data-testid`。重点/完整前端测试、类型检查、构建、bundle 门禁和变更文件 lint 均通过。
- `bf7e501aa`：将 Grok SSO 导入请求体限制为 25 MiB，并增加处理器测试，确认超限请求会在
  调用 OAuth client 前被拒绝。
- `593e13727`：允许 Antigravity OAuth 导入使用已验证邮箱或默认值自动命名；为管理员 API
  Key 重新生成/删除增加确认弹窗；手工输入的超量 Grok SSO 批次在调用 API 前拦截。
- `25baef30f`：为私有管理员 API Key 路由增加 operator-mode 包装视图，分组变更改走现有
  管理员契约；显式解绑使用文档规定的 `group_id: 0` 哨兵。重点/完整前端检查以及本地
  类型和 lint 门禁均通过；生产环境未修改。
- `e34ef0eb9`：API Key 创建幂等响应现在只保存脱敏内容；首次响应保留一次性 secret，
  持久化重放会省略 secret。实现和处理器重点测试记录在
  [`phase-2-api-key-idempotency.md`](../tmp/usability-review/current/phase-2-api-key-idempotency.md)：
  浏览器尚未自动附加幂等键，因为脱敏重放无法在首次响应丢失时恢复密钥；后续仍需
  明确 API 契约。
- `efca8436b`：历史个人资料 API 前缀 `/api/v1/users` 和公开模型广场前缀
  `/api/v1/model-plaza` 现在稳定返回机器可读的 `410 Gone`；严格段边界测试避免
  误拦截相似路径。生产环境未修改。
- `049ece0f3`：补齐 `/api/v1/admin/groups/:id/subscriptions` 动态订阅退役路径，
  同样返回双语 `410 Gone`，且不影响运维分组 API。生产环境未修改。
- `9dacfcf2f`：离线迁移报告验证限制为 4 MiB，并在读取前后校验文件描述符与路径
  身份、大小和修改时间；替换或变更时 fail-closed。生产环境未修改。
- `bfbc68c94`：账号/代理备份导入现在传播创建后及复用代理的 `UpdateProxy` 失败，
  结构化结果和失败计数不再误报为完整成功。生产环境未修改。
- `2d3d24d5f`：迁移报告路径检查改用 `Lstat`，拒绝符号链接或非普通文件替换。
  生产环境未修改。
- `ea8c669c7`：仅代理导入现在会将代理状态同步失败计入 `proxy_failed`，与结构化
  错误条目保持一致。生产环境未修改。

审查分支还包含代理部分更新契约加固：省略的生命周期和凭据字段会保留现值，显式
null/空值则会清除对应可空设置。这部分尚未纳入上面的生产版本，也没有部署；在按
发布流程完成单独验证并 promotion 前，生产仍保持 v0.2.7 的已审查 immutable digest。

## OPC 生产部署

生产部署位于 `/opt/sub2api`，Compose project 为 `sub2api`，当前输入为：

- `/opt/sub2api/.env.v0.2.6`
- `/opt/sub2api/docker-compose.v0.2.6.yml`

`/opt/sub2api/.env` 和 `docker-compose.local.yml` 保留旧的本地 provenance 及 v0.2.5
回滚 digest；生产 promotion 明确使用上面的版本化文件。

应用容器版本为 `0.2.7`，OCI revision 为上述提交，状态 healthy、重启次数为 0。
PostgreSQL 和 Redis 仅作为既有依赖保留，容器 ID、启动时间、数据挂载和健康状态均
未改变。v0.2.5 的环境、Compose 文件和 digest 仍可用于无 schema 变化的应用回滚。

## 发布前恢复与 canary 证据

隔离的 v0.2.6 synthetic-provider canary 验证了 readiness、OpenAI-compatible provider
gateway smoke、内部网络、禁止公共出站、账号/key 绑定和 disposable private migration。
生产 rollout `exapi-v026-production-20260824a` 创建了加密 logical dump 和物理
PostgreSQL snapshot；两者均在 networkless disposable target 中独立恢复并通过完整性、
解密和基线计数校验。所有证据仅保存在 OPC checkout 的 `tmp/rollouts/` 或受保护的
off-host 对象存储中，仓库不保存密钥、凭据或数据库内容。

## 生产观察

`exapi-v026-production-20260824-observation` 运行 60 分钟、30 秒间隔，共 120 次
readiness probe：

- readiness failures `0`，unexpected 5xx `0`，重启 `0`；
- error rate `0.0`，p95 `1.0 ms`，基线 `4.852 ms`；
- 新增 P0/P1 告警 `0`，生产拓扑和依赖身份验证通过。

当前机器是 allowlisted WireGuard peer `100.97.17.2`。从该 peer 访问 control
`/ready`、`/api/v1/operator/me` 和只读账号列表均返回 200；公网 `/ready` 返回 200，
公网根路径及公网 control route 返回 404。

## 兼容性与回滚

本次 v0.2.6 与 v0.2.5 之间没有新的数据库 migration，生产 schema 仍为 migration
218、`private_schema_version=2`、`batch_image_jobs=0`。因此允许仅替换应用镜像回滚；
不要重建 PostgreSQL/Redis，也不要对生产使用 destructive private-only confirmation
token。通用门禁和命令见 [`../deploy/PRODUCTION_ROLLOUT.md`](../deploy/PRODUCTION_ROLLOUT.md)。

返回英文标准记录：[`PROJECT_STATUS.md`](PROJECT_STATUS.md)。
