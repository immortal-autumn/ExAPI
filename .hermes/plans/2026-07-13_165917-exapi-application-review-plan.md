# ExAPI Application Review Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Review the ExAPI private single-user gateway deployment and produce a clear report showing what is safe, what works, and what needs attention.

**Architecture:** This is a review-only runbook. Do not change application code or deployment settings while following it. The reviewer gathers evidence from the repo, Docker deployment, HTTP endpoints, browser UI, database metadata, and logs, then writes a report with findings and recommendations.

**Tech Stack:** Go/Gin backend, Vue 3/Vite/Pinia frontend, PostgreSQL, Redis, Docker Compose, nginx/public gateway, ExAPI OpenAI-compatible API routes, Vitest, Go tests, curl/browser checks.

---

## Quick Start

If you only have time for the essential review, run these sections in order:

1. [Safety Rules](#safety-rules)
2. [Review Checklist](#review-checklist)
3. [Step 1: Capture Baseline](#step-1-capture-baseline)
4. [Step 2: Check Public vs Private Access](#step-2-check-public-vs-private-access)
5. [Step 3: Confirm Update/Restart Is Disabled](#step-3-confirm-updaterestart-is-disabled)
6. [Step 4: Verify Gateway API Key Flow](#step-4-verify-gateway-api-key-flow)
7. [Step 5: Verify the Private UI](#step-5-verify-the-private-ui)
8. [Step 9: Write the Final Report](#step-9-write-the-final-report)

Save the final report here:

```text
/home/opc/src/sub2api/.hermes/review-artifacts/YYYY-MM-DD_exapi-application-review.md
```

---

## Safety Rules

Follow these throughout the review:

- Do **not** print secrets, OAuth tokens, cookies, API keys, `.env` contents, or full credential fields.
- Do **not** change source code, database rows, Docker Compose config, DNS, nginx, or deployment settings during review.
- Do **not** run update, rollback, restart, migration, or cleanup commands unless the user explicitly asks for remediation.
- Prefer read-only commands.
- If a command needs an API key, load it into a shell variable and never echo it.
- If output may contain secrets, redact before saving it into review artifacts.
- Treat `/home/opc/src/sub2api` as the repo under review.
- Treat `/opt/sub2api/docker-compose.local.yml` as the live deployment config.

---

## Review Checklist

Use this as the main progress tracker.

### Deployment baseline

- [ ] Current Git commit recorded.
- [ ] Current Docker image ID recorded.
- [ ] `sub2api`, `sub2api-postgres`, and `sub2api-redis` are healthy.
- [ ] Private single-user mode flag is confirmed.
- [ ] Public host is confirmed.

### Public/private boundary

- [ ] Public control-plane routes return `404`.
- [ ] Public gateway route without key returns `401 API_KEY_REQUIRED`.
- [ ] Local/private UI routes return `200`.
- [ ] Local API route without auth returns `401`, not `200`.

### Update allowance removal

- [ ] `/api/v1/admin/system/check-updates` returns `404` in private mode.
- [ ] `/api/v1/admin/system/update` returns `404` in private mode.
- [ ] `/api/v1/admin/system/rollback` returns `404` in private mode.
- [ ] `/api/v1/admin/system/restart` returns `404` in private mode.
- [ ] UI no longer shows update/check/rollback/restart text.
- [ ] Browser does not request update endpoints.

### Gateway function

- [ ] Missing API key fails with `401`.
- [ ] Invalid API key fails with `401`.
- [ ] Valid API key can call `/v1/models`.
- [ ] Valid API key can make one safe chat-completion smoke test.
- [ ] Usage log increments after smoke test.

### UI surface

- [ ] Sidebar only shows private gateway/admin items.
- [ ] Restricted legacy SaaS routes redirect to the single-user cockpit.
- [ ] Removed SaaS chunks are absent from deployed frontend assets.

### Secret handling

- [ ] Schema-level secret fields identified without selecting values.
- [ ] Logs scanned for raw keys/tokens/cookies using redaction.
- [ ] Report contains no raw secrets.

### Final report

- [ ] Findings are classified by severity.
- [ ] Evidence is included for each finding.
- [ ] Passed controls are listed as non-findings.
- [ ] Open questions are listed.
- [ ] No remediation was performed during review.

---

## Step 1: Capture Baseline

**Purpose:** Record exactly what version and deployment you reviewed.

**Artifact to create:**

```text
/home/opc/src/sub2api/.hermes/review-artifacts/baseline.md
```

### 1.1 Repository baseline

Run:

```bash
cd /home/opc/src/sub2api
git status --short
git rev-parse HEAD
git log -10 --oneline
```

Record:

- Current branch.
- Current commit SHA.
- Recent commits.
- Any untracked files.

Expected notes:

- Untracked `.hermes/plans/*` files are planning artifacts, not deployed runtime code.
- If source files are modified, stop and ask whether to review dirty state or clean HEAD.

### 1.2 Deployment baseline

Run:

```bash
sudo docker compose -f /opt/sub2api/docker-compose.local.yml ps
sudo docker inspect sub2api --format 'image={{.Config.Image}} image_id={{.Image}} started={{.State.StartedAt}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}'
sudo docker image inspect sub2api:single-user-private-control --format 'id={{.Id}} created={{.Created}}'
```

Record:

- Running containers and health.
- Live image tag.
- Live image ID.
- App start time.

Expected:

```text
sub2api            healthy
sub2api-postgres   healthy
sub2api-redis      healthy
```

### 1.3 Runtime mode flags

Run only this allowlisted environment check:

```bash
sudo docker inspect sub2api --format '{{range .Config.Env}}{{println .}}{{end}}' \
  | grep -E '^(SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE|SUB2API_PUBLIC_HOST|SERVER_PORT|GIN_MODE)='
```

Expected:

- `SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true`, or equivalent enabled value.
- `SUB2API_PUBLIC_HOST=sub2api.research.for-immortal.cn`, or the expected public host.

Do not dump the full environment.

---

## Step 2: Check Public vs Private Access

**Purpose:** Confirm the public internet sees only gateway routes, while private/local access can reach the control plane.

**Artifact to create:**

```text
/home/opc/src/sub2api/.hermes/review-artifacts/http-route-boundary.md
```

### 2.1 Public control-plane routes should be hidden

Run:

```bash
for path in \
  / \
  /login \
  /admin/dashboard \
  /admin/accounts \
  /api/v1/auth/me \
  /api/v1/admin/accounts \
  /api/v1/admin/system/check-updates \
  /api/v1/payment/config; do
  code=$(curl --noproxy '*' -sS -o /tmp/exapi-boundary.txt -w '%{http_code}' "https://sub2api.research.for-immortal.cn${path}" || true)
  printf '%s\t%s\n' "$code" "$path"
done
```

Expected:

```text
404 /
404 /login
404 /admin/dashboard
404 /admin/accounts
404 /api/v1/auth/me
404 /api/v1/admin/accounts
404 /api/v1/admin/system/check-updates
404 /api/v1/payment/config
```

If any route returns `200` publicly, classify as High or Critical depending on what it exposes.

### 2.2 Public gateway without key should require auth

Run:

```bash
curl --noproxy '*' -sS -o /tmp/exapi-models-no-key.json -w 'models_no_key=%{http_code}\n' \
  https://sub2api.research.for-immortal.cn/v1/models
head -c 300 /tmp/exapi-models-no-key.json
```

Expected:

```text
models_no_key=401
API_KEY_REQUIRED
```

### 2.3 Private/local UI should work

Run:

```bash
for path in / /admin/dashboard /admin/accounts /api/v1/auth/me; do
  code=$(curl --noproxy '*' -sS -o /tmp/exapi-local-boundary.txt -w '%{http_code}' "http://127.0.0.1:8027${path}" || true)
  printf '%s\t%s\n' "$code" "$path"
done
```

Expected:

```text
200 /
200 /admin/dashboard
200 /admin/accounts
401 /api/v1/auth/me
```

The `401` for `/api/v1/auth/me` is expected without browser cookies.

---

## Step 3: Confirm Update/Restart Is Disabled

**Purpose:** Confirm the current private deployment cannot update, rollback, or restart itself from the web app.

**Artifact to create:**

```text
/home/opc/src/sub2api/.hermes/review-artifacts/update-removal-review.md
```

### 3.1 Review backend route registration

Read:

```text
/home/opc/src/sub2api/backend/internal/server/routes/admin.go
```

Look for this logic in `registerSystemRoutes`:

```go
system.GET("/version", h.Admin.System.GetVersion)
if singleUserPrivateControlPlaneEnabled() {
    return
}
system.GET("/check-updates", h.Admin.System.CheckUpdates)
system.GET("/rollback-versions", h.Admin.System.GetRollbackVersions)
system.POST("/update", h.Admin.System.PerformUpdate)
system.POST("/rollback", h.Admin.System.Rollback)
system.POST("/restart", h.Admin.System.RestartService)
```

Expected:

- In private single-user mode, only `/version` is registered.
- Update, rollback, and restart routes are not registered.

### 3.2 Probe local update endpoints

Run:

```bash
for spec in \
  'GET /api/v1/admin/system/version' \
  'GET /api/v1/admin/system/check-updates' \
  'GET /api/v1/admin/system/rollback-versions' \
  'POST /api/v1/admin/system/update' \
  'POST /api/v1/admin/system/rollback' \
  'POST /api/v1/admin/system/restart'; do
  method=${spec%% *}
  path=${spec#* }
  code=$(curl --noproxy '*' -sS -X "$method" -o /tmp/exapi-update-review.txt -w '%{http_code}' "http://127.0.0.1:8027${path}" || true)
  printf '%s\t%s\n' "$code" "$spec"
done
```

Expected:

```text
401 GET /api/v1/admin/system/version
404 GET /api/v1/admin/system/check-updates
404 GET /api/v1/admin/system/rollback-versions
404 POST /api/v1/admin/system/update
404 POST /api/v1/admin/system/rollback
404 POST /api/v1/admin/system/restart
```

`/version` may return `401` without auth. That is fine. The important result is that update/rollback/restart routes return `404`.

### 3.3 Review frontend update UI

Read:

```text
/home/opc/src/sub2api/frontend/src/components/layout/AppSidebar.vue
/home/opc/src/sub2api/frontend/src/components/common/VersionBadge.vue
```

Expected in `AppSidebar.vue`:

```vue
<VersionBadge :version="siteVersion" :disable-updates="singleUserPrivateControlPlane" />
```

Expected in `VersionBadge.vue`:

- Full admin dropdown is shown only when `isAdmin && !disableUpdates`.
- `onMounted` does not call `appStore.fetchVersion(false)` when `disableUpdates` is true.
- `handleUpdate`, `handleRollback`, and `handleRestart` return early when updates are disabled.

### 3.4 Browser UI/network check

Open:

```text
http://127.0.0.1:8027/admin/accounts
```

Run in browser console:

```javascript
JSON.stringify({
  textIncludesUpdate: document.body.innerText.includes('已是最新版本') ||
    document.body.innerText.includes('有新版本可用') ||
    document.body.innerText.includes('查看更新') ||
    document.body.innerText.includes('版本回退'),
  updateResources: performance.getEntriesByType('resource')
    .map(e => e.name)
    .filter(u => u.includes('check-updates') ||
      u.includes('/system/update') ||
      u.includes('rollback-versions') ||
      u.includes('/system/restart'))
})
```

Expected:

```json
{"textIncludesUpdate":false,"updateResources":[]}
```

---

## Step 4: Verify Gateway API Key Flow

**Purpose:** Confirm the public gateway requires API keys and works with a valid key.

**Artifact to create:**

```text
/home/opc/src/sub2api/.hermes/review-artifacts/api-key-auth-review.md
```

### 4.1 Missing key should fail

Run:

```bash
curl --noproxy '*' -sS -o /tmp/exapi-no-key.json -w '%{http_code}\n' \
  https://sub2api.research.for-immortal.cn/v1/models
head -c 300 /tmp/exapi-no-key.json
```

Expected:

```text
401
API_KEY_REQUIRED
```

### 4.2 Invalid key should fail

Run:

```bash
curl --noproxy '*' -sS -o /tmp/exapi-invalid-key.json -w '%{http_code}\n' \
  -H 'Authorization: Bearer sk-invalid-review-only' \
  https://sub2api.research.for-immortal.cn/v1/models
head -c 300 /tmp/exapi-invalid-key.json
```

Expected:

```text
401
```

The response must not reveal user IDs, account IDs, or secret details.

### 4.3 Valid key should list models

Use an operator-provided key or securely load the current active key into a variable. Do not print it.

Example pattern:

```bash
KEY="<load securely; do not echo>"
curl --noproxy '*' -sS -o /tmp/exapi-valid-models.json -w 'models_with_key=%{http_code}\n' \
  -H "Authorization: Bearer ${KEY}" \
  https://sub2api.research.for-immortal.cn/v1/models
python3 - <<'PY'
import json
from pathlib import Path
s = Path('/tmp/exapi-valid-models.json').read_text()
data = json.loads(s)
print({'object': data.get('object'), 'model_count': len(data.get('data', []))})
PY
```

Expected:

```text
models_with_key=200
{'object': 'list', 'model_count': <nonzero>}
```

### 4.4 One safe chat-completion smoke test

Use the known working model:

```text
codex-auto-review
```

Run:

```bash
KEY="<load securely; do not echo>"
cat >/tmp/exapi-review-chat.json <<'JSON'
{
  "model": "codex-auto-review",
  "messages": [
    {"role": "user", "content": "Reply with exactly: ExAPI review OK"}
  ],
  "stream": false
}
JSON
curl --noproxy '*' --max-time 90 -sS -o /tmp/exapi-review-chat-result.json -w 'http=%{http_code} time=%{time_total}\n' \
  -H "Authorization: Bearer ${KEY}" \
  -H 'Content-Type: application/json' \
  -d @/tmp/exapi-review-chat.json \
  https://sub2api.research.for-immortal.cn/v1/chat/completions
python3 - <<'PY'
import json
from pathlib import Path
s = Path('/tmp/exapi-review-chat-result.json').read_text()
data = json.loads(s)
choice = (data.get('choices') or [{}])[0]
msg = choice.get('message') or {}
print({
  'object': data.get('object'),
  'model': data.get('model'),
  'finish_reason': choice.get('finish_reason'),
  'content_preview': (msg.get('content') or '')[:200],
})
PY
```

Expected:

- HTTP `200`.
- Model is `codex-auto-review`.
- Response content includes `ExAPI review OK`.

### 4.5 Usage log should update

Run:

```bash
sudo docker exec sub2api-postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off -F $'"'"'\t'"'"' -Atc "
select 'usage_logs_10m', count(*) from usage_logs where created_at > now() - interval '10 minutes';
select id, platform, type, status, schedulable, last_used_at from accounts where deleted_at is null order by id;
"'
```

Expected:

- `usage_logs_10m` is at least `1` after the smoke test.
- OpenAI OAuth account `last_used_at` is recent.

---

## Step 5: Verify the Private UI

**Purpose:** Confirm the private UI is focused on gateway operations and does not expose SaaS/multi-user surfaces.

**Artifact to create:**

```text
/home/opc/src/sub2api/.hermes/review-artifacts/frontend-surface-review.md
```

### 5.1 Check sidebar items

Open:

```text
http://127.0.0.1:8027/admin/accounts
```

Expected sidebar items:

```text
仪表盘
运维监控
账号管理
IP管理
使用记录
API 密钥
系统设置
```

Unexpected items include:

```text
用户管理
分组管理
订单
套餐
兑换码
推广
公告
支付
余额
```

If unexpected items appear, record a finding.

### 5.2 Restricted legacy route should redirect

Open:

```text
http://127.0.0.1:8027/admin/users
```

Expected:

- It should show the single-user cockpit/dashboard area.
- It should not show a user-management table.

### 5.3 Router tests

Run:

```bash
cd /home/opc/src/sub2api
pnpm --dir frontend exec vitest run \
  src/router/__tests__/singleUserGatewayMode.spec.ts \
  src/router/__tests__/guards.spec.ts
```

Expected:

```text
Test Files 2 passed
Tests 37 passed
```

### 5.4 Deployed asset check

Run:

```bash
sudo docker exec sub2api sh -lc 'find /app -path "*/dist/assets/*.js" -type f | sed "s#.*/##" | grep -E "UsersView|GroupsView|PaymentView|RedeemView|PromoCodesView|AdminOrders|SubscriptionsView|Affiliate|AnnouncementsView" || true'
```

Expected:

- Empty output.

If any removed legacy chunk appears, record a Medium finding.

---

## Step 6: Check Secret Handling

**Purpose:** Confirm the review and runtime logs do not expose credentials.

**Artifact to create:**

```text
/home/opc/src/sub2api/.hermes/review-artifacts/secret-handling-review.md
```

### 6.1 Identify secret-bearing schema fields without values

Run:

```bash
sudo docker exec sub2api-postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off -F $'"'"'\t'"'"' -Atc "
select table_name, column_name, data_type
from information_schema.columns
where table_schema = 'public'
  and (
    column_name ilike '%token%' or
    column_name ilike '%secret%' or
    column_name ilike '%credential%' or
    column_name ilike '%cookie%' or
    column_name = 'key'
  )
order by table_name, ordinal_position;
"'
```

Expected:

- Only schema names are printed.
- No actual token/key/cookie values are selected.

### 6.2 Redacted log scan

Run:

```bash
sudo docker logs --since 24h sub2api 2>&1 \
  | grep -Ei 'refresh_token|access_token|authorization:|x-api-key|cookie:|set-cookie:|sk-[A-Za-z0-9_-]{8,}' \
  | sed -E 's/(refresh_token|access_token|authorization|x-api-key|cookie|set-cookie)([^[:space:]]*)/\1[REDACTED]/Ig; s/sk-[A-Za-z0-9_-]{8,}/sk-[REDACTED]/g' \
  | tail -100
```

Expected:

- Ideally empty output.
- If output exists, it must be redacted before being saved.
- Any raw unredacted secret is Critical.

### 6.3 Review source redaction behavior

Read these files:

```text
backend/internal/server/middleware/logger.go
backend/internal/server/middleware/request_logger.go
backend/internal/server/middleware/api_key_auth.go
backend/internal/handler/openai_gateway_handler.go
```

Checklist:

- Logs include IDs, not raw API keys.
- Logs do not dump full request headers or bodies by default.
- Upstream auth headers are not logged.
- API key middleware stores key metadata, not raw key, in request context where possible.

---

## Step 7: Check Docker and Port Exposure

**Purpose:** Confirm the deployment shape matches private control-plane expectations.

**Artifact to create:**

```text
/home/opc/src/sub2api/.hermes/review-artifacts/deployment-review.md
```

### 7.1 Compose structure

Run:

```bash
sudo grep -nE '^[[:space:]]*(image:|container_name:|ports:|env_file:|environment:|networks:|volumes:|healthcheck:)' /opt/sub2api/docker-compose.local.yml
```

Checklist:

- App binds to intended local/WireGuard addresses.
- App is not bound to public `0.0.0.0` unless explicitly intended and protected upstream.
- Postgres and Redis ports are not publicly published.
- Healthchecks exist.
- Persistent volumes exist.
- Private mode and public host environment flags are configured.

### 7.2 Runtime listening ports

Run:

```bash
sudo docker compose -f /opt/sub2api/docker-compose.local.yml ps
sudo ss -ltnp | grep -E ':(8027|5432|6379)\b' || true
```

Expected:

- App is on `127.0.0.1:8027` and/or `100.97.17.1:8027`.
- Postgres/Redis are not exposed on public interfaces.

### 7.3 Dockerfile review

Read:

```text
/home/opc/src/sub2api/Dockerfile
/home/opc/src/sub2api/.dockerignore
```

Checklist:

- Frontend is built and embedded.
- Final image runs as non-root where intended.
- `.env` and local configs are excluded by `.dockerignore`.
- Healthcheck targets `/health`.
- Postgres client tools match database major version.

---

## Step 8: Build and Test Validation

**Purpose:** Confirm the reviewed version can be built and key targeted tests pass.

**Artifact to create:**

```text
/home/opc/src/sub2api/.hermes/review-artifacts/test-validation-review.md
```

### 8.1 Check toolchain

Run:

```bash
cd /home/opc/src/sub2api/backend
go version
awk '/^go / {print}' go.mod
```

Expected:

- If local Go is older than `go.mod`, record that local Go tests are blocked.
- Use Docker build as backend compile verification in that case.

### 8.2 Run targeted frontend tests

Run:

```bash
cd /home/opc/src/sub2api
pnpm --dir frontend exec vitest run \
  src/router/__tests__/singleUserGatewayMode.spec.ts \
  src/router/__tests__/guards.spec.ts
```

Expected:

```text
2 test files passed
37 tests passed
```

### 8.3 Run frontend production build

Run:

```bash
cd /home/opc/src/sub2api
pnpm --dir frontend run build
```

Expected:

- `vue-tsc -b` passes.
- `vite build` passes.
- Existing chunk-size or browserslist warnings can be recorded as non-blocking if unchanged.

### 8.4 Run Docker production build

Run:

```bash
cd /home/opc/src/sub2api
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
sudo docker build \
  --build-arg COMMIT="$COMMIT-review" \
  --build-arg DATE="$DATE" \
  -t sub2api:review-build \
  .
```

Expected:

- Full Docker build succeeds.
- This verifies frontend build and backend Go compile using the Dockerfile toolchain.

### 8.5 Avoid misleading full-suite failures

Do not treat unrelated full Vitest failures as findings unless reproduced by a targeted test. Known unrelated categories may include:

- Pinia test setup errors in unrelated suites.
- jsdom canvas `getContext` not implemented for QR tests.
- Existing component selector fixture mismatches.

Record these as test debt only if they block review confidence.

---

## Step 9: Write the Final Report

**Purpose:** Turn the evidence into a readable review report.

**File to create:**

```text
/home/opc/src/sub2api/.hermes/review-artifacts/YYYY-MM-DD_exapi-application-review.md
```

Use this template:

```markdown
# ExAPI Application Review Report

## Executive Summary

- Reviewed version:
- Live image:
- Overall status:
- Highest severity finding:

## Scope

## Baseline

## Findings

| ID | Severity | Title | Status |
|----|----------|-------|--------|
| F-001 | Info | Public control plane hidden | Passed |

## Finding Details

### F-001: Example title

**Severity:** Info / Low / Medium / High / Critical
**Status:** Open / Fixed / Passed / Accepted
**Evidence:**
**Reproduction:**
**Impact:**
**Recommendation:**
**Files:**

## Controls Verified

- Public control plane hidden.
- Gateway requires API key.
- Update/rollback/restart routes disabled.
- Private UI does not show update dropdown.
- App/Postgres/Redis healthy.

## Commands Run

## Risks and Open Questions

## Appendix: Redacted Evidence
```

### Severity guide

- **Critical:** Public unauthenticated control-plane access, credential leak, RCE, API key auth bypass.
- **High:** Authenticated local path can unexpectedly mutate deployment, public route exposure, serious authz bug.
- **Medium:** Private-only issue with meaningful security or operations impact.
- **Low:** Confusing UI, minor hardening, noisy non-secret logs.
- **Info:** Verified behavior or non-actionable observation.

---

## Optional: Split Review Across Subagents

If using subagents, divide the work like this:

### Reviewer A: Route Boundary and Update Removal

Covers:

- Step 2
- Step 3

Returns:

```markdown
## Summary
## Commands run
## Evidence
## Findings
## Non-findings
## Open questions
```

### Reviewer B: Gateway and API Key Flow

Covers:

- Step 4
- OpenAI/account usage parts of Step 8 if needed

Returns the same summary format.

### Reviewer C: Frontend Surface

Covers:

- Step 5
- Relevant frontend tests in Step 8

Returns the same summary format.

### Reviewer D: Secrets and Deployment

Covers:

- Step 6
- Step 7

Returns the same summary format.

### Reviewer E: Build, Logs, and Final Report

Covers:

- Step 8
- Step 9

Returns the same summary format.

---

## What Counts as Done

The review is complete when:

- [ ] The final report exists under `.hermes/review-artifacts/`.
- [ ] The report names the exact reviewed commit and image ID.
- [ ] Public control-plane routes were checked.
- [ ] Gateway API key behavior was checked.
- [ ] Update/rollback/restart removal was checked.
- [ ] Private UI route pruning was checked.
- [ ] Deployment port exposure was checked.
- [ ] Logs were scanned for recent fatal errors and raw secrets.
- [ ] Findings have severity, evidence, impact, and recommendation.
- [ ] No raw secrets appear in artifacts.
- [ ] No code/deployment changes were made during review.

---

## If Findings Need Fixes

Do not fix issues inside this review plan. Instead, create a separate remediation plan:

```text
/home/opc/src/sub2api/.hermes/plans/YYYY-MM-DD_HHMMSS-exapi-review-remediation.md
```

Each remediation task should include:

- Exact file path.
- Failing regression test.
- Minimal implementation.
- Verification command.
- Suggested commit message.

Example remediation task:

```markdown
### Task: Add private-mode regression test for updater routes

**File:** `backend/internal/server/routes/admin_private_mode_test.go`

**Test:** Set `SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true`, register admin system routes, and assert `/check-updates`, `/update`, `/rollback`, and `/restart` return 404.

**Command:** `go test ./internal/server/routes -run PrivateModeSystemRoutes -v`

**Commit:** `test: cover private mode updater route removal`
```
