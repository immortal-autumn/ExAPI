# ExAPI 项目状态

这是 ExAPI fork 的标准双语状态记录。发布、生产部署、回滚或运维不变量变化时，
请同步更新本文件和英文版 [`PROJECT_STATUS.md`](PROJECT_STATUS.md)。英文文档仍是
默认入口；本文件提供简体中文对应内容。

## 当前发布

最后审阅：**2026-08-24（Europe/London）**

| 项目 | 当前值 |
|---|---|
| 产品版本 | `0.2.6` |
| GitHub 仓库 | `immortal-autumn/ExAPI` |
| Git tag | `v0.2.6` |
| 主分支 | `main`（已快进到审阅提交） |
| 发布分支 | `revision/exapi-v0.2.1` |
| 审阅提交 | `8363e0decd68786e02c9620e616e17f1284e0ff2` |
| OCI 镜像 | `ghcr.io/immortal-autumn/sub2api2personal@sha256:5ef74f0df89989ae7922fa819ac67ea159c8769871173fba33548baf0a708b43` |
| GitHub Release | <https://github.com/immortal-autumn/ExAPI/releases/tag/v0.2.6> |
| Release workflow | <https://github.com/immortal-autumn/ExAPI/actions/runs/32739586602> |

镜像经过多架构构建、OCI 标签、SPDX SBOM、SLSA/ SBOM attestations 和
`gh attestation verify` 验证。生产环境只使用上述 immutable digest，不使用
`latest` 等可变标签。GHCR 包名仍保留 `sub2api2personal` 以兼容现有部署。

## 管理员专用化审查分支

非生产审查分支 `revision/exapi-v0.2.1` 截至 2026-09-01 已继续完成管理员界面加固。
以下可独立回滚的提交均已通过各自的本地重点/完整检查，以及 GitHub CI 和安全扫描：

- `30f5dc180`：日常代理凭据最小暴露；
- `3453be8cf`：代理破坏性操作的界面防护；
- `faca8d3e0`：代理创建/更新请求的进行中防重复提交保护；
- `99cfb8a64`：代理 JSON 导入请求的进行中防重复提交保护。

下一项未推送的审查范围是代理删除接口的输入和结果正确性。它尚未纳入上面的生产
版本，也没有部署；在按发布流程完成单独验证并 promotion 前，生产仍保持 v0.2.6 的
已审查 immutable digest。

## OPC 生产部署

生产部署位于 `/opt/sub2api`，Compose project 为 `sub2api`，当前输入为：

- `/opt/sub2api/.env.v0.2.6`
- `/opt/sub2api/docker-compose.v0.2.6.yml`

`/opt/sub2api/.env` 和 `docker-compose.local.yml` 保留旧的本地 provenance 及 v0.2.5
回滚 digest；生产 promotion 明确使用上面的版本化文件。

应用容器版本为 `0.2.6`，OCI revision 为上述提交，状态 healthy、重启次数为 0。
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
