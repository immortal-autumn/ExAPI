# ExAPI 项目状态

这是 ExAPI fork 的标准双语状态记录。发布、生产部署、回滚或运维不变量变化时，
请同步更新本文件和英文版 [`PROJECT_STATUS.md`](PROJECT_STATUS.md)。英文文档仍是
默认入口；本文件提供简体中文对应内容。

## 当前发布

最后审阅：**2026-09-02（Europe/London）**

| 项目 | 当前值 |
|---|---|
| 产品版本 | `0.2.14`（2026-09-02 已部署到 OPC） |
| GitHub 仓库 | `immortal-autumn/ExAPI` |
| Git tag | `v0.2.14` |
| 主分支 | `main`（当前为 `f79ef301b`；发布分支保持独立） |
| 发布分支 | `revision/exapi-v0.2.1` |
| 审阅提交 | `4b0352fa87720425bf4fb5c23aa91e2c0e212c9e` |
| OCI 镜像 | `ghcr.io/immortal-autumn/sub2api2personal@sha256:e8a6d161a1acb5d454a13526ef2914533d077fd5aefae7a412bc45f58513857d` |
| GitHub Release | <https://github.com/immortal-autumn/ExAPI/releases/tag/v0.2.14> |
| Release workflow | <https://github.com/immortal-autumn/ExAPI/actions/runs/33625979848> |

镜像经过多架构构建、OCI 标签、SPDX SBOM、SLSA/SBOM attestations 和
`gh attestation verify` 验证。生产环境只使用上述 immutable digest，不使用
`latest` 等可变标签。GHCR 包名仍保留 `sub2api2personal` 以兼容现有部署。

## v0.2.11 发布验证结果

带注释的 `v0.2.11` 标签已从提交 `b2a5889b4` 发布，release/security/CI 工作流均通过。
其 immutable OCI manifest digest 为
`sha256:1a27d4282b714ecf5b50234a5a55f5ea89c3e353b897fa8b5877cdaf71eda673`，镜像标签与版本
`0.2.11` 及该提交一致。镜像已拉取到 OPC 验证，但没有 promotion 到生产。

对该精确 digest 运行的新 synthetic-provider canary 在切换前失败：位置式 identity 映射
漏计新增的 `prompt_audit_events`，导致 `user_group_rate_multipliers` 被按不存在的
`id` 列快照。canary 使用隔离资源并自动清理；生产仍保持 v0.2.10。因此尽管 artifact
门禁通过，v0.2.11 仍不可用于部署。

## v0.2.12 发布验证结果

`v0.2.12` artifact 已构建并完成 attestation，manifest digest 为
`sha256:e0e7e22dd11a38cdd8b97e4f11750dff1613e602a0342907e9b6aa2b95c9f2f3`。
第一次全新的 synthetic-provider canary 已通过此前发现的 user/group 映射，但随后在真实
表结构 `user_affiliates` 上失败：该表使用 `user_id` 作为主键，不存在 `id` 列。隔离
canary 已自动清理，生产仍保持 v0.2.10。因此该 artifact 已被 supersede，不得 promotion。

## v0.2.13 发布候选准备

`backend/cmd/server/VERSION` 现声明 `0.2.13`。候选新增 user-primary-key 表的明确 identity
类型，并在回归测试覆盖 `user_affiliates` 两个引用和 `passkey_user_handles`。必须创建新的
带注释标签和 digest；不得复用 v0.2.11 或 v0.2.12 artifact/canary 证据。只有新 digest
通过完整 release workflow、新鲜 restored-data 与 synthetic-provider canary，并在部署前
立即完成异地 readiness/告警验证后，才允许 promotion。

fail-closed 私有化切换矩阵仍保持覆盖：保留的 API key、usage/billing 与运维归属引用会迁移
或置空；客户专属行会删除，历史快照保留，并在签名报告记录行数及 identity checksum。
缺失可选表/列记录为跳过，不可置空的历史引用会中止事务；旧版 `user_external_identities`
也有明确策略。

## v0.2.14 发布与部署结果

