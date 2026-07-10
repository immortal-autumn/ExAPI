# ExAPI Rename Inventory

Generated from:

```bash
rg -n --hidden \
  --glob '!frontend/node_modules/**' \
  --glob '!backend/internal/web/dist/**' \
  --glob '!.git/**' \
  --glob '!frontend/tsconfig.tsbuildinfo' \
  'Sub2API|Sub2Api|sub2api|SUB2API' .
```

Observed total before implementation: **4156** matches. The high count is expected because generated Ent files and Go imports use the upstream module path `github.com/Wei-Shaw/sub2api`.

## Replace now: product/display strings

- `frontend/package.json`: package name `sub2api-frontend` -> `exapi-frontend`.
- `frontend/src/stores/app.ts`: fallback `Sub2API` -> centralized ExAPI brand default.
- `frontend/src/router/title.ts`: browser title fallback `Sub2API` -> ExAPI.
- `frontend/src/views/auth/RegisterView.vue`: auth fallback site name `Sub2API` -> ExAPI.
- `frontend/src/views/auth/EmailVerifyView.vue`: auth fallback site name `Sub2API` -> ExAPI.
- `frontend/src/views/public/LegalDocumentView.vue`: public legal fallback site name `Sub2API` -> ExAPI.
- `frontend/src/components/layout/AuthLayout.vue`: auth layout fallback site name `Sub2API` -> ExAPI.
- `frontend/src/views/admin/SettingsView.vue`: site/payment defaults/placeholders `Sub2API` -> ExAPI.
- `frontend/src/i18n/locales/{en,zh}/misc.ts`: onboarding copy `Sub2API` -> ExAPI.
- `frontend/src/i18n/locales/{en,zh}/landing.ts`: setup copy `Sub2API` -> ExAPI.
- `frontend/src/i18n/locales/{en,zh}/admin/settings.ts`: visible admin settings copy/placeholders `Sub2API` -> ExAPI.
- `frontend/src/views/admin/AccountsView.vue`: cosmetic export filename prefix `sub2api-account-` -> `exapi-account-`.
- `frontend/src/views/admin/ProxiesView.vue`: cosmetic export filename prefix `sub2api-proxy-` -> `exapi-proxy-`.
- `backend/cmd/server/main.go`: startup/setup visible product strings -> ExAPI.
- `backend/internal/setup/cli.go`: setup wizard visible product strings -> ExAPI.
- `backend/internal/setup/setup.go`: default admin email -> `admin@exapi.local`.
- `backend/internal/web/embed_test.go`: bundled web title fixtures -> ExAPI.
- `deploy/Dockerfile`, root `Dockerfile`, goreleaser Dockerfiles: display metadata/comments -> ExAPI while preserving runtime binary paths.
- `deploy/config.example.yaml`, `deploy/README.md`, `deploy/DOCKER.md`, `deploy/docker-deploy.sh`, `deploy/sub2api.service`: display/docs text -> ExAPI where safe.
- Top-level `README.md`, `README_CN.md`, `README_JA.md`: rewrite as ExAPI fork docs and quarantine/remove upstream sponsorship content.

## Keep for compatibility in this first pass

- `backend/go.mod`: `module github.com/Wei-Shaw/sub2api`.
- Go imports under `backend/**`: `github.com/Wei-Shaw/sub2api/...`.
- Generated Ent package comments/imports under `backend/ent/**`.
- Docker/Linux runtime user/group/service/binary paths: `sub2api`, `/opt/sub2api`, `/app/sub2api`.
- Database defaults: `sub2api` DB/user where changing would require migration.
- Redis/localStorage/cache keys: `sub2api:` and `sub2api_*` where changing would invalidate existing clients.
- WebSocket subprotocol `sub2api-admin` in ops APIs.
- OAuth/referrer test `sub2api` where upstream provider expects a known value.
- Historical compatibility names such as `sub2apipay`.

## Needs explicit migration approval later

- Rename repo/module path to `github.com/immortal-autumn/exapi`.
- Rename systemd unit from `sub2api.service` to `exapi.service`.
- Rename Docker Compose service/container from `sub2api` to `exapi`.
- Rename binary `/app/sub2api` to `/app/exapi`.
- Rename Linux user/group `sub2api` to `exapi`.
- Rename data/config paths `/opt/sub2api` and `/etc/sub2api`.
- Rename DB, Redis prefix, localStorage keys, and cache databases.

## Verification target

After implementation, run `scripts/check-exapi-brand.sh`. Remaining `Sub2API`/`sub2api` matches should be compatibility allowlisted, upstream attribution/history, or intentionally deferred runtime identifiers.
