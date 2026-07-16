# Cockpit Tools + Sub2API Integration Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Make Cockpit Tools and the current Sub2API fork/deployment work together cleanly by documenting and optionally adding a Sub2API target/profile workflow for local/WireGuard admin/API access, without weakening public-domain authentication.

**Architecture:** Keep Sub2API as the server-side gateway/admin platform and Cockpit Tools as the local desktop account/IDE cockpit. The first implementation phase should be documentation/configuration only; optional later phases can add Cockpit-side profile support for Sub2API OpenAI-compatible base URLs and Sub2API-side metadata endpoints if they are genuinely needed.

**Tech Stack:** Sub2API Go 1.26/Gin backend, Vue 3 frontend, Docker Compose deployment; Cockpit Tools React 19/Tauri 2/Rust desktop app; GitHub PR workflow.

---

## Current Context / Assumptions

- Repositories inspected:
  - Cockpit Tools: `/home/opc/src/cockpit-tools`, upstream `jlcodes99/cockpit-tools`, current checkout `2ad714e`.
  - Sub2API fork/deployment: `/home/opc/src/sub2api`, branch `feat/local-admin-bypass`, latest commit `9aa8adc`.
- Live Sub2API deployment is under `/opt/sub2api` and currently exposes:
  - `https://sub2api.research.for-immortal.cn/` public HTTPS, normal login required.
  - `http://127.0.0.1:8027/` local-only admin bypass path.
  - `http://100.97.17.1:8027/` WireGuard-only admin bypass path.
- Sub2API current custom behavior:
  - `/` and `/home` redirect to `/admin/dashboard`.
  - Public unauthenticated users are redirected to `/login?redirect=/admin/dashboard`.
  - Local/WireGuard browser hosts can auto-login via `/api/v1/auth/local-admin`.
- Cockpit Tools is a desktop app focused on local AI IDE account switching and multi-instance management. It is not a Sub2API server and should not be treated as a drop-in replacement.
- Integration should avoid storing Sub2API admin tokens in Cockpit Tools unless explicitly required and reviewed.
- Public Sub2API auth must remain protected.

## Proposed Approach

Implement in phases:

1. **Document current interoperability** so a developer/user knows exactly how to point clients or browsers at Sub2API from local/WireGuard machines.
2. **Add a Sub2API connection/profile concept to Cockpit Tools only if needed**, initially as local configuration metadata, not as credential sync.
3. **Optionally add Sub2API discovery metadata endpoint** to Sub2API if Cockpit Tools needs reliable server capability detection.
4. **Keep admin bypass constrained to local/WireGuard hosts**, with explicit CIDR/env controls and negative tests for public domain.

This plan is intentionally conservative: start with docs and small configuration affordances, not deep account-token migration between systems.

---

## Task 1: Add a Sub2API/Cockpit interoperability note to Sub2API docs

**Objective:** Document how Cockpit Tools users should access the current Sub2API deployment over public HTTPS, localhost, and WireGuard.

**Files:**
- Create: `docs/COCKPIT_TOOLS_INTEROP.md`
- Modify: `README.md` only if adding a short link is desired after review.

**Step 1: Create the documentation file**

Create `docs/COCKPIT_TOOLS_INTEROP.md` with this content:

```markdown
# Cockpit Tools Interoperability

Sub2API and Cockpit Tools solve different layers of the AI tooling workflow:

- Cockpit Tools is a local desktop cockpit for AI IDE accounts and instances.
- Sub2API is a server-side AI API gateway and admin platform.

## Current Sub2API access points

| Purpose | URL | Auth behavior |
|---|---|---|
| Public web/admin access | `https://sub2api.research.for-immortal.cn/` | Normal login required |
| Local server access | `http://127.0.0.1:8027/` | Local admin bypass enabled |
| WireGuard access | `http://100.97.17.1:8027/` | WireGuard admin bypass enabled |

## Recommended workflow

1. Use Cockpit Tools locally for desktop AI IDE account and instance management.
2. Use Sub2API for OpenAI-compatible/API-gateway access and server-side admin control.
3. From a WireGuard-connected desktop, visit:

   ```text
   http://100.97.17.1:8027/
   ```

   This redirects to the admin dashboard and uses the local/WireGuard bypass.

