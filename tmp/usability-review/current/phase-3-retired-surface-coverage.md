# Phase 3 retired customer surface coverage / Phase 3 普通用户面退役覆盖

Date: 2026-09-02

## Scope / 范围

The stable retired-surface middleware now covers both historical personal
profile URLs (`/api/v1/users/...`) and the public model-plaza surface
(`/api/v1/model-plaza...`). Both return the same machine-readable `410 Gone`
contract as the existing customer, payment, subscription, and affiliate
surfaces. Segment-boundary matching remains strict, so similarly named
operator or unrelated paths are not intercepted.

稳定的普通用户面退役中间件现在同时覆盖历史个人资料接口
(`/api/v1/users/...`) 和公开模型广场接口 (`/api/v1/model-plaza...`)。两者
都返回与既有客户、支付、订阅和推广接口相同的机器可读 `410 Gone` 契约。
路径仍按段边界严格匹配，不会拦截相似命名的运维或无关接口。

## Evidence / 证据

- `backend/internal/server/middleware/retired_customer_surface.go`: added
  `/api/v1/users` and `/api/v1/model-plaza` prefixes.
- `backend/internal/server/middleware/retired_customer_surface_test.go`: added
  positive 410 cases for both static and nested dynamic customer routes, plus
  near-prefix non-interception cases.
- Go focused test could not run in this checkout because neither `go` nor
  `gofmt` is installed; the patch is gofmt-compatible and `git diff --check`
  passes. GitHub CI remains the authoritative Go build/test gate.

- `backend/internal/server/middleware/retired_customer_surface.go`：新增
  `/api/v1/users` 与 `/api/v1/model-plaza` 前缀。
- `backend/internal/server/middleware/retired_customer_surface_test.go`：新增
  静态及动态嵌套客户路由的 410 正例，以及相似前缀不拦截反例。
- 当前 checkout 没有安装 `go` 或 `gofmt`，因此无法本地运行 Go focused
  test；补丁符合 gofmt 格式，且 `git diff --check` 已通过。GitHub CI 仍是
  Go 构建与测试的权威门禁。

## Compatibility / 兼容性

Operator key, group, usage, account, proxy, channel, control-plane, and
gateway prefixes are unchanged. Production OPC was not modified or restarted.

管理员 API Key、分组、用量、账号、代理、渠道、控制面和网关前缀均未改变。
OPC 生产环境未修改、未重启。
