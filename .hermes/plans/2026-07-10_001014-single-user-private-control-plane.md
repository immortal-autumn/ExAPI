# Single-User Private Control Plane Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Convert the current Sub2API deployment/fork into a single-user gateway where the public Internet exposes only AI API endpoints, while the web control panel and all control/admin/user APIs are available only from localhost and the WireGuard network; then improve single-user usability with Cockpit Tools-inspired quota monitoring, wakeup tasks, local integration, and multi-account management.

**Architecture:** Enforce public/private separation at the edge first with nginx allowlists, then add app-level safeguards so accidental proxy exposure cannot publish the control plane. Keep Sub2API as the server-side AI gateway and borrow Cockpit Tools heuristics only as product patterns: dashboard-first single-operator UX, quota cards, wakeup schedules, local/WireGuard convenience, and multi-account quick switching/health surfaces.

**Tech Stack:** Sub2API Go 1.26/Gin backend, Vue 3/Pinia frontend, PostgreSQL, Redis, Docker Compose, nginx, WireGuard; Cockpit Tools design reference from React/Tauri/Rust but no code copying due license and architecture mismatch.

---

## Requirements Restatement

1. Single-user operation:
   - Treat the deployment as one operator/admin, not a public multi-user portal.
   - Avoid exposing registration/login/control-panel UX publicly.
2. Public Internet exposure:
   - Public domain should expose only AI-compatible API endpoints, e.g. `/v1`, `/v1beta`, `/responses`, `/chat/completions`, `/embeddings`, `/images`, `/videos`, `/backend-api/codex`, `/antigravity` as needed.
   - Public should not expose SPA shell, `/login`, `/admin`, `/dashboard`, `/api/v1/admin`, `/api/v1/user`, `/api/v1/auth/*`, setup, payment, docs/static control-panel routes, or local admin bypass.
3. Private control panel exposure:
   - Control panel should be reachable from:
     - `http://127.0.0.1:8027/`
     - `http://100.97.17.1:8027/`
   - Both should support the local/WireGuard admin bypass already implemented.
4. Cockpit Tools-inspired usability:
   - Better quota monitoring for upstream accounts/models.
   - Wakeup tasks for configured accounts/models.
   - Local integration shortcuts/status for localhost/WireGuard use.
   - Multi-account management optimized for one operator.
5. Security constraints:
   - Do not weaken public API-key auth.
   - Do not expose admin JWT/local admin bypass publicly.
   - Do not copy Cockpit Tools code because Cockpit Tools is CC BY-NC-SA 4.0 and Sub2API is LGPL-3.0.

---

## Current Context

- Current source: `/home/opc/src/sub2api`, branch `feat/local-admin-bypass`, latest known commit `9aa8adc`.
- Current deployment: `/opt/sub2api/docker-compose.local.yml`.
- Current app ports:
  - `127.0.0.1:8027 -> 8080`
  - `100.97.17.1:8027 -> 8080`
- Current public nginx vhost: `/etc/nginx/conf.d/sub2api.research.for-immortal.cn.conf` proxies all `/` to `http://127.0.0.1:8027`.
- Current public behavior is therefore too broad: it exposes the SPA/login/control panel shell, even though local admin bypass itself is denied publicly.
- Gateway routes from `backend/internal/server/routes/gateway.go` include:
  - `/v1/messages`
  - `/v1/messages/count_tokens`
  - `/v1/models`
  - `/v1/usage`
  - `/v1/responses`
  - `/v1/chat/completions`
  - `/v1/embeddings`
  - `/v1/images/...`
  - `/v1/videos/...`
  - `/v1beta/...` Gemini native compatibility
  - `/responses`, `/chat/completions`, `/embeddings`, `/images/...`, `/videos/...` unprefixed aliases
  - `/backend-api/codex/...`
  - `/antigravity/...`
- Auth/control routes from `backend/internal/server/routes/auth.go`, `admin.go`, and `user.go` live under `/api/v1/...` and should not be public except possibly non-sensitive health or client metadata if explicitly allowed.

---

## Phase 1: Edge privacy boundary — public nginx exposes only AI endpoints

### Task 1: Back up and inspect the current nginx vhost

**Objective:** Capture the current public proxy config before changing public exposure.

**Files:**
- Read: `/etc/nginx/conf.d/sub2api.research.for-immortal.cn.conf`
- Backup: `/etc/nginx/conf.d/sub2api.research.for-immortal.cn.conf.bak.<timestamp>`