4. From the public Internet, visit:

   ```text
   https://sub2api.research.for-immortal.cn/
   ```

   This requires normal login.

## Security constraints

- The local admin bypass must never be available through the public hostname.
- The trusted CIDR is configured by `SUB2API_LOCAL_ADMIN_BYPASS_CIDRS`.
- The current WireGuard CIDR is `100.97.17.0/24`.
- The current WireGuard server address is `100.97.17.1`.

## Verification commands

From the Sub2API server:

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8027/health
curl -sS -o /dev/null -w '%{http_code}\n' http://100.97.17.1:8027/health
```

From a WireGuard client such as `precision`:

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://100.97.17.1:8027/health
```

Public bypass should remain denied:

```bash
curl -sS -o /tmp/sub2api-public-bypass.json \
  -w '%{http_code}\n' \
  -X POST https://sub2api.research.for-immortal.cn/api/v1/auth/local-admin
```

Expected public result: `403`.
```

**Step 2: Verify docs render as Markdown**

Run:

```bash
cd /home/opc/src/sub2api
python3 - <<'PY'
from pathlib import Path
p = Path('docs/COCKPIT_TOOLS_INTEROP.md')
text = p.read_text()
assert '# Cockpit Tools Interoperability' in text
assert '100.97.17.1:8027' in text
assert '403' in text
print('docs ok')
PY
```

Expected: `docs ok`.

**Step 3: Commit**

```bash
cd /home/opc/src/sub2api
git add docs/COCKPIT_TOOLS_INTEROP.md
git commit -m "docs: add cockpit tools interoperability note"
```

---

## Task 2: Add a short README link to the interoperability note

**Objective:** Make the interoperability note discoverable without expanding the README significantly.

**Files:**
- Modify: `README.md`
- Optional mirror later: `README_CN.md`, `README_JA.md`

**Step 1: Add a small docs link**

Add near the existing documentation/deployment section, not in the sponsor block:

```markdown
### Cockpit Tools interoperability

For using this Sub2API deployment alongside Cockpit Tools, including local and WireGuard admin access, see [`docs/COCKPIT_TOOLS_INTEROP.md`](docs/COCKPIT_TOOLS_INTEROP.md).
```

**Step 2: Verify link target exists**

Run:

```bash
cd /home/opc/src/sub2api
test -f docs/COCKPIT_TOOLS_INTEROP.md
python3 - <<'PY'
from pathlib import Path
readme = Path('README.md').read_text()
assert 'docs/COCKPIT_TOOLS_INTEROP.md' in readme
print('readme link ok')
PY
```

Expected: `readme link ok`.

**Step 3: Commit**

```bash
cd /home/opc/src/sub2api
git add README.md
git commit -m "docs: link cockpit tools interoperability guide"
```

---

## Task 3: Add regression tests for Sub2API root/login behavior if not already covered

**Objective:** Prevent the index page from returning silently in the future; root should route to admin/dashboard and public unauthenticated users should see login.

**Files:**
- Modify/Create: `frontend/src/router/__tests__/rootRedirect.spec.ts` if router tests exist.
- Otherwise Create: `frontend/src/router/rootRedirect.test.ts` following existing Vitest conventions.

**Step 1: Inspect existing router tests**

Run:

```bash
cd /home/opc/src/sub2api
find frontend/src -path '*test*' -o -path '*spec*' | sort | grep -E 'router|auth|redirect' || true
```

Expected: identify where route-guard tests belong.

**Step 2: Write failing test for `/` redirect target**

Use Vitest and Vue Router conventions already present in the repo. The assertion should prove that route records for `/` and `/home` redirect to `/admin/dashboard`.

Example test skeleton:

```ts
import { describe, expect, it } from 'vitest'
import router from '../index'

