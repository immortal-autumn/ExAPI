# Phase 4 external prerequisite audit / Phase 4 外部前置条件审计

Date: 2026-09-02 (Europe/London)

## Scope / 范围

This was a read-only audit of the OPC rollout prerequisites. No migration,
provider cleanup, snapshot creation, archive upload, container restart, or
production file change was performed.

这是对 OPC 发布前置条件的只读审计。未执行迁移、服务商清理、快照创建、归档上传、
容器重启或生产文件修改。

## Observed results / 观测结果

- SSH access to `opc@100.97.17.1` through the authorized WireGuard peer worked.
- The retained `sub2api` application, PostgreSQL, and Redis containers were
  healthy and unchanged; the application remained on the v0.2.7 immutable
  digest with zero restarts.
- Protected archive, snapshot, cleanup, monitoring, recipient, and offline
  identity adapter paths were present and readable by their intended root-only
  checks. Secret contents were not read or copied.
- `verify-alert-delivery` passed its existing proof check.
- `configure-external-readiness` failed closed because the off-host readiness
  proof was stale. The synthetic-provider proof was also older than the
  runbook freshness window.

- 已通过授权 WireGuard peer 成功 SSH 到 `opc@100.97.17.1`。
- 保留的 `sub2api` 应用、PostgreSQL 和 Redis 容器均 healthy 且未变化；应用仍运行
  v0.2.7 immutable digest，重启次数为 0。
- 受保护的归档、快照、清理、监控、recipient 和 offline identity 适配器路径均存在，
  并通过预期的 root-only 可读性检查；未读取或复制密钥内容。
- `verify-alert-delivery` 的现有证据检查通过。
- `configure-external-readiness` 因公网外部 readiness 证据过期而 fail-closed；
  synthetic-provider 证据也超出运行手册要求的新鲜度窗口。

## Gate decision / 门禁结论

Phase 4 remains **blocked**. Refresh the off-host readiness and synthetic
provider proofs, then rerun the non-destructive checks before considering a
dry-run or release tag. The OPC production deployment was not changed.

Phase 4 仍然**阻塞**。应先刷新公网外部 readiness 与 synthetic provider 证据，
再重新执行非破坏性检查，之后才能考虑 dry-run 或 release tag。OPC 生产部署未改变。