**Step 1: Read current config**

```bash
sudo sed -n '1,220p' /etc/nginx/conf.d/sub2api.research.for-immortal.cn.conf
```

Expected: see `location / { proxy_pass http://127.0.0.1:8027; ... }`.

**Step 2: Create backup**

```bash
sudo cp -a /etc/nginx/conf.d/sub2api.research.for-immortal.cn.conf \
  /etc/nginx/conf.d/sub2api.research.for-immortal.cn.conf.bak.$(date -u +%Y%m%dT%H%M%SZ)
```

Expected: backup file exists.

**Step 3: Commit?**

No git commit; this is live server config. Record backup name in deployment notes instead.

---

### Task 2: Replace public catch-all proxy with explicit AI endpoint allowlist

**Objective:** Make public `https://sub2api.research.for-immortal.cn/` return 404/403 for control panel paths while preserving AI API endpoints.

**Files:**
- Modify: `/etc/nginx/conf.d/sub2api.research.for-immortal.cn.conf`

**Step 1: Replace HTTPS server block locations**

Use this location strategy inside the `listen 8443 ssl` server block.

```nginx
# Only AI gateway endpoints are public.
# Control panel, auth UI, admin APIs, user APIs, setup, payment, and SPA shell are intentionally private.

location = /health {
    proxy_pass http://127.0.0.1:8027;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

location ^~ /v1/ {
    proxy_pass http://127.0.0.1:8027;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}

location ^~ /v1beta/ {
    proxy_pass http://127.0.0.1:8027;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}

location ^~ /backend-api/codex/ {
    proxy_pass http://127.0.0.1:8027;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}

location ^~ /antigravity/ {
    proxy_pass http://127.0.0.1:8027;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}

# Unprefixed compatibility endpoints used by some clients.
location ~ ^/(responses|chat/completions|embeddings|images|videos)(/|$) {
    proxy_pass http://127.0.0.1:8027;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}

# Everything else is not public.
location / {
    return 404;
}
```

**Important:** If `$connection_upgrade` is not globally defined in nginx, either add this map at HTTP scope or replace `proxy_set_header Connection $connection_upgrade;` with:

```nginx
proxy_set_header Connection "upgrade";
```

Prefer checking existing nginx global config first:

```bash
sudo nginx -T | grep -n "connection_upgrade\|map \$http_upgrade"
```

**Step 2: Test nginx config**

```bash
sudo nginx -t
```

Expected: syntax ok.

**Step 3: Reload nginx**

```bash
sudo systemctl reload nginx
```

Expected: reload succeeds.

**Step 4: Verify public control panel is hidden**

```bash
for path in / /home /login /admin/dashboard /dashboard /api/v1/auth/login /api/v1/admin/dashboard; do
  printf '%-35s ' "$path"
  curl -sS -o /dev/null -w '%{http_code} %{content_type}\n' \
    "https://sub2api.research.for-immortal.cn${path}"
done
```

Expected: `404` for all listed public control paths.

**Step 5: Verify public gateway paths still reach the app**

Use unauthenticated probes; expected may be `401`/`403`, not `404`, because API key auth should reject missing credentials.

```bash
for path in /v1/models /v1/messages /v1/chat/completions /v1beta/models /backend-api/codex/models /antigravity/models; do
  printf '%-35s ' "$path"
  curl -sS -o /dev/null -w '%{http_code} %{content_type}\n' \
    "https://sub2api.research.for-immortal.cn${path}"
done
```

Expected:
- `/v1/models`: likely `401`/`403` due missing API key.
- POST-only endpoints may return `404`/`405` if probed with GET; use POST-specific probes later.
- The key point: no HTML control panel should be returned.

---

### Task 3: Verify private control panel remains available via localhost and WireGuard

**Objective:** Prove nginx public lockdown does not affect private direct control panel access.

**Files:**
- No source edits.

**Step 1: Browser verify local/WireGuard root**

Use browser navigation:

```text
http://100.97.17.1:8027/
```

Expected:

```text
Admin Dashboard - Sub2API
```

**Step 2: Curl private health**

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8027/health
curl -sS -o /dev/null -w '%{http_code}\n' http://100.97.17.1:8027/health
```

Expected: both `200`.

**Step 3: Verify from precision**

```bash
ssh -o BatchMode=yes -o ConnectTimeout=8 precision \
  'curl -sS -o /dev/null -w "%{http_code}\n" --connect-timeout 5 --max-time 10 http://100.97.17.1:8027/'
