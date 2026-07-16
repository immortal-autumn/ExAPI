# ExAPI Highest-Level Application Inspection Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Perform a read-only, end-to-end inspection of the deployed ExAPI/Sub2API private single-user gateway, identify every mismatch between the intended appliance surface and the actual source/runtime surface, and produce an evidence-backed remediation backlog without changing the application.

**Architecture:** Inspect the application as seven connected layers: product contract, frontend source/reachability, browser DOM/network behavior, backend routes/services/jobs, database/cache state, deployment/public boundary, and quality/security/performance. Reconcile static source evidence with the deployed image and live behavior so neither route-only tests nor file-existence assumptions can hide nested SaaS controls. Findings are recorded with severity, evidence, affected paths, and an explicit disposition: retain, hide/gate, remove, or investigate.

**Tech Stack:** Vue 3, TypeScript, Vite, Pinia, Vue Router, Vitest, Go, Gin, PostgreSQL, Redis, Docker Compose, OCI/Linux, WireGuard/private cockpit, OpenAI-compatible `/v1/*`, browser accessibility/console/resource inspection.

---

## Inspection Contract

This is an **inspection-only** plan.

- Do not modify source, tests, deployment configuration, database rows, Redis keys, DNS, nginx, Docker images, or running services.
- Do not restart or recreate containers.
- Do not invoke updater, rollback, migration, cleanup, payment, registration, or destructive admin endpoints.
- Do not print `.env` values, credentials, OAuth tokens, cookies, API keys, connection strings, or credential-bearing database columns.
- Read only allowlisted environment variable names/boolean mode values; never dump the full environment.
- Treat `/home/opc/src/sub2api` as the source repository.
- Treat `/opt/sub2api/docker-compose.local.yml` as the live deployment definition.
- Treat `https://sub2api.research.for-immortal.cn` as the public gateway.
- Treat `http://127.0.0.1:8027` and `http://100.97.17.1:8027` as private control-plane endpoints.
- Preserve PostgreSQL and Redis. Database inspection is schema/statistics/query-plan only unless the user separately authorizes a credentialed functional probe.
- Save all review output under `.hermes/review-artifacts/highest-level-inspection/`.
- Redact response bodies before writing artifacts. Prefer status codes, counts, route names, component names, and field names.

## Intended Product Contract

The review must test the implementation against this contract rather than against historical upstream behavior.

### Public surface

- Authenticated OpenAI-compatible `/v1/*` only.
- Missing or invalid API credentials return `401`.
- Browser pages, login, registration, payment, customer self-service, admin routes, updater, rollback, and restart remain unavailable publicly.

### Private cockpit

Retain only operator functionality needed to run a single-user gateway:

- Dashboard and operational health.
- Accounts/upstream OAuth management.
- API-key management.
- Usage and diagnostics.
- Gateway routing/settings.
- Gateway security and risk controls.
- Email only where required for operator alerts.
- Backup/restore controls.

### Explicitly out of product

- User registration and email verification.
- Invitation codes, promotions, redeem codes, affiliates, and payments.
- Customer login providers and customer account recovery.
- Multi-user management, customer profiles, customer 2FA, balances, subscriptions, and quotas.
- Customer marketing pages, onboarding tours, plans, support/contact merchandising, and self-service documentation links.
- In-app updater, rollback, restart, or service-management controls.

### Known discrepancy that must be treated as a confirmed finding

The deployed UI currently exposes nested SaaS controls even though top-level SaaS tabs/routes were removed:

- `RegistrationSettingsPanel` is rendered from `SecuritySettingsTab.vue`.
- Customer login panels for LinuxDo, GitHub/Google email OAuth, WeChat, DingTalk, and OIDC remain visible.
- Turnstile copy still targets login/registration.
- General settings still contain SaaS/customer-facing branding and support fields.
- The onboarding tour still advertises groups, user key distribution, and billing.

The inspection must find all analogous mismatches, not merely restate these examples.

---

## Finding Schema

Every finding in the final report must use this structure:

