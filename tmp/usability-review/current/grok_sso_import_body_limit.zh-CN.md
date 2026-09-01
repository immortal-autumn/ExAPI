# Grok SSO 导入请求体上限证据

对应英文记录：[English record](grok_sso_import_body_limit.md)

## 变更

- `GrokOAuthHandler.CreateAccountsFromSSO` 增加明确的 25 MiB 请求体上限；
- 超限请求在调用 OAuth client 之前返回 HTTP 413；
- 错误信息与现有管理员导入契约一致：`Import exceeds 25 MiB request limit`。

## 验证

```bash
PATH="/media/u5531440/My Passport/ExAPI/tmp/toolchains/go-1.26.6/go/bin:$PATH" \
  go test -tags unit ./internal/handler/admin \
  -run 'TestGrokSSO|TestAccountCreateWithoutAutomaticGrokProbeServiceStillSucceeds'
```

结果：通过。此 bounded fix 未部署 OPC。