```

Expected: `200`.

---

## Phase 2: App-level single-user privacy safeguards

### Task 4: Add a server-side public-route gate feature flag

**Objective:** Defense in depth: even if nginx is misconfigured, Sub2API can reject public-host control-panel/API paths when single-user private-control mode is enabled.

**Files:**
- Modify: `backend/internal/server/middleware/...` create `public_control_plane_guard.go`
- Modify: `backend/internal/server/router.go`
- Test: `backend/internal/server/middleware/public_control_plane_guard_test.go`

**Step 1: Write failing tests**

Create `backend/internal/server/middleware/public_control_plane_guard_test.go`:

```go
package middleware

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/require"
)

func TestPublicControlPlaneGuardAllowsGatewayPaths(t *testing.T) {
    gin.SetMode(gin.TestMode)
    t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
    t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

    r := gin.New()
    r.Use(PublicControlPlaneGuard())
    r.GET("/v1/models", func(c *gin.Context) { c.Status(http.StatusNoContent) })

    req := httptest.NewRequest(http.MethodGet, "https://sub2api.research.for-immortal.cn/v1/models", nil)
    req.Host = "sub2api.research.for-immortal.cn"
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    require.Equal(t, http.StatusNoContent, w.Code)
}

func TestPublicControlPlaneGuardBlocksControlPanel(t *testing.T) {
    gin.SetMode(gin.TestMode)
    t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
    t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

    r := gin.New()
    r.Use(PublicControlPlaneGuard())
    r.GET("/admin/dashboard", func(c *gin.Context) { c.Status(http.StatusNoContent) })

    req := httptest.NewRequest(http.MethodGet, "https://sub2api.research.for-immortal.cn/admin/dashboard", nil)
    req.Host = "sub2api.research.for-immortal.cn"
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    require.Equal(t, http.StatusNotFound, w.Code)
}

func TestPublicControlPlaneGuardAllowsWireGuardControlPanel(t *testing.T) {
    gin.SetMode(gin.TestMode)
    t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
    t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

    r := gin.New()
    r.Use(PublicControlPlaneGuard())
    r.GET("/admin/dashboard", func(c *gin.Context) { c.Status(http.StatusNoContent) })

    req := httptest.NewRequest(http.MethodGet, "http://100.97.17.1:8027/admin/dashboard", nil)
    req.Host = "100.97.17.1:8027"
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    require.Equal(t, http.StatusNoContent, w.Code)
}
```

Run:

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test -tags unit ./internal/server/middleware -run PublicControlPlaneGuard -count=1
```

Expected before implementation: FAIL.

**Step 2: Implement middleware**

Create `backend/internal/server/middleware/public_control_plane_guard.go`:

```go
package middleware

import (
    "net"
    "net/http"
    "os"
    "strings"

    "github.com/gin-gonic/gin"
)

func PublicControlPlaneGuard() gin.HandlerFunc {
    return func(c *gin.Context) {
        if !singleUserPrivateControlPlaneEnabled() {
            c.Next()
            return
        }
        if !isPublicHost(c.Request.Host) {
            c.Next()
            return
        }
        if isPublicGatewayPath(c.Request.URL.Path) {
            c.Next()
            return
        }
        c.AbortWithStatus(http.StatusNotFound)
    }
}

func singleUserPrivateControlPlaneEnabled() bool {
    switch strings.ToLower(strings.TrimSpace(os.Getenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE"))) {
    case "1", "true", "yes", "on":
        return true
    default:
        return false
    }
}

func isPublicHost(rawHost string) bool {
    host := rawHost
    if h, _, err := net.SplitHostPort(rawHost); err == nil {
        host = h
    }
    host = strings.ToLower(strings.Trim(host, "[] "))
    publicHost := strings.ToLower(strings.TrimSpace(os.Getenv("SUB2API_PUBLIC_HOST")))
    if publicHost == "" {
        return false
    }
    return host == publicHost
}

func isPublicGatewayPath(path string) bool {
    if path == "/health" {
        return true
    }
    prefixes := []string{
        "/v1/",
        "/v1beta/",
        "/backend-api/codex/",
        "/antigravity/",
        "/responses",
        "/chat/completions",
        "/embeddings",
        "/images/",
        "/videos/",
    }
    for _, prefix := range prefixes {
        if path == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(path, prefix) {
            return true
        }
    }
    return false
}
```