```markdown
### [ID] [Severity] Short title

- **Layer:** product | frontend | browser | backend | data | deployment | security | performance
- **Evidence:** exact file/line, route/status, DOM text, request URL, asset, query statistic, or test name
- **Expected:** product-contract behavior
- **Actual:** observed behavior
- **Impact:** security, operator confusion, dead weight, hidden traffic, maintenance risk, or performance
- **Disposition:** retain | gate in private mode | remove | investigate
- **Likely files:** exact paths
- **Acceptance test:** exact deterministic check for a future remediation
```

Severity rules:

- **Critical:** credential exposure, public admin access, arbitrary execution, or unauthenticated gateway use.
- **High:** public/private boundary bypass, destructive control exposed, or sensitive data leakage.
- **Medium:** visible non-product functionality, dormant requests/jobs, misleading controls, or meaningful resource waste.
- **Low:** stale copy/assets/tests, maintainability debt, or minor inefficiency.
- **Informational:** confirmed-safe controls and justified retained compatibility code.

---

### Task 1: Freeze and Record the Inspection Baseline

**Objective:** Establish exactly which source commit, working tree, image, and containers are being reviewed.

**Files:**
- Create during inspection: `.hermes/review-artifacts/highest-level-inspection/01-baseline.md`
- Read: `/opt/sub2api/docker-compose.local.yml`

**Step 1: Capture repository identity**

Run:

```bash
cd /home/opc/src/sub2api
git status --short
git branch --show-current
git rev-parse HEAD
git log -12 --oneline
```

Expected:

- Branch and commit recorded.
- Untracked `.hermes/plans/*.md` identified separately from runtime source.
- If tracked source is dirty, stop and ask whether to inspect working tree or committed HEAD.

**Step 2: Capture deployment identity**

Run:

```bash
sudo docker compose -f /opt/sub2api/docker-compose.local.yml ps
sudo docker inspect sub2api --format 'image={{.Config.Image}} image_id={{.Image}} started={{.State.StartedAt}} status={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}'
sudo docker image inspect sub2api:single-user-private-control --format 'id={{.Id}} created={{.Created}} size={{.Size}}'
```

Expected: app, PostgreSQL, and Redis are healthy; source/image drift is explicitly recorded.

**Step 3: Record only allowlisted runtime mode values**

Run a filter that prints only these names:

```bash
sudo docker inspect sub2api --format '{{range .Config.Env}}{{println .}}{{end}}' \
  | python3 -c "import sys; allowed={'SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE','SUB2API_PUBLIC_HOST','SERVER_PORT','GIN_MODE'}; [print(line.strip()) for line in sys.stdin if line.split('=',1)[0] in allowed]"
```

Expected: private-control-plane mode is enabled. Do not save any unrelated variables.

**Step 4: Write baseline artifact**

Record commands, sanitized output, timestamp, and any source/image mismatch.

**Step 5: Commit**

Do not commit during inspection. Review artifacts remain uncommitted until Elwin accepts the report.

---

### Task 2: Build a Product-Surface Truth Matrix

**Objective:** Create a single exhaustive matrix mapping intended capabilities to frontend routes, nested components, API calls, backend routes, services/jobs, and data tables.

**Files:**
- Create during inspection: `.hermes/review-artifacts/highest-level-inspection/02-product-surface-matrix.md`
- Read: `frontend/src/config/singleUserProduct.ts`
- Read: `frontend/src/router/index.ts`
- Read: `backend/internal/config/product_mode.go`
- Read: `backend/internal/server/routes/admin.go`
- Read: `backend/internal/server/routes/user.go`
- Read: `backend/internal/server/routes/auth.go`
- Read: `backend/internal/service/wire.go`

**Step 1: Enumerate frontend routes**

Use source search to list every route path and lazy import. Classify each as retain, compatibility redirect, unreachable legacy, or violation.

**Step 2: Enumerate backend routes**

List every registered route under `/api/v1`, `/v1`, auth, user, admin, payment, subscription, promotion, affiliate, system, and backup groups. Record the product-mode guard controlling each route family.

**Step 3: Enumerate services and workers**

Trace constructors and startup calls for subscription expiry, announcements, email, payment, cleanup, metrics, account tests, backups, and scheduler jobs. Distinguish “constructed but idle” from “started and active.”

