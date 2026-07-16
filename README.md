# ExAPI

ExAPI 是一个面向个人/私有基础设施的 AI API 网关，用于把本地工具、编程 Agent 和应用流量路由到可管理的上游 AI 账号池。

本项目 fork 自 `Wei-Shaw/sub2api`。为了兼容已有部署，部分内部模块、数据库、缓存、服务名仍保留 `sub2api` 标识；但本 fork 的产品方向是私有控制面、配额可见性、多账号韧性和本地集成体验。

## 主要特性

- OpenAI 兼容 `/v1` 网关。
- 继承上游的 Claude / Codex / Gemini / Antigravity 等客户端兼容路由。
- 适合单用户部署的 localhost / WireGuard 私有控制面。
- 上游账号池、定时账号测试、配额窗口、用量日志。
- 面向 IDE、Agent、自动化脚本的 API Key 管理。
- 保留上游多用户、订阅、支付、兑换码等可选能力，但不作为本 fork 的默认重点。

## 部署模式

### 私有单用户模式

推荐用于个人服务器：

- 公网域名只暴露 AI 网关路径；
- 管理后台仅通过 localhost 或 WireGuard/VPN 访问；
- 侧边栏优先展示账号、API Key、用量、代理、运维和设置；
- 控制台直接展示配额监控和本地集成入口。

### 标准多用户模式

ExAPI 仍保留上游的 SaaS 风格功能，例如用户、订阅、支付、分组、兑换码等。它们适合团队/受控场景，但不是本 fork 的默认产品重点。

## 技术栈

- 后端：Go、Gin、Ent、PostgreSQL、Redis。
- 前端：Vue 3、TypeScript、Pinia、Vue Router、TailwindCSS、Vite。
- 部署：Docker/Compose 或 systemd，通常放在 nginx/Caddy/Cloudflare 后面。

Go module 路径和不少运行时名称仍保留 `sub2api`。尝试完整重命名 module、服务、数据库或数据目录前，请先阅读 [`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md)。

## 快速开始

Docker 部署请从 [`deploy/`](deploy/) 下的模板开始。除非你执行了完整迁移，否则上游文档中涉及 `sub2api` 路径/服务名的部署命令仍应保持兼容。

ExAPI 私有部署建议配置：

```env
RUN_MODE=simple
SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true
SUB2API_PUBLIC_HOST=your-public-ai-gateway.example.com
# Required external root; docker-deploy.sh generates these securely.
SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data-v1
SUB2API_DATA_ENCRYPTION_KEYS_JSON={"data-v1":"<base64-encoded-32-byte-key>"}
```

The external data keyring is mandatory and must be retained outside PostgreSQL
and ordinary data backups. Prefer `deploy/docker-deploy.sh` or `deploy/install.sh`,
which generate and permission it automatically; see [`deploy/README.md`](deploy/README.md)
for manual generation, rotation, migration, and recovery guidance.

公网只放行 AI 网关路径，`/admin`、`/login`、`/api/v1/*` 控制面 API 应保持私有。

## 安全声明

通过网关使用上游消费者 AI 账号可能违反服务商条款。请自行阅读相关条款、遵守所在地法律，并仅使用你有权操作的账号和流量。本项目仅用于技术研究和自托管基础设施场景；部署、账号和数据风险由使用者自行承担。

## 上游致谢

ExAPI 派生自 Wei-Shaw 及贡献者维护的 Sub2API 开源项目。本 fork 默认 README 不再展示上游赞助/推广内容，而是聚焦私有运维工作流。兼容性说明见 [`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md)。