**Step 3: Install middleware early**

Modify `backend/internal/server/router.go` after security headers and before frontend middleware/routes:

```go
r.Use(middleware2.PublicControlPlaneGuard())
```

Use existing alias `middleware2`.

**Step 4: Run tests**

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test -tags unit ./internal/server/middleware -run PublicControlPlaneGuard -count=1
GOTOOLCHAIN=auto go test -tags unit ./internal/server/routes ./internal/server/middleware -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
cd /home/opc/src/sub2api
git add backend/internal/server/middleware/public_control_plane_guard.go backend/internal/server/middleware/public_control_plane_guard_test.go backend/internal/server/router.go
git commit -m "feat: gate public control plane in single-user mode"
```

---

### Task 5: Pass single-user private-control env vars through Compose

**Objective:** Enable app-level guard in the live deployment.

**Files:**
- Modify: `/opt/sub2api/docker-compose.local.yml`
- Modify: `/opt/sub2api/.env`
- Later mirror in source deployment template if desired: `deploy/docker-compose.local.yml`

**Step 1: Add environment passthrough**

In `/opt/sub2api/docker-compose.local.yml`, under service `environment:`, add:

```yaml
      - SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=${SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE:-false}
      - SUB2API_PUBLIC_HOST=${SUB2API_PUBLIC_HOST:-}