**Step 4: Link data domains**

Map each capability to its PostgreSQL tables and Redis keys by schema/type names only. Never select secret-bearing values.

**Step 5: Write the matrix**

Use columns:

```markdown
| Capability | Contract | Frontend route | Nested UI | Browser requests | Backend route | Worker | Tables/cache | Status |
```

Expected: every product capability has an explicit status; no “unknown” remains without a follow-up finding.

---

### Task 3: Perform an Exhaustive Frontend Reachability Census

**Objective:** Find all visible, mountable, prefetched, or bundled SaaS functionality—including nested panels inside retained routes.

**Files:**
- Create: `.hermes/review-artifacts/highest-level-inspection/03-frontend-reachability.md`
- Read: `frontend/src/App.vue`
- Read: `frontend/src/router/index.ts`
- Read: `frontend/src/components/layout/AppLayout.vue`
- Read: `frontend/src/components/layout/AppSidebar.vue`
- Read: `frontend/src/components/layout/AppHeader.vue`
- Read: `frontend/src/composables/useOnboardingTour.ts`
- Read: `frontend/src/views/admin/DashboardView.vue`
- Read: `frontend/src/views/admin/SettingsView.vue`
- Read: `frontend/src/views/admin/settings/tabs/*.vue`
- Read: `frontend/src/views/admin/settings/panels/*.vue`
- Read: `frontend/src/stores/*.ts`
- Read: `frontend/src/api/**/*.ts`

**Step 1: Inventory all route-level views**

Record every `component: () => import(...)`, redirect, route guard, and restricted path.

**Step 2: Inventory every child component of retained views**

Starting from Dashboard, Ops, Accounts, API Keys, Usage, Settings, Security, and Backup, recursively follow imports and template tags. Do not stop at the tab component boundary.

For each child, record:

- Import type: static or async.
- Render condition: unconditional, `v-if`, `v-show`, slot, or dynamic component.
- Whether private mode participates in the condition.
- APIs/stores/timers created on import, setup, mount, watch, or route change.

**Step 3: Search product-language indicators**

Search frontend source and Chinese locale strings for:

```text
注册|登录|用户|支付|订阅|套餐|推广|邀请|兑换|优惠|余额|返佣|公告|客服|营销|更新|回滚|重启
registration|login|payment|subscription|plan|affiliate|promotion|redeem|billing|announcement|update|rollback|restart
```

Classify each hit as retained operator terminology, test-only, unreachable source, bundled source, or visible violation.

**Step 4: Audit global lifecycle behavior**

Trace all `onMounted`, `watch(..., { immediate: true })`, router hooks, polling, visibility listeners, timers, and store initialization reachable from private routes.

**Step 5: Audit onboarding and first-run behavior**

Trace how `useOnboardingTour.ts` is triggered, what storage/config controls it, and all copy/actions it exposes. Confirm whether private mode disables the tour.

**Step 6: Audit tests for false confidence**

Inspect route-matrix, product-manifest, Settings, sidebar, App lifecycle, and operator tests. Flag tests that only assert top-level tab absence while nested SaaS components remain visible.

**Step 7: Write frontend findings**

At minimum, include the confirmed Registration/Security and onboarding discrepancies with exact component paths and future acceptance tests.

---

### Task 4: Perform Browser DOM, Navigation, and Network Inspection

**Objective:** Observe the actual deployed UI across every retained private route and capture visible controls, hidden requests, timers, console errors, and dynamically loaded chunks.

**Files:**
- Create: `.hermes/review-artifacts/highest-level-inspection/04-browser-runtime.md`

**Step 1: Start a fresh browser session**

Navigate directly to each route with a cache-busting query:

```text
/admin/dashboard
/admin/ops
/admin/accounts
/admin/api-keys
/admin/usage
/admin/settings
/admin/risk-control
```

Use direct navigation rather than sidebar clicks when the browser harness is unreliable.

**Step 2: Census visible UI**

For each route, record:

