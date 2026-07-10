# ExAPI Refactor Regression Checklist

## Public routes must stay hidden

These routes are control-plane UI/API surfaces and must not become reachable through a public AI-only domain:

- `/`
- `/home`
- `/login`
- `/admin/dashboard`
- `/api/v1/auth/login`
- `/api/v1/auth/local-admin`
- `/api/v1/admin/dashboard`
- `/api/v1/user/profile`

Expected public status: `404`.

## Public AI endpoints stay reachable but protected

These gateway-compatible endpoints may remain reachable through a public AI-only domain, but unauthenticated requests must stay protected:

- `/v1/models`
- `/v1beta/models`
- `/backend-api/codex/models`
- `/antigravity/models`

Expected public unauthenticated status: `401` JSON.

## Private control plane stays reachable

These private control-plane entry points must remain reachable from localhost/WireGuard after deployment:

- `http://127.0.0.1:8027/admin/dashboard`
- `http://100.97.17.1:8027/admin/dashboard`

## Compatibility identifiers intentionally retained

Do not rename these as part of structural refactors:

- Go module path: `github.com/Wei-Shaw/sub2api`
- Runtime path: `/opt/sub2api`
- Container path/binary convention: `/app/sub2api`
- Environment variable prefix: `SUB2API_*`
- Existing DB/cache/service identifiers using `sub2api`

## Standard local gates

```bash
cd frontend
./node_modules/.bin/vitest run src/i18n/__tests__/zh-only.spec.ts src/config/__tests__/brand.spec.ts src/i18n/__tests__/brand-copy.spec.ts src/router/__tests__/title.spec.ts src/stores/__tests__/app.spec.ts src/utils/__tests__/singleUserCockpit.spec.ts src/views/__tests__/KeyUsageView.spec.ts
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build
```

```bash
cd backend
GOTOOLCHAIN=auto go test ./internal/brand ./internal/setup ./internal/service ./internal/server ./internal/repository ./internal/payment/provider
GOTOOLCHAIN=auto go test -tags embed ./internal/web
```

```bash
.hermes/scripts/check-exapi-brand.sh
python3 .hermes/scripts/audit-large-files.py | head -30
```