describe('root redirects', () => {
  it('redirects root and home to the admin dashboard', () => {
    const root = router.getRoutes().find((route) => route.path === '/')
    const home = router.getRoutes().find((route) => route.path === '/home')

    expect(root?.redirect).toBe('/admin/dashboard')
    expect(home?.redirect).toBe('/admin/dashboard')
  })
})
```

If `router` is not exported as default in a test-friendly way, add a small exported helper in `frontend/src/router/index.ts`:

```ts
export { routes }
```

and test the route table directly.

**Step 3: Run failing/passing test cycle**

Run:

```bash
cd /home/opc/src/sub2api/frontend
pnpm test:run frontend/src/router/__tests__/rootRedirect.spec.ts
```

Expected after current implementation: PASS.

If the current code cannot be imported cleanly due app store side effects, keep the test simpler by exporting `routes` and testing the array.

**Step 4: Commit**

```bash
cd /home/opc/src/sub2api
git add frontend/src/router/index.ts frontend/src/router/__tests__/rootRedirect.spec.ts
git commit -m "test: cover root admin redirect"
```

---

## Task 4: Add regression tests for local/WireGuard admin bypass host logic

**Objective:** Ensure `127.0.0.1`, `localhost`, `::1`, and `100.97.17.1` browser hosts can bypass only when enabled, while public hostname remains denied.

**Files:**
- Modify: `backend/internal/handler/auth_handler_local_admin_test.go`
- Existing implementation: `backend/internal/handler/auth_handler.go`

**Step 1: Ensure test table includes public negative case**

Add/confirm test cases like:

```go
{
    name:       "wireguard host and docker bridge remote",
    host:       "100.97.17.1:8027",
    remoteAddr: "172.18.0.1:55321",
    want:       true,
},
{
    name:       "wireguard host and wireguard peer remote",
    host:       "100.97.17.1:8027",
    remoteAddr: "100.97.17.23:55321",
    want:       true,
},
{
    name:       "public host rejected even from private remote",
    host:       "sub2api.research.for-immortal.cn",
    remoteAddr: "172.18.0.1:55321",
    want:       false,
},
```

Ensure the test sets:

```go
t.Setenv("SUB2API_LOCAL_ADMIN_BYPASS_CIDRS", "100.97.17.0/24")
```

**Step 2: Run targeted backend tests**

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test -tags unit ./internal/handler -run 'TestLocalAdminBypassEnabled|TestIsLocalAdminBypassRequest' -count=1
```

Expected: PASS.

**Step 3: Commit**

```bash
cd /home/opc/src/sub2api
git add backend/internal/handler/auth_handler_local_admin_test.go
git commit -m "test: cover wireguard admin bypass host rules"
```

---

## Task 5: Optional Cockpit Tools profile documentation only

**Objective:** If the integration should be documented from the Cockpit Tools side, add a non-invasive doc explaining how to use a Sub2API base URL from AI tools that support OpenAI-compatible endpoints.

**Files:**
- Create: `/home/opc/src/cockpit-tools/docs/sub2api-profile.md`
- Do not modify Cockpit runtime code yet.

**Step 1: Create docs**

```markdown
# Sub2API Profile Notes

Sub2API can be used as a server-side OpenAI-compatible/API gateway endpoint while Cockpit Tools manages local AI IDE accounts and instances.

## Example endpoints

Public endpoint:

```text
https://sub2api.research.for-immortal.cn
```

WireGuard admin endpoint:

```text
http://100.97.17.1:8027
```

Use the public endpoint for normal client/API consumption. Use the WireGuard endpoint only for trusted local administration.

## Security note

Do not paste Sub2API admin tokens or upstream provider tokens into unrelated Cockpit Tools account fields. Use normal API keys generated by Sub2API for client integrations.
```

**Step 2: Verify docs**

```bash
cd /home/opc/src/cockpit-tools
test -f docs/sub2api-profile.md
grep -q 'Sub2API' docs/sub2api-profile.md
```

**Step 3: Commit only if maintaining a Cockpit Tools fork**

```bash
cd /home/opc/src/cockpit-tools
git add docs/sub2api-profile.md
git commit -m "docs: add Sub2API profile notes"
```

Do not push unless the user explicitly wants a Cockpit Tools fork/PR.

---

## Task 6: Optional Cockpit Tools UI support for a Sub2API base URL profile

**Objective:** Add a minimal Cockpit Tools settings/profile field for a Sub2API base URL without managing Sub2API credentials or tokens.

**Do this only if the user confirms they want code changes in Cockpit Tools.**

