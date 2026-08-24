# ExAPI

ExAPI 是面向个人和私有基础设施的 AI API 网关，用于把本地工具、编程 Agent
和应用流量路由到可管理的上游 AI 账号池。

本项目 fork 自 `Wei-Shaw/sub2api`。为了兼容已有部署，部分内部模块、数据库、
缓存和服务名仍保留 `sub2api` 标识；ExAPI 的产品方向是私有控制面、配额可见性、
多账号韧性和本地集成体验。

[English（默认）](README.md) | 简体中文

## 当前状态

当前已审阅并部署的版本是 **ExAPI v0.2.5**，对应提交
`14a7c412a17971b160de356baaab7a3555fb90fa`。生产环境只使用经过发布工作流验证、
带 SBOM/来源证明的不可变 OCI digest，不使用 `latest` 等可变标签。

- 当前发布、镜像 digest、部署验证和已知上游账号状态：
  [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md)
- 文档导航与维护规则：[`docs/README.zh-CN.md`](docs/README.zh-CN.md)
- 生产发布与回滚门禁：
  [`deploy/PRODUCTION_ROLLOUT.md`](deploy/PRODUCTION_ROLLOUT.md)

## 主要特性

- OpenAI 兼容 `/v1` 网关。
- 继承上游的 Claude、Codex、Gemini、Antigravity 等客户端兼容路由。
- 适合单用户部署的 localhost/WireGuard 私有控制面。
- 上游账号池、定时账号测试、配额窗口和用量日志。
- 手动账号探测与调度状态相互独立，失败结果可见但不会静默停用账号。
- Antigravity 强制实时配额刷新、按模型配额展示和 Google 429 分类。
- 面向 IDE、Agent 和自动化脚本的 API Key 管理。
- 保留上游多用户、订阅、支付、分组和兑换码等可选能力，但不作为本 fork
  的默认重点。

## 部署模式

### 私有单用户模式

推荐用于个人服务器：

- 公网域名只暴露 AI 网关路径；
- 管理后台仅通过 localhost 或 WireGuard/VPN 访问；
- 侧边栏优先展示账号、API Key、用量、代理、运维和设置；
- 控制台直接展示配额监控和本地集成入口。

### 标准多用户模式

ExAPI 仍保留上游的 SaaS 风格功能，例如用户、订阅、支付、分组和兑换码。
它们适合团队或受控场景，但不是本 fork 的默认产品重点。

## 技术栈

- 后端：Go、Gin、Ent、PostgreSQL、Redis。
- 前端：Vue 3、TypeScript、Pinia、Vue Router、TailwindCSS、Vite。
- 部署：Docker/Compose 或 systemd，通常放在 nginx、Caddy 或 Cloudflare 后面。

Go module 路径和不少运行时名称仍保留 `sub2api`。尝试完整重命名 module、服务、
数据库或数据目录前，请先阅读
[`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md)。

## 快速开始

Docker 部署请从 [`deploy/`](deploy/) 下的模板开始。除非执行了完整迁移，否则上游
文档中涉及 `sub2api` 路径或服务名的部署命令仍应保持兼容。

ExAPI 私有部署建议配置：

```env
RUN_MODE=simple
SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true
SUB2API_PUBLIC_HOST=your-public-ai-gateway.example.com
# Required external root; docker-deploy.sh generates these securely.
SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data-v1
SUB2API_DATA_ENCRYPTION_KEYS_JSON={"data-v1":"<base64-encoded-32-byte-key>"}
```

外部数据密钥环是强制要求，必须保存在 PostgreSQL 和普通数据备份之外。推荐使用
`deploy/docker-deploy.sh` 或 `deploy/install.sh`，脚本会自动生成并设置权限；手动
生成、轮换、迁移和恢复请参阅 [`deploy/README.md`](deploy/README.md)。

公网只放行 AI 网关路径，`/admin`、`/login`、`/api/v1/*` 控制面 API 应保持私有。
控制面必须从显式允许的 WireGuard peer 验证；服务器自身或未授权 peer 收到 404
是预期的隐藏行为，不代表控制进程未启动。详细边界见
[`deploy/EDGE_SECURITY.md`](deploy/EDGE_SECURITY.md)。

## 账号探测与配额诊断

管理员手动测试账号时，最新结果保存在 `account.extra.account_test_probe`，但不会
直接修改 `account.status` 或 `schedulable`。定时测试不会覆盖手动结果；凭据、路由
或代理发生实质变化时旧结果会失效。

Antigravity 的实时查询使用 `force=true` 绕过后端配额缓存。配额查询成功只说明
配额接口可达，不等价于推理请求一定可用；仍需选择上游当前公布的模型执行手动探测。
完整运维流程见 [`docs/ACCOUNT_PROBES.md`](docs/ACCOUNT_PROBES.md)。

## 安全声明

通过网关使用上游消费者 AI 账号可能违反服务商条款。请自行阅读相关条款、遵守所在地
法律，并仅使用你有权操作的账号和流量。本项目仅用于技术研究和自托管基础设施场景；
部署、账号和数据风险由使用者自行承担。

## 上游致谢

ExAPI 派生自 Wei-Shaw 及贡献者维护的 Sub2API 开源项目。本 README 聚焦私有运维工作流，
不展示上游赞助或推广内容。兼容性说明见
[`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md)。