v0.2.13 发布流程在 promotion 前被替换：其 CI 唯一失败原因为新增 Go 测试 map 条目未按
`gofmt` 对齐；该标签的镜像没有部署。格式修复已作为独立提交保存，
`backend/cmd/server/VERSION` 现声明 `0.2.14`。
新的带注释标签从提交 `4b0352fa87720425bf4fb5c23aa91e2c0e212c9e` 发布。多架构 OCI manifest
digest 为
`sha256:e8a6d161a1acb5d454a13526ef2914533d077fd5aefae7a412bc45f58513857d`，amd64 digest 为
`sha256:6e979419dd9ab1f1ebbccd664a1cc2b21cd439dfa5caa1ba63cbd99e2ff895b9`，arm64 digest 为
`sha256:59fa4a44007bd57a7516b35ccefb1077107e8fdecd98b8546893c080ab876e70`，SPDX SBOM SHA-256
为 `f071f64fcc656ed6d6f4ef4b17f435da294265cbb239b9772716b76b61010250`。发布流程、artifact
attestation、恢复、readiness、告警和私有化切换门禁均已通过，并已完成 OPC promotion。

## 管理员专用化审查分支

发布/审查分支 `revision/exapi-v0.2.1` 截至 2026-09-01 已继续完成管理员界面加固。
以下可独立回滚的提交均已通过各自的本地重点/完整检查，以及 GitHub CI 和安全扫描：

以下条目中的“生产环境未修改”等状态短语，均指各审查提交记录时的状态。所有列出的
提交都是 v0.2.14 的祖先，因此均已包含在当前部署镜像中，除非条目明确说明例外。

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
- `6d8e39679`：代理专用导入与账号/代理合并导入现在使用一致的代理状态同步失败
  文案，管理员看到的结构化部分失败结果更明确。生产环境未修改。
- `72aa62eea`：新增复用及新建代理状态同步失败的回归测试，并将 Phase 3 证据记录
  纳入 Git；完整 GitHub CI 与安全扫描均通过。生产环境未修改。
- `f850a0f14`：将管理员批量生图页面迁移到明确的控制面路由
  `/admin/batch-images`，同时保留 `/batch-image` 作为双语退役页面；导航、预取、
  路由矩阵测试、类型检查和变更文件 lint 均通过。生产环境未修改。
- `7a9917c3f`：为管理员 API Key 分组变更加上逐 Key 进行中的请求保护并禁用控件，
  防止快速连续修改产生竞态；延迟请求回归测试、类型检查和变更文件 lint 均通过。
  生产环境未修改。
- `049ee862b`：定时账号测试计划在访问 provider 前通过 PostgreSQL 有界租约
  （`FOR UPDATE SKIP LOCKED`）原子认领，并仅允许租约持有者完成更新，避免多副本
  重复执行和旧 worker 覆盖新计划。重点测试、仓储/服务测试、`go vet` 与 race 测试
  均通过。生产环境未修改。

审查分支还包含代理部分更新契约加固：省略的生命周期和凭据字段会保留现值，显式
null/空值则会清除对应可空设置。这部分在 v0.2.14 之前属于审查内容，未包含在 v0.2.10
生产镜像中；其提交是 v0.2.14 的祖先，已包含在当前部署镜像中。

此前的仅审查提交新增了中英文管理员专用产品范围契约（`bdd4329e3`），让所有受
支持的部署示例默认使用 `RUN_MODE=simple`，同时保留后端传统 `standard` 回退，并
让 OAuth/setup-token 导入按管理员名称、已验证邮箱或服务商回退值生成账号显示名称
（`8b25ba36c`）。账号/私有路由聚焦测试、前端类型检查与构建、部署绑定/安全/发布
契约和 release contract 均通过。这些提交未包含在 v0.2.10 镜像中，但已作为 v0.2.14
祖先提交并随当前镜像部署到 OPC。

审查提交 `da3e56788` 将保留的私有安全边界加入管理员设置页；安全页为只读时
不会触发全局设置保存；并将安全、OAuth、分页和 404 文案统一本地化为英文及简体中文。
该提交新增安全页和保留设置页签的回归覆盖；完整前端套件（267 个文件、1,435 个测试）、
类型检查、lint、生产构建、bundle 预算和部署契约均通过。该提交仅存在于
`revision/exapi-v0.2.1`，未包含在 v0.2.10 镜像中，但已包含在当前 v0.2.14 部署中。
对应的 GitHub CI run `33607824144` 与 Security Scan run `33607824171` 均已成功完成。

## OPC 生产部署

生产部署位于 `/opt/sub2api`，Compose project 为 `sub2api`，使用：

- `/opt/sub2api/.env.v0.2.14`
- `/opt/sub2api/docker-compose.v0.2.14.yml`