**Files likely to change:**
- `/home/opc/src/cockpit-tools/src/pages/SettingsPage.tsx`
- `/home/opc/src/cockpit-tools/src/stores/...` relevant settings store after inspection
- `/home/opc/src/cockpit-tools/src-tauri/src/modules/config.rs`
- `/home/opc/src/cockpit-tools/src-tauri/src/commands/...` relevant config command file after inspection

**Step 1: Inspect existing settings storage**

Run:

```bash
cd /home/opc/src/cockpit-tools
grep -R "UserConfig\|get_user_config\|save.*config\|SettingsPage" -n src src-tauri/src | head -120
```

Expected: identify config model and Tauri command names.

**Step 2: Add failing Rust config test**

Add a test to `src-tauri/src/modules/config.rs` verifying that default config has no Sub2API URL or has an empty string:

```rust
#[test]
fn default_sub2api_base_url_is_empty() {
    let config = UserConfig::default();
    assert_eq!(config.sub2api_base_url, "");
}
```

Expected before implementation: FAIL because field does not exist.

**Step 3: Add minimal config field**

In `UserConfig`, add:

```rust
#[serde(default)]
pub sub2api_base_url: String,
```

Update `Default` implementation:

```rust
sub2api_base_url: String::new(),
```

**Step 4: Run Rust test**

```bash
cd /home/opc/src/cockpit-tools/src-tauri
cargo test default_sub2api_base_url_is_empty
```

Expected: PASS.

**Step 5: Add UI field in SettingsPage**

Add a text input labeled `Sub2API Base URL` with placeholder:

```text
https://sub2api.example.com
```

Do not store API keys in this task.

**Step 6: Run typecheck**

```bash
cd /home/opc/src/cockpit-tools
npm run typecheck
```

Expected: PASS.

**Step 7: Commit**

```bash
cd /home/opc/src/cockpit-tools
git add src-tauri/src/modules/config.rs src/pages/SettingsPage.tsx src/stores/*
git commit -m "feat: add Sub2API base URL setting"
```

---

## Task 7: Optional Sub2API capability endpoint for clients

**Objective:** Provide a non-secret public metadata endpoint so Cockpit Tools or other clients can detect that a target is Sub2API and see safe capabilities.

**Do this only if a client needs server discovery.**

**Files likely to change:**
- `backend/internal/server/routes/common.go` or similar common route file
- `backend/internal/handler/...` if common handlers are separated
- `backend/internal/server/routes/..._test.go`

**Step 1: Locate common route file**

```bash
cd /home/opc/src/sub2api
grep -R "RegisterCommonRoutes" -n backend/internal/server backend/internal/handler
```

**Step 2: Write failing test**

Add a test asserting:

```http
GET /api/v1/capabilities
```

returns:

```json
{
  "name": "sub2api",
  "features": {
    "openai_compatible_gateway": true,
    "admin_dashboard": true,
    "local_admin_bypass_configured": false
  }
}
```

Do not reveal CIDRs, secrets, admin email, or internal deployment topology.

**Step 3: Implement minimal handler**

Add a handler that returns static public capabilities plus safe feature flags only.