- Headings, tabs, cards, dialogs, tours, buttons, switches, links, and menu items.
- Whether each maps to the product contract.
- Any customer/user/payment/registration/update language.
- Empty or misleading controls whose backend route is disabled.

For Settings, inspect every retained tab and expand every accordion/dialog without saving.

**Step 3: Inspect network requests**

After each route settles, wait beyond all known delayed timers (minimum five seconds), then collect sanitized resource URLs:

```javascript
performance.getEntriesByType('resource')
  .map(e => e.name)
  .filter(u => u.includes('/api/'))
  .map(u => new URL(u).pathname)
```

Do not collect request headers, cookies, or response bodies.

Flag requests to registration, users, payment, subscriptions, plans, announcements, affiliates, promotions, redeem, public settings not needed by private mode, update, rollback, or restart.

**Step 4: Inspect console/runtime errors**

Record console warnings/errors and uncaught exceptions. Separate product defects from browser-harness warnings.

**Step 5: Inspect loaded assets**

Record JavaScript chunk names loaded by each route. Flag payment, registration, customer login, announcements, or other SaaS chunks loaded on initial private navigation.

**Step 6: Test deep links and refreshes**

Refresh each retained route directly. Verify no login loop, blank page, or fallback to public/customer UI.

**Step 7: Write browser artifact**

Include a route-by-route table with DOM violations, unexpected API paths, console errors, and loaded legacy chunks.

---

### Task 5: Audit Backend Route Registration and Authorization Boundaries

**Objective:** Prove that disabled SaaS capabilities cannot be reached and retained operator/gateway routes have correct authorization.

**Files:**
- Create: `.hermes/review-artifacts/highest-level-inspection/05-backend-route-security.md`
- Read: `backend/internal/server/routes/*.go`
- Read: `backend/internal/server/middleware/*.go`
- Read: `backend/internal/config/product_mode.go`
- Read: `backend/internal/server/routes/single_user_route_matrix_test.go`

**Step 1: Build a static route inventory**

List method, path, handler, middleware, product-mode guard, and expected status without credentials.

**Step 2: Probe public boundary**

Probe status codes only for:

- `/`, `/login`, `/register`, `/admin/*`.
- Auth, registration, verification, recovery, customer OAuth.
- Payment, subscriptions, plans, affiliate, promotions, redeem, announcements.
- User administration and customer self-service.
- Update, rollback, restart.
- `/v1/models` without and with an invalid synthetic key.

Expected: control/SaaS routes `404`; gateway missing/invalid credentials `401`.

**Step 3: Probe private boundary**

Verify private SPA routes return `200`; raw admin APIs without browser auth return the intended denial status. Do not infer browser authorization behavior from curl alone.

**Step 4: Inspect local-admin bypass scope**

Trace host/IP checks and middleware order. Confirm bypass applies only to private/local browser control-plane flows and cannot affect the public domain or `/v1/*` gateway authentication.

**Step 5: Inspect CORS, trusted proxy, and client-IP handling**

Confirm trusted proxy headers are used only under explicit configuration and that public clients cannot spoof privileged locality.

**Step 6: Compare tests with route inventory**

Identify untested route families or tests that only inspect source strings rather than router behavior.

---

### Task 6: Audit Services, Workers, Pollers, and Hidden Background Work

**Objective:** Identify non-product work that still starts, polls, schedules, or consumes resources despite route removal.

**Files:**
- Create: `.hermes/review-artifacts/highest-level-inspection/06-background-work.md`
- Read: `backend/internal/service/wire.go`
- Read: `backend/internal/service/*.go`
- Read: `backend/internal/scheduler/**/*.go`
- Read: `backend/internal/repository/*.go`
- Read: `frontend/src/App.vue`
- Read: `frontend/src/stores/*.ts`
- Read: `frontend/src/composables/*.ts`

**Step 1: Enumerate backend startup hooks**

Find every goroutine, ticker, cron registration, `Start`, `Run`, watcher, cleanup, and scheduler call.

**Step 2: Classify each worker**

Retain only gateway/account health, operational metrics, backups, and explicitly required alerting. Classify payment, subscription, announcement, promotion, affiliate, customer email, and updater work as violations unless conclusively idle.