应用容器固定为 immutable digest
`sha256:e8a6d161a1acb5d454a13526ef2914533d077fd5aefae7a412bc45f58513857d`，版本
`0.2.14`，revision 为 `4b0352fa87720425bf4fb5c23aa91e2c0e212c9e`。应用、PostgreSQL
和 Redis 均为 healthy；PostgreSQL/Redis 未被重建，三者重启次数均为 0。

切换前恢复集 `exapi-v0214-recovery-20260902a` 保存在加密、版本化的异地对象中，保留至
2027-09-03。logical restore 使用一次性目标
`exapi-logical-restore-exapi-v0214-recovery-20260902a`、卷
`exapi-logical-restore-exapi-v0214-recovery-20260902a-data`、数据库 `exapi_restore`，
以及备份版本 `b6156b4a-75b4-4877-8684-7d58234dde0c`；独立 physical restore 使用
snapshot `7af6943e-9a5d-4f06-bc8f-282e55ccc333`。两条恢复路径均验证了加密、完整性和
网络隔离。

v0.2.14 restored-data canary（`exapi-v0214-restored-observe-20260902c`）和
synthetic-provider canary（`exapi-v0214-synthetic-20260902a`）各运行 30 分钟，均完成
60/60 readiness 检查，失败数、重启数和意外 5xx 均为 0，并取得相应 provider/出站或
解密/完整性证明。异地监控同时验证了 503 critical alert 和 200 recovery alert。

最终 rollout manifest 位于
[`tmp/rollouts/exapi-v0214-cutover-20260902a/rollout-manifest.json`](../tmp/rollouts/exapi-v0214-cutover-20260902a/rollout-manifest.json)，
SHA-256 为 `b80f4b13211c78f19b9d5503cdf86c6e1c39461d558a07279a0caec0fcca2eb5`。该清单已
验证、使用受保护 cosign 密钥签名、完成 provenance 验证，并以 COMPLIANCE 模式保留在
`s3://exapi-rollout-records/exapi-v0214-cutover-20260902a` 至 2027-09-03。对象版本为：
manifest `14dd653e-b76e-452d-8b49-f7b44121c967`、checksum
`69f549d8-fb36-4701-bea4-f217ef07c967`、签名包
`fcfbfb51-7081-476a-baf8-4e5010665d2f`、provenance 证明
`0fd6d018-7fe9-414b-ae67-e41701e45cc2`。

发布后的本地门禁仍全部通过：前端 Vitest `266` 个文件、`1432` 个测试通过，`vue-tsc`
类型检查和两个 bundle 预算检查通过，snapshot evidence 单元测试 `5` 个通过，生产
rollout contract 通过。

## 生产观察

`exapi-v0214-production-observe-20260902a` 运行 60 分钟、30 秒间隔，共 120 次
readiness probe：

- readiness failures `0`，unexpected 5xx `0`，重启 `0`；
- error rate `0.0`，p95 `4.0 ms`，基线 `4.852 ms`；
- 新增 P0/P1 告警 `0`，生产拓扑和依赖身份验证通过。

维护关闭后，公网 `/health` 和 `/ready` 均返回 200；从 allowlisted WireGuard
operator peer 访问 control endpoint、control `/ready`、`/api/v1/operator/me` 和只读账号
列表均返回 200。公开状态记录有意省略私有监听地址；公网根路径及公网 control route 返回
404。

## v0.2.14 账号探测行为

手动 provider 测试会将有界、脱敏的结果写入 `account.extra.account_test_probe`；该
结果只用于显示与诊断，不会改变调度状态。失败的手动测试不会改变 `account.status` 或
`schedulable`；后台测试不会覆盖最新的手动结果；凭据、原始 provider response body、
路由、代理或重复账号变更不会被写入快照，相关账号变更会清理旧结果。详细契约见
[`ACCOUNT_PROBES.md`](ACCOUNT_PROBES.md)。

## 兼容性与回滚

v0.2.14 的恢复校验记录 schema migration `246`、`private_schema_version=2` 和
`batch_image_jobs=0`。本次已完成 ciphertext-only private cutover；回滚必须使用
保留的 snapshot、上一版 immutable 应用 digest、上一版环境文件和匹配的 keyroots，
禁止仅替换应用镜像，也不要重建 PostgreSQL/Redis。通用门禁和命令见
[`../deploy/PRODUCTION_ROLLOUT.md`](../deploy/PRODUCTION_ROLLOUT.md)。

返回英文标准记录：[`PROJECT_STATUS.md`](PROJECT_STATUS.md)。
