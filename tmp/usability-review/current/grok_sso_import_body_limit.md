# Grok SSO import request-body limit evidence

Date: 2026-09-01

## Change

- Added an explicit 25 MiB request-body limit to `GrokOAuthHandler.CreateAccountsFromSSO`.
- The handler now rejects oversized requests before any OAuth client call.
- The rejection path matches the existing admin import contract: HTTP 413 with `Import exceeds 25 MiB request limit`.

## Verification

Focused handler test run:

```bash
PATH="/media/u5531440/My Passport/ExAPI/tmp/toolchains/go-1.26.6/go/bin:$PATH" go test -tags unit ./internal/handler/admin -run 'TestGrokSSO|TestAccountCreateWithoutAutomaticGrokProbeServiceStillSucceeds'
```

Result: passed.

## Notes

- No commit was created for this bounded fix.
- The change stays within the admin Grok SSO import boundary and does not alter OAuth service behavior.
