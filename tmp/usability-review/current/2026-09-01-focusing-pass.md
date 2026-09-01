# Frontend usability / safety pass — 2026-09-01

简体中文：[中文记录](2026-09-01-focusing-pass.zh-CN.md)

Scope:

- Antigravity OAuth blank-name fallback and batch naming
- Admin API key regenerate/delete confirmation
- Grok SSO import client-side token limit guard

Verification:

- `pnpm exec vitest run src/components/account/__tests__/CreateAccountModal.spec.ts src/views/admin/settings/__tests__/SecurityAdminApiKeyPanel.spec.ts --reporter verbose`

Result:

- Passed: 2 test files, 21 tests
- The prior Vue warning from the OAuth flow stub is gone after declaring `validate-refresh-token` and `import-sso` emits
- Grok SSO imports over `IMPORT_MAX_OBJECTS` now stop before the API call

Notes:

- This pass stays intentionally bounded to frontend behavior and focused tests.
- No commit was created in this phase.
