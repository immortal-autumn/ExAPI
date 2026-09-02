# Phase 3 proxy-import error propagation / Phase 3 代理导入错误传播

Date: 2026-09-02

## Scope / 范围

Account/proxy backup import no longer discards errors from the post-create or
post-reuse proxy status synchronization. A synchronization failure is returned
in the structured import error list and increments
`proxy_failed`, so an operator cannot mistake a partially applied import for a
complete one.

账号/代理备份导入不再忽略创建后或复用后的代理状态同步错误。同步失败会写入
结构化导入错误列表并增加 `proxy_failed` 计数，使管理员不会把部分成功误认为
完整导入。

## Evidence / 证据

- `backend/internal/handler/admin/account_data.go`: propagates both existing
  and newly created proxy `UpdateProxy` failures.
- `backend/internal/handler/admin/proxy_data.go`: counts proxy-only status
  synchronization failures and uses the same explicit wording for existing and
  newly imported proxies.
- `backend/internal/handler/admin/proxy_data_handler_test.go`: reproduces
  reused- and newly-created-proxy synchronization failures and verifies the
  structured result fields and actionable messages.
- `backend/internal/handler/admin/admin_service_stub_test.go`: injectable update
  failure for the focused handler test.
- Focused local verification passed with the checkout-local Go toolchain:
  `go test -vet=off -tags unit ./internal/handler/admin -run
  '^TestProxyImportData(ReportsStatusSynchronizationFailure|ReportsCreatedProxyStatusSynchronizationFailure)$'`.
  `git diff --check` passes and GitHub CI remains the authoritative full gate.

- `backend/internal/handler/admin/account_data.go`：传播复用代理和新建代理的
  `UpdateProxy` 失败。
- `backend/internal/handler/admin/proxy_data.go`：代理专用导入会计入状态同步失败，
  并对复用及新建代理使用一致、明确的错误文案。
- `backend/internal/handler/admin/proxy_data_handler_test.go`：复现复用及新建代理的
  状态同步失败，并验证结构化结果字段和可操作错误文案。
- `backend/internal/handler/admin/admin_service_stub_test.go`：为 focused handler
  test 提供可注入的更新失败。
- 使用 checkout 内的 Go 工具链完成 focused 测试并通过；`git diff --check` 已通过，
  GitHub CI 仍是完整 Go 门禁的权威结果。