**Step 4: Run tests**

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test -tags unit ./internal/server/routes -run Capabilities -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
cd /home/opc/src/sub2api
git add backend/internal/server/routes backend/internal/handler
git commit -m "feat: add public capabilities endpoint"
```

---

## Task 8: Deployment verification after any Sub2API code changes

**Objective:** Ensure the live deployment still behaves correctly after rebuilding the custom image.

**Files:**
- No source edits.
- Deployment path: `/opt/sub2api/docker-compose.local.yml`

**Step 1: Build image**

```bash
cd /home/opc/src/sub2api
sudo docker build -t sub2api:local-admin-bypass .
```

Expected: image build succeeds.

**Step 2: Restart app only**

```bash
cd /opt/sub2api
sudo docker compose -f docker-compose.local.yml up -d --force-recreate sub2api
```

Expected: `sub2api` recreated; Postgres/Redis remain running.

**Step 3: Wait for health**

```bash
for i in $(seq 1 90); do
  health=$(sudo docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' sub2api 2>/dev/null || true)
  echo "sub2api health=$health"
  [ "$health" = healthy ] && break
  sleep 2
done
```

Expected: `sub2api health=healthy`.

**Step 4: Verify public negative case**

```bash
curl -sS -o /tmp/sub2api_public_bypass.json \
  -w 'status=%{http_code} type=%{content_type}\n' \
  -X POST https://sub2api.research.for-immortal.cn/api/v1/auth/local-admin
```

Expected: `status=403`.

**Step 5: Verify WireGuard/local positive cases**

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8027/health
curl -sS -o /dev/null -w '%{http_code}\n' http://100.97.17.1:8027/health
```

Expected: both `200`.

From `precision`:

```bash
ssh -o BatchMode=yes -o ConnectTimeout=8 precision \
  'curl -sS -o /dev/null -w "%{http_code}\n" --connect-timeout 5 --max-time 10 http://100.97.17.1:8027/health'
```

Expected: `200`.

---

## Files Likely to Change

Minimum recommended changes:

- `docs/COCKPIT_TOOLS_INTEROP.md`
- `README.md`
- `frontend/src/router/__tests__/rootRedirect.spec.ts` or equivalent test path
- `backend/internal/handler/auth_handler_local_admin_test.go`

Optional Cockpit Tools changes:

- `/home/opc/src/cockpit-tools/docs/sub2api-profile.md`
- `/home/opc/src/cockpit-tools/src-tauri/src/modules/config.rs`
- `/home/opc/src/cockpit-tools/src/pages/SettingsPage.tsx`
- `/home/opc/src/cockpit-tools/src/stores/...` after settings-store inspection

Optional Sub2API discovery endpoint changes:

- `backend/internal/server/routes/common.go` or equivalent
- `backend/internal/server/routes/*_test.go`

## Tests / Validation Summary

Sub2API backend:

```bash
cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test -tags unit ./internal/handler -run 'TestLocalAdminBypassEnabled|TestIsLocalAdminBypassRequest' -count=1
GOTOOLCHAIN=auto go test -tags unit ./internal/server/routes ./internal/server/middleware -count=1
```

Sub2API frontend:

```bash
cd /home/opc/src/sub2api/frontend
pnpm test:run
pnpm run build
```

Full deployment build:

```bash
cd /home/opc/src/sub2api
sudo docker build -t sub2api:local-admin-bypass .
```

Live deployment probes:

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8027/health
curl -sS -o /dev/null -w '%{http_code}\n' http://100.97.17.1:8027/health
curl -sS -o /tmp/sub2api_public_bypass.json -w '%{http_code}\n' -X POST https://sub2api.research.for-immortal.cn/api/v1/auth/local-admin
```

Expected:

- Local health: `200`
- WireGuard health: `200`
- Public bypass: `403`

## Risks / Tradeoffs

- **Credential leakage risk:** Do not copy upstream provider tokens from Cockpit Tools into Sub2API or vice versa without a separate security review.
- **License mismatch:** Cockpit Tools is CC BY-NC-SA 4.0; Sub2API is LGPL-3.0. Avoid copying code between projects. Prefer integration through configuration/docs/API calls.
- **Public bypass risk:** Any change to local/WireGuard admin bypass must preserve public-domain denial.
- **Client IP ambiguity:** Docker published ports may show the Docker bridge IP to the container, not the real WireGuard peer. Current implementation checks the browser-visible host IP and remote private/local status; keep tests for this.
- **Scope creep:** A full Cockpit Tools UI integration is optional. Documentation and safe base-URL metadata may be enough.

## Open Questions

1. Should Cockpit Tools get a first-class `Sub2API` profile UI, or is documentation enough?
2. Should Sub2API expose a public `/api/v1/capabilities` endpoint for tool discovery?
3. Should Cockpit Tools ever store Sub2API API keys, or should users configure them directly in the target IDE/client?
4. Is the current WireGuard subnet permanently `100.97.17.0/24`, or should the deployment docs describe how to regenerate this from `ip addr show wg0`?
5. Do we need Chinese/Japanese docs alongside the English interoperability note?

## Recommended First Execution Slice

Start with Tasks 1–4 only:

1. Add Sub2API interoperability docs.
2. Link docs from README.
3. Add frontend root redirect regression test.
4. Add/confirm backend WireGuard bypass regression tests.

Stop there and review. Do not modify Cockpit Tools runtime code until the user confirms they want a Cockpit Tools fork/PR.
