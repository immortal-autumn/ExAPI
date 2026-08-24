# ExAPI 文档导航

[English（默认）](README.md) | 简体中文

文档分为持续维护的项目与运维指南、功能契约、兼容性参考和历史设计证据。

## 语言规则

- 不带语言后缀的文档路径（例如 `README.md`）以英文为标准版本和默认语言。
- 简体中文译文使用 `.zh-CN.md`；为了避免破坏已有链接，现有 `_CN.md` 文件在迁移前
  仍然有效。
- 成对译文必须在文件顶部互相链接。运维命令、配置键、API 字段、digest 和安全要求
  必须保持语义一致。
- 行为发生变化时，先更新英文标准版本，并在同一变更中同步中文译文。如果译文暂时
  落后，必须标明最后同步版本，不能把过期说明当作当前行为。
- `openspec/changes/` 下的历史证据保留原始语言；当前运维说明应写入下列持续维护文档。

## 项目与运维文档

- [`PROJECT_STATUS.md`](PROJECT_STATUS.md) — 当前发布、部署、验证和外部状态的标准记录。
- [`PROJECT_STATUS.zh-CN.md`](PROJECT_STATUS.zh-CN.md) — 当前状态记录的简体中文镜像；英文仍为默认入口。
- [`ACCOUNT_PROBES.md`](ACCOUNT_PROBES.md) — 手动探测和强制用量刷新语义。
- [`../development.md`](../development.md) — 当前开发优先级和强制质量门禁。
- [`../deploy/README.md`](../deploy/README.md) — 部署入口。
- [`../deploy/PRODUCTION_ROLLOUT.md`](../deploy/PRODUCTION_ROLLOUT.md) — 恢复、
  canary、发布、观察和回滚门禁。
- [`../deploy/EDGE_SECURITY.md`](../deploy/EDGE_SECURITY.md) — 公网与控制监听器、
  反向代理安全边界。

## 兼容性与上游控制

- [`UPSTREAM_COMPATIBILITY.md`](UPSTREAM_COMPATIBILITY.md) — 保留的 `sub2api`
  运行时标识和迁移边界。
- [`UPSTREAM_LOCK.md`](UPSTREAM_LOCK.md) — 已审计的上游基线和锁定验证。
- [`../backend/migrations/README.md`](../backend/migrations/README.md) — 前向迁移行为。
- [`../backend/resources/model-pricing/README.md`](../backend/resources/model-pricing/README.md)
  — 内嵌定价资源契约。

## 功能契约

- [`ASYNC_IMAGE_TASKS.md`](ASYNC_IMAGE_TASKS.md)
- [`BATCH_IMAGE_MVP.md`](BATCH_IMAGE_MVP.md)
- [`COMPOSITE_GROUPS.md`](COMPOSITE_GROUPS.md)
- [`PAYMENT.md`](PAYMENT.md) 和 [`PAYMENT_CN.md`](PAYMENT_CN.md)
- [`ADMIN_PAYMENT_INTEGRATION_API.md`](ADMIN_PAYMENT_INTEGRATION_API.md)

这些文档描述各自子系统，可能包含私有部署默认不启用的多用户可选能力。

## 开发与设计参考

- [`../DEV_GUIDE.md`](../DEV_GUIDE.md) — 仓库开发约定。
- [`design/EXAPI_UI_DIRECTION.md`](design/EXAPI_UI_DIRECTION.md) — 私有运维界面方向。
- `frontend/src/` 下组件目录中的 `README.md`、`INTEGRATION.md` 和 `EXAMPLES.md`
  只描述各自的路由、store、视图或组件。
- [`../skills/sub2api-admin/SKILL.md`](../skills/sub2api-admin/SKILL.md) 及其参考文件
  描述仓库内置的后台自动化 skill。

## 历史证据

`openspec/changes/` 下的文件是特定变更的设计、源码冻结、实现和验证证据，应视为历史
记录。当当前行为取代这些内容时，应从历史文件链接到持续维护文档，不要把历史证据改写
成当前说明。

## 维护规则

- 每个可变事实只保留一个标准位置，其他文档使用链接引用。
- 生产制品必须固定到 digest，不得把 `latest` 写成生产部署方式。
- 不得提交环境文件、服务商凭据、原始探测响应、数据库转储、签名密钥或私有运维地址。
- 部署观察必须带日期，并把外部服务商状态标记为临时状态。
- 公共 API、运维流程、部署不变量或兼容边界变化时，必须在同一变更中更新文档。
- 仓库、release、安装器和外部文档链接默认指向英文，并从英文页链接到对应中文译文。