**Step 3: Enumerate frontend pollers/timers**

Find `setInterval`, delayed `setTimeout`, route hooks, visibility listeners, and store polling. Tie each to browser resource evidence.

**Step 4: Inspect failure behavior**

Determine whether disabled routes cause repeated 404s, log spam, retries, stale cached state, or misleading UI.

**Step 5: Write worker disposition table**

```markdown
| Worker/poller | Starts where | Trigger | Interval | External effect | Contract | Disposition |
```

---

### Task 7: Audit Data Model, Secret Handling, and Persistence Risk

**Objective:** Understand what dormant SaaS data and secret-bearing state remain without reading secret values or changing data.

**Files:**
- Create: `.hermes/review-artifacts/highest-level-inspection/07-data-and-secrets.md`
- Read: `backend/internal/model/*.go`
- Read: `backend/internal/repository/*.go`
- Read: `backend/migrations/*.sql`

**Step 1: Inventory tables and approximate sizes**

Use `pg_stat_user_tables`, `pg_stat_user_indexes`, and `pg_total_relation_size` only. Record counts and sizes, not row contents.

**Step 2: Classify tables by product domain**

Mark gateway-critical, operator-critical, dormant SaaS, audit/logging, or unknown.

**Step 3: Inventory secret-bearing fields by schema**

Record field/column names for API-key hashes, OAuth tokens, refresh tokens, cookies, passwords, SMTP credentials, admin keys, and proxy credentials. Never select values.

**Step 4: Inspect encryption/redaction pathways**

Trace model hooks, repository writes, API serialization, logger sanitization, backup inclusion, and UI masking.

**Step 5: Inspect backup scope**

Determine whether backups contain encrypted or plaintext secrets and how restore access is protected. Do not generate or download a backup.

**Step 6: Review retention and growth evidence**

Record table growth, dead tuples, and index use. Do not recommend schema/index changes without measured evidence.

---

### Task 8: Audit API-Key and Upstream Gateway Integrity

**Objective:** Verify that frontend cleanup did not weaken OpenAI-compatible gateway authentication, routing, model filtering, usage recording, or upstream account handling.

**Files:**
- Create: `.hermes/review-artifacts/highest-level-inspection/08-gateway-integrity.md`
- Read: `backend/internal/server/routes/proxy.go`
- Read: `backend/internal/handler/proxy*.go`
- Read: `backend/internal/service/*proxy*.go`
- Read: `backend/internal/service/*api_key*.go`
- Read: `backend/internal/repository/*api_key*.go`

**Step 1: Inspect authentication flow statically**

Trace bearer-key extraction, hashing/lookup, enabled/expiry checks, IP controls, group/model restrictions, concurrency/rate limits, and usage attribution.

**Step 2: Run unauthenticated and invalid-key probes**

Use a clearly synthetic invalid key. Verify `401` and ensure responses reveal no IDs or secret metadata.

**Step 3: Handle valid-key testing safely**

If no raw test key is available, mark authenticated probes blocked rather than extracting or exposing an existing key. If Elwin separately provides a disposable key through an approved secret channel, use it without printing and remove temporary response files afterward.

**Step 4: Inspect upstream account/OAuth safety**

Verify tokens are not returned to list endpoints, logs, browser state, or backup previews. Confirm account tests and schedulers use bounded timeouts and do not log credentials.

**Step 5: Reconcile usage evidence**

If a credentialed probe is authorized, compare aggregate usage counts before/after without selecting sensitive request content.

---

### Task 9: Audit Security Posture and Abuse Cases

**Objective:** Review the highest-impact attack surfaces beyond route presence.

**Files:**
- Create: `.hermes/review-artifacts/highest-level-inspection/09-security-review.md`
- Read: backend middleware, upload handlers, backup handlers, proxy handlers, OAuth callbacks, settings handlers, and frontend HTML/Markdown rendering utilities.

**Step 1: Secret scan tracked source and diffs**

Use Git-aware scanners or regex scans that report file/line and redact matches. Exclude `.git`, build output, dependencies, `.env`, and archives from printed output.

**Step 2: Inspect authorization and object access**

