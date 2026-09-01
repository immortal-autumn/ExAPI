# API Key create idempotency / API Key 创建幂等性

Date: 2026-09-01

## Decision / 决策

The create handler now stores a sanitized idempotency response: the first
successful response contains the raw API key, while a replay never returns the
secret. This prevents the idempotency store from retaining a reusable API key.
The browser UI does not automatically attach an idempotency key yet, because a
secret-redacted replay cannot recover a key when the original response was
lost; this remains an explicit follow-up contract decision.

创建处理器现在保存脱敏后的幂等响应：首次成功响应包含原始 API Key，重放响应
永远不返回 secret，避免幂等存储保留可复用密钥。浏览器 UI 暂未自动附加幂等键，
因为脱敏重放无法在首次响应丢失时恢复密钥；这仍是后续需要明确的契约决策。

## Verification / 验证

```bash
cd backend
PATH="../tmp/toolchains/go-1.26.6/go/bin:$PATH" \
  go test -tags unit ./internal/handler -run \
  'TestAPIKeyHandlerCreateIdempotencyRedactsSecretOnReplay|TestExecuteUserIdempotentJSON' -count=1
PATH="../tmp/toolchains/go-1.26.6/go/bin:$PATH" \
  go vet -tags=unit ./internal/handler
```

The focused tests and vet passed. A full handler unit suite also passed
(`36.345s`). No production deployment was performed.