```

**Step 2: Add live env values**

In `/opt/sub2api/.env`, add:

```dotenv
SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true
SUB2API_PUBLIC_HOST=sub2api.research.for-immortal.cn
```

**Step 3: Restart and verify env inside container**

```bash
cd /opt/sub2api
sudo docker compose -f docker-compose.local.yml up -d --force-recreate sub2api
sudo docker exec sub2api env | grep -E '^SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=|^SUB2API_PUBLIC_HOST='
```

Expected:

```text
SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true
SUB2API_PUBLIC_HOST=sub2api.research.for-immortal.cn
```

---

## Phase 3: Single-user UX simplification

### Task 6: Add a “single-user mode” frontend flag and simplify navigation

**Objective:** Hide public/multi-user UX affordances when single-user mode is enabled: registration, user subscription/payment pages, generic user dashboard, public home/index.

**Files:**
- Modify: `frontend/src/stores/app.ts` or public settings handling after inspection
- Modify: `frontend/src/router/index.ts`
- Modify: relevant navigation component, likely `frontend/src/components/layout/Sidebar.vue` or equivalent after inspection
- Test: router/navigation tests where available

**Step 1: Expose safe public setting**

Add a safe public settings field from backend settings injection or app config:

```ts
single_user_mode_enabled: boolean
```

If backend public settings system already exists, add field there. Do not expose secrets, CIDRs, admin email, or bypass token behavior.

**Step 2: Route guard behavior**

When `single_user_mode_enabled` is true:

- `/` and `/home` continue to redirect to `/admin/dashboard`.
- `/register`, `/forgot-password`, `/reset-password`, public payment/subscription UX should redirect to `/login` or return not found unless explicitly needed.
- Existing authenticated admin continues to reach `/admin/dashboard`.
- Local/WireGuard bypass still works.

**Step 3: Hide navigation entries**

Hide or demote user-facing entries:

- Sign up/register link.
- My Subscriptions.
- Redeem if not needed.
- Payment/order pages if not needed.
- Affiliate/promo public user features.

Keep admin entries relevant to single operator:

- Dashboard.
- Accounts.
- Channels.
- Groups, if groups remain useful for API-key routing.
- Usage.
- Ops.
- Settings.
- API Keys.

**Step 4: Test**

```bash
cd /home/opc/src/sub2api/frontend
pnpm test:run
pnpm run build
```

Expected: PASS.

**Step 5: Commit**

```bash
cd /home/opc/src/sub2api
git add frontend/src backend/internal
git commit -m "feat: simplify frontend for single-user mode"
```

---

## Phase 4: Cockpit Tools-inspired quota monitoring

### Task 7: Inventory existing quota/usage services before adding UI

**Objective:** Reuse existing Sub2API quota/usage data instead of adding duplicate mechanisms.

**Files to inspect:**
- `backend/internal/service/*quota*.go`
- `backend/internal/service/usage*.go`
- `backend/internal/handler/admin/*` or existing admin ops handlers
- `frontend/src/views/admin/DashboardView.vue`
- `frontend/src/views/admin/ops/OpsDashboard.vue`

**Step 1: Search quota surfaces**

```bash
cd /home/opc/src/sub2api
grep -R "quota\|Quota\|usage\|Usage" -n backend/internal/service backend/internal/handler frontend/src/views/admin | head -200
```

**Step 2: Decide source of truth**

Prefer existing admin ops endpoints if they already report:

- per-account availability,
- per-platform quota,
- model quota/reset times,
- recent usage,
- errors/rate limits.

Avoid creating new tables unless current data is insufficient.

---

### Task 8: Add single-operator quota cards to admin dashboard

**Objective:** Mimic Cockpit Tools dashboard heuristic: show per-platform/account quota cards with remaining quota, reset time, and quick refresh.

**Files:**
- Backend endpoint likely existing or new under `backend/internal/server/routes/admin.go` ops/dashboard area.
- Frontend: `frontend/src/views/admin/DashboardView.vue` or a new component under `frontend/src/views/admin/components/QuotaOverviewCards.vue`.

**Step 1: Write backend test for quota summary endpoint if new**

If no existing endpoint fits, add:

```http
GET /api/v1/admin/single-user/quota-summary
```

Response shape:

```json
{
  "accounts": [
    {
      "account_id": 123,
      "name": "openai-main",
      "platform": "openai",
      "status": "available",
      "quota": {
        "hourly_remaining": 10,
        "weekly_remaining": 200,
        "reset_at": "2026-07-10T01:00:00Z"
      },
      "last_checked_at": "2026-07-10T00:00:00Z"
    }
  ]
}
```

Use nullable fields where providers differ.

**Step 2: Implement minimal aggregator**

Create or extend a service method that gathers existing quota caches/status.

Do not perform slow live upstream calls in dashboard load; use cached data plus a manual refresh button.

**Step 3: Add frontend cards**

Create `frontend/src/views/admin/components/QuotaOverviewCards.vue`:

- grouped by platform,
- status chip,
- remaining quota/reset time,
- refresh button,
- last checked time,
- degraded state if quota unavailable.

**Step 4: Verify**

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test -tags unit ./internal/service ./internal/handler -run QuotaSummary -count=1

cd /home/opc/src/sub2api/frontend
pnpm run build
```

**Step 5: Commit**

```bash
git add backend/internal frontend/src/views/admin
git commit -m "feat: add single-user quota overview"
```

---

## Phase 5: Cockpit Tools-inspired wakeup tasks

### Task 9: Reuse or add scheduled test/wakeup primitives

**Objective:** Provide single-operator wakeup tasks that periodically send lightweight model/API probes through selected accounts/groups to keep quota/account state fresh.

**Files to inspect first:**
- `backend/internal/service/scheduled_test_service.go`
- `backend/internal/service/scheduled_test_*`
- `backend/internal/server/routes/admin.go` scheduled test routes
- `frontend/src/views/admin/*Scheduled*` or settings/ops pages

**Step 1: Inspect existing scheduled test feature**

```bash
cd /home/opc/src/sub2api
grep -R "ScheduledTest\|scheduled test\|定时测试\|Wakeup\|wakeup" -n backend/internal frontend/src | head -200
```

**Step 2: Prefer renaming/presenting existing scheduled tests as wakeup tasks**

If scheduled tests already perform lightweight gateway calls, add a single-user “Wakeup Tasks” UI wrapper rather than duplicating scheduler code.

**Step 3: Add wakeup task fields only if needed**

Fields:

```text
name
platform/account/group target
model
prompt template or fixed minimal prompt
time window / cron
max runs per day
enabled
last run status
last run cost/tokens
```

**Step 4: Safety defaults**

- Disabled by default.
- Low frequency.
- Token cap.
- No public endpoint.
- Admin-only/private-control panel only.

**Step 5: Tests**

Backend tests should verify:

- disabled task does not run,
- token cap is enforced,
- failure status is recorded,
- task cannot be created by non-admin.

Commands:

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test -tags unit ./internal/service -run 'Scheduled|Wakeup' -count=1
```

**Step 6: Commit**

```bash
git add backend/internal frontend/src/views/admin
git commit -m "feat: add single-user wakeup tasks"
```

---

## Phase 6: Local integration shortcuts

### Task 10: Add a private-only local integration panel

**Objective:** Make the private control panel more useful from localhost/WireGuard by showing copyable endpoint/API-key snippets and local health.

**Files:**
- Frontend: create `frontend/src/views/admin/components/LocalIntegrationPanel.vue`
- Frontend: include in `frontend/src/views/admin/DashboardView.vue` or Ops dashboard
- Backend: optional endpoint for safe base URLs/capabilities under admin auth only

**Panel should show:**

- Public AI endpoint:

```text
https://sub2api.research.for-immortal.cn/v1
```

- WireGuard admin/control URL:

```text
http://100.97.17.1:8027/
```

- Local server URL:

```text
http://127.0.0.1:8027/
```

- Example OpenAI-compatible client config:

```text
base_url = https://sub2api.research.for-immortal.cn/v1
api_key = <copy from API Keys page>
```

- Health indicators for `/health`, DB, Redis if existing admin health endpoint can provide them.

**Security rule:** Never display admin bypass tokens because there are none; never display upstream account tokens.

**Tests:**

```bash
cd /home/opc/src/sub2api/frontend
pnpm run build
```

**Commit:**

```bash
git add frontend/src/views/admin
git commit -m "feat: add local integration panel"
```

---

## Phase 7: Multi-account management for one operator

### Task 11: Add Cockpit-style account grouping and quick actions if missing

**Objective:** Improve existing admin account management so a single operator can quickly see and act on multiple upstream accounts.

**Files to inspect:**
- `frontend/src/views/admin/AccountsView.vue`
- `backend/internal/server/routes/admin.go` account routes
- `backend/internal/service/*account*.go`

**Heuristics borrowed from Cockpit Tools:**

- Platform tabs or grouped account cards.
- Status: available, rate-limited, disabled, error, quota unknown.
- Tags/groups for accounts.
- Quick actions:
  - refresh quota/status,
  - enable/disable,
  - test account,
  - set priority/group,
  - view recent errors,
  - copy safe client config.
- Bulk actions only where safe.

**Implementation steps:**

1. Inventory current account list API and UI.
2. Add missing backend fields to account list response, not new endpoints if avoidable.
3. Add platform grouping UI.
4. Add manual refresh/test buttons using existing admin test endpoints.
5. Add clear empty/degraded states.

**Tests:**

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test -tags unit ./internal/handler ./internal/service -run 'Account|Quota|Schedule' -count=1

cd /home/opc/src/sub2api/frontend
pnpm run build
```

**Commit:**

```bash
git add backend/internal frontend/src/views/admin/AccountsView.vue
git commit -m "feat: improve single-user account management"
```

---

## Phase 8: Deployment and verification

### Task 12: Build custom image

**Objective:** Build the updated Sub2API image with private-control/single-user improvements.

**Command:**

```bash
cd /home/opc/src/sub2api
sudo docker build -t sub2api:single-user-private-control .
```

Expected: build succeeds.

**Optional:** keep `sub2api:local-admin-bypass` tag during transition or retag after validation.

---

### Task 13: Update live Compose image and env

**Objective:** Deploy the new image with explicit private-control flags.

**Files:**
- Modify: `/opt/sub2api/docker-compose.local.yml`
- Modify: `/opt/sub2api/.env`

**Environment values:**

```dotenv
SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true
SUB2API_PUBLIC_HOST=sub2api.research.for-immortal.cn
SUB2API_LOCAL_ADMIN_BYPASS=true
SUB2API_LOCAL_ADMIN_BYPASS_CIDRS=100.97.17.0/24
WIREGUARD_BIND_HOST=100.97.17.1
```

**Compose image:**

```yaml
image: sub2api:single-user-private-control
```

**Restart:**

```bash
cd /opt/sub2api
sudo docker compose -f docker-compose.local.yml up -d --force-recreate sub2api
```

Expected: `sub2api` healthy.

---

### Task 14: Final verification matrix

**Objective:** Prove public-only-AI and private-only-control behavior.

**Public control paths should be hidden:**

```bash
for path in / /home /login /admin/dashboard /dashboard /api/v1/auth/login /api/v1/admin/dashboard /api/v1/user/profile; do
  printf '%-35s ' "$path"
  curl -sS -o /dev/null -w '%{http_code} %{content_type}\n' \
    "https://sub2api.research.for-immortal.cn${path}"
done
```

Expected: `404` for all.

**Public AI endpoints should be reachable but require API key:**

```bash
curl -sS -o /dev/null -w '%{http_code}\n' https://sub2api.research.for-immortal.cn/v1/models
curl -sS -o /dev/null -w '%{http_code}\n' https://sub2api.research.for-immortal.cn/v1beta/models
```

Expected: auth-shaped error, usually `401`/`403`, not SPA HTML.

**Private panel should work:**

Browser:

```text
http://100.97.17.1:8027/
```

Expected: admin dashboard.

Curl:

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8027/health
curl -sS -o /dev/null -w '%{http_code}\n' http://100.97.17.1:8027/health
```

Expected: both `200`.

From `precision`:

```bash
ssh -o BatchMode=yes -o ConnectTimeout=8 precision \
  'curl -sS -o /dev/null -w "%{http_code}\n" --connect-timeout 5 --max-time 10 http://100.97.17.1:8027/'
```

Expected: `200`.

**Public local-admin bypass must remain denied:**

```bash
curl -sS -o /tmp/sub2api_public_bypass.json \
  -w 'status=%{http_code} type=%{content_type}\n' \
  -X POST https://sub2api.research.for-immortal.cn/api/v1/auth/local-admin
```

Expected: `404` after nginx/app gate, or `403` if app sees it. Prefer `404` publicly.

---

## Files Likely to Change

Source code:

- `backend/internal/server/middleware/public_control_plane_guard.go`
- `backend/internal/server/middleware/public_control_plane_guard_test.go`
- `backend/internal/server/router.go`
- `backend/internal/handler/auth_handler.go` only if bypass messages/host rules need refinement
- `backend/internal/handler/auth_handler_local_admin_test.go`
- `frontend/src/router/index.ts`
- `frontend/src/views/admin/DashboardView.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/ops/OpsDashboard.vue`
- New frontend components under `frontend/src/views/admin/components/`
- Potential service/handler files for quota summary or wakeup tasks after inventory

Deployment:

- `/etc/nginx/conf.d/sub2api.research.for-immortal.cn.conf`
- `/opt/sub2api/docker-compose.local.yml`
- `/opt/sub2api/.env`

Docs:

- `docs/SINGLE_USER_PRIVATE_CONTROL_PLANE.md`
- `README.md` small link only

---

## Risks and Tradeoffs

1. **Nginx allowlist can accidentally omit a needed API endpoint.**
   - Mitigation: start with known gateway route prefixes from `gateway.go`; verify real clients after deployment.
2. **Some clients may use unprefixed compatibility routes.**
   - Mitigation: include `/responses`, `/chat/completions`, `/embeddings`, `/images`, `/videos` in public allowlist.
3. **SPA is no longer public.**
   - Intended. Public `/` returning 404 may surprise humans; document public API endpoint separately.
4. **App-level guard path list can drift from nginx path list.**
   - Mitigation: add tests and keep route list in one helper; document both.
5. **Wakeup tasks can burn quota.**
   - Mitigation: disabled by default, low token cap, max runs/day, clear cost display.
6. **Cockpit Tools inspiration should not become code copying.**
   - Mitigation: copy product patterns only, not source code.
7. **Single-user mode may conflict with existing payment/subscription features.**
   - Mitigation: hide/deprioritize rather than delete database/model logic.

---

## Open Questions

1. Which public AI endpoint prefixes must remain supported for your clients beyond `/v1`, `/v1beta`, `/responses`, `/chat/completions`, `/embeddings`, `/images`, `/videos`, `/backend-api/codex`, and `/antigravity`?
2. Should public `/health` remain exposed, or should health also be private-only?
3. Should public root `/` return `404`, `403`, or a tiny JSON/plain-text message like `Sub2API gateway only`?
4. For wakeup tasks, which platforms/accounts should be supported first: OpenAI/Codex, Claude, Gemini, Antigravity, Grok?
5. Should registration/payment/subscription routes be disabled at backend level in single-user mode, or only hidden/unreachable via nginx/private frontend?

---

## Recommended Execution Order

1. Phase 1 nginx public/private split.
2. Verify real clients still work against public AI endpoints.
3. Phase 2 app-level guard.
4. Phase 3 single-user frontend simplification.
5. Phase 4 quota dashboard improvements.
6. Phase 5 wakeup tasks.
7. Phase 6 local integration panel.
8. Phase 7 multi-account UI polish.

Do not start Phases 4–7 until the public/private security boundary is verified in production.
