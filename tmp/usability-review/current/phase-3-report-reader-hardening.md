# Phase 3 migration report reader hardening / Phase 3 迁移报告读取加固

Date: 2026-09-02

## Scope / 范围

The offline report verifier now fails closed on oversized report files and
checks the opened descriptor and canonical path before and after reading. It
also rejects size, modification-time, or inode changes during the read. This
limits memory exhaustion and common same-UID path-replacement races before
signature/database verification.

离线报告验证器现在会对超大报告文件 fail-closed，并在读取前后分别校验已
打开文件描述符与规范路径；读取期间发生大小、修改时间或 inode 变化时拒绝
继续。这在签名和数据库验证前限制了内存耗尽及常见的同 UID 路径替换竞态。

## Evidence / 证据

- `backend/cmd/verify-private-cutover-report/main.go`: bounded reads at 4 MiB,
  descriptor/path identity checks, and post-read stability checks.
- `backend/cmd/verify-private-cutover-report/main_test.go`: oversized reports
  are rejected and protected bounded files remain readable.
- The local checkout has no `go` or `gofmt`; Go unit/build validation is
  delegated to GitHub CI. `git diff --check` passes.

- `backend/cmd/verify-private-cutover-report/main.go`：4 MiB 读取上限、描述符/路径
  身份校验及读取后稳定性校验。
- `backend/cmd/verify-private-cutover-report/main_test.go`：验证超大报告被拒绝，
  受保护且大小正常的文件仍可读取。
- 当前 checkout 没有 `go` 或 `gofmt`；Go 单元测试和构建由 GitHub CI 执行，
  `git diff --check` 已通过。