Review admin/account/API-key/usage/backup endpoints for missing ownership/admin checks and IDOR patterns.

**Step 3: Inspect injection boundaries**

Check SQL parameterization, shell execution, template/HTML rendering, Markdown/iframe support, URL validation, header forwarding, SSRF, path traversal, archive extraction, and file upload handling.

**Step 4: Inspect CSRF/CORS/session behavior**

Confirm private-browser mutations cannot be triggered cross-origin and public origins cannot obtain privileged credentials.

**Step 5: Inspect OAuth callback/state handling**

Even when customer OAuth routes are disabled, verify retained upstream-account OAuth uses state/nonce/redirect validation and does not share customer-login assumptions.

**Step 6: Review dependency advisories without updating**

Run read-only audit commands where lockfiles support them. Record actionable production vulnerabilities separately from dev-only noise; do not install or upgrade packages.

**Step 7: Request independent security review**

Delegate the sanitized route/security findings and relevant source diffs to a fresh reviewer. Fail closed if the reviewer cannot inspect evidence.

---

### Task 10: Audit Performance, Bundles, Queries, and Resource Use

**Objective:** Identify measured bottlenecks and dormant cost without speculative optimization.

**Files:**
- Create: `.hermes/review-artifacts/highest-level-inspection/10-performance.md`
- Read: `frontend/scripts/check-bundle-budget.mjs`
- Read: `frontend/vite.config.*`
- Read: relevant dashboard/settings/accounts views and backend query repositories.

**Step 1: Record frontend build composition**

Run the existing production build and bundle checker only if inspection execution permits non-deployment build artifacts. Record route chunks, shared chunks, and SaaS-named chunks.

**Step 2: Compare route load sets**

Use browser performance entries to identify chunks and requests loaded by each private route. Distinguish emitted-but-unloaded legacy chunks from initial-route cost.

**Step 3: Record container resource baseline**

Use read-only `docker stats --no-stream`, process/thread counts, and disk/image sizes. Do not tune resources.

**Step 4: Inspect query evidence**

Use PostgreSQL statistics and existing operational metrics. Run `EXPLAIN` only for read-only queries with sanitized constants; never use `EXPLAIN ANALYZE` on mutating statements.

**Step 5: Evaluate budgets**

Verify Accounts, Settings, and Ops budgets. Propose additional budgets for app shell, Dashboard, API Keys, and route-specific hidden-request counts where evidence justifies them.

---

### Task 11: Audit Quality Gates, Tests, and Deployment Drift

**Objective:** Determine whether current tests would catch the observed product-surface failures and whether the built image corresponds to reviewed source.

**Files:**
- Create: `.hermes/review-artifacts/highest-level-inspection/11-quality-and-drift.md`
- Read: frontend Vitest tests, backend route tests, Dockerfile, `.dockerignore`, Compose file, and CI workflows.

**Step 1: Run repository-wide validation**

Commands:

```bash
cd /home/opc/src/sub2api/frontend
NODE_ENV=test ./node_modules/.bin/vitest run
./node_modules/.bin/vue-tsc -b
npm run build
npm run check:bundle

cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test ./...
GOTOOLCHAIN=auto go vet ./...
```

Record warnings separately from failures.

**Step 2: Evaluate coverage by product contract**

Require tests that assert:

- Exact operator route list.
- Exact nested component allowlist for every retained settings tab.
- Absence of registration/customer-login/payment/onboarding controls in private mode.
- Absence of dormant SaaS network requests after timer windows.
- Backend route denial for every removed domain.
- Public/private boundary behavior.
- Updater/restart absence.

Flag skipped tests that encode removed behavior but remain in the default suite.

**Step 3: Verify source-to-image traceability**

Inspect Docker labels/build metadata. Record the current warning that Git commit information may not be captured by the build. Recommend traceability improvements as a finding, not an implementation.

**Step 4: Inspect ignored/generated artifacts**

Ensure local `frontend/dist` or embedded `backend/internal/web/dist` cannot silently diverge from Docker-built assets.

---

### Task 12: Produce the Highest-Level Report and Remediation Backlog

