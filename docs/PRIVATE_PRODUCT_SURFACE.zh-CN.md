# ExAPI 私有产品范围

[English（默认）](PRIVATE_PRODUCT_SURFACE.md) | 简体中文

ExAPI 只有一种受支持的产品模式：管理员专用的私有控制面，用于运维上游 AI
账号池及其 API Key 网关。

## 支持的管理员能力

- 网关健康、模型路由、用量、错误和运维诊断。
- 上游账号与 OAuth 生命周期、配额检查和账号探测。
- API Key 创建、轮换、撤销、限制及路由分组绑定。
- 网关分组、通道定价/监控、代理、风控和提示词审计（按配置启用）。
- 加密备份/恢复和管理员安全设置。

### API 密钥安全处理

完整网关密钥只会在创建成功的响应中返回，并通过一次性对话框显示。
列表、详情、更新和幂等重放响应出于安全考虑只包含密钥前缀（或不包含
`key` 字段），因此无法恢复已有密钥。如果关闭了对话框或响应已被脱敏，
请撤销该密钥并重新创建；不要尝试从界面或数据库读取原始密钥。

## 已退役的客户能力

用户注册、客户登录/找回、自助服务、订阅、余额、支付、兑换码/促销码、推广、公告
和客户管理 API 均已退役。在私有模式下，它们不会出现在控制面导航中，也不会注册
为活动后端路由；旧 API 前缀返回稳定的 `CUSTOMER_SURFACE_RETIRED` 响应。

上游源码和数据库 schema 可能仍包含兼容代码及表。这不表示可以启用 SaaS 模式，也
不表示应删除历史 schema。任何后续删除都必须单独进行迁移和恢复评审。

## 路由与部署策略

浏览器允许列表维护在 `frontend/src/config/singleUserProduct.ts`；后端产品模式契约
位于 `backend/internal/config/product_mode.go`。新部署必须设置
`RUN_MODE=simple` 和 `SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true`；公网监听器只
暴露网关流量，控制监听器限制为 localhost/WireGuard 对等节点。

网络和发布门禁请参阅 [`../deploy/EDGE_SECURITY.md`](../deploy/EDGE_SECURITY.md) 及
[`../deploy/PRODUCTION_ROLLOUT.md`](../deploy/PRODUCTION_ROLLOUT.md)。