**Objective:** Consolidate all evidence into one decision-ready report with no remediation performed.

**Files:**
- Create: `.hermes/review-artifacts/highest-level-inspection/FINAL-REPORT.md`
- Create: `.hermes/review-artifacts/highest-level-inspection/REMEDIATION-BACKLOG.md`

**Step 1: Reconcile all artifacts**

Cross-check source, browser, route, worker, data, security, performance, and quality evidence. Resolve contradictions before reporting.

**Step 2: Write executive summary**

Answer:

1. Is the public gateway boundary secure?
2. Is the private cockpit actually minimal?
3. Which visible controls are misleading or nonfunctional?
4. Which dormant routes/services/jobs still consume resources?
5. Are secrets adequately protected?
6. Is gateway behavior preserved?
7. What should be fixed first?

**Step 3: List findings by severity**

Use the required finding schema and include exact evidence. Do not mix observations with remediation claims.

**Step 4: List passed controls**

Document verified-safe boundaries so future work does not regress or unnecessarily rewrite them.

**Step 5: Build remediation waves**

Group future work into reversible waves:

- **Wave 0:** tests that expose current nested UI/runtime mismatches.
- **Wave 1:** remove visible registration, customer login, payment, onboarding, and stale copy.
- **Wave 2:** stop remaining hidden frontend requests/pollers and remove unreachable imports.
- **Wave 3:** remove proven-idle backend workers/wiring while preserving schema compatibility.
- **Wave 4:** bundle/dependency cleanup and source-to-image traceability.
- **Wave 5:** rebuild, deploy only app container, and rerun acceptance.

Each backlog item must include exact files, acceptance criteria, dependencies, rollback, and risk.

**Step 6: Independent report review**

Dispatch two fresh reviewers in parallel:

- Product-surface reviewer: verify every contract capability has a disposition.
- Security/operations reviewer: verify severity, evidence, secret handling, and deployment conclusions.

Revise only factual errors or missing evidence; preserve disagreements as open questions.

**Step 7: Present report for acceptance**

Do not begin remediation until Elwin reviews and explicitly approves the backlog or selects a wave.

---

## Final Acceptance Criteria for the Inspection

The inspection is complete only when:

- Source commit and deployed image are identified and drift is explained.
- Every intended/removed capability appears in the product-surface matrix.
- Every retained route has a recursive nested-component inventory.
- Every retained route has live DOM, resource, console, and loaded-chunk evidence.
- All frontend timers, watchers, pollers, and backend workers have dispositions.
- Public and private route matrices are tested.
- Secret-bearing schema is documented without exposing values.
- Gateway authentication/routing integrity is reviewed; credentialed checks are either safely completed or explicitly blocked.
- Full frontend/backend quality gates are recorded.
- Findings are severity-ranked and independently reviewed.
- A reversible remediation backlog exists.
- No source, deployment, service, database, or cache mutation occurred during inspection.

## Risks and Tradeoffs

- **False confidence from route tests:** A removed tab can still leave nested controls visible. Recursive component and browser inspection is mandatory.
- **Source/runtime mismatch:** Source can be clean while the deployed image is stale. Record both identities and compare behavior.
- **Dormant compatibility code:** File existence is not automatically a defect; visibility, loading, requests, worker startup, and maintenance cost determine disposition.
- **Credentialed gateway tests:** Do not weaken secret handling merely to complete a checkbox. Mark blocked if no disposable key is available.
- **Skipped legacy tests:** They may be harmless historical coverage or evidence of unfinished product cleanup. Classify individually.
- **Large scope:** Keep the inspection read-only and defer fixes into waves; mixing audit and remediation destroys evidence and makes causality ambiguous.

## Open Questions for Elwin After the Report

- Should operator SMTP/email alerting remain, or should the Email tab also be removed?
- Should administrator API-key integration remain distinct from ordinary gateway API keys?
- Should the IP-management navigation item remain part of the minimal cockpit?
- Should dormant SaaS tables/migrations remain indefinitely for upgrade compatibility, or be handled in a later archival migration?
- Is a disposable gateway key available through an approved secret channel for final credentialed `/v1` acceptance?
