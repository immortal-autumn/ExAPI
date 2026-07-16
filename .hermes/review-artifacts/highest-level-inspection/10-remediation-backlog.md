# ExAPI Remediation Backlog

Derived from the 2026-07-13 highest-level inspection. This is a proposed sequence; no remediation was applied during inspection.

## Execution order

1. Unify/test the gateway route contract.
2. Stop clearly forbidden customer-email/payment workers; classify Batch Image/quota.
3. Complete key/secret inventory and design together with encrypted backup/restore.
4. Enforce product capabilities across server, build, runtime, workers, and assets.
5. Reassess dependency advisories by actual sink.
6. Replace Host-based local-admin trust before ingress expansion.
7. Close scheduler, S3 SSRF, OAuth, Redis, container, and provenance gaps.
8. Prune backend/schema only after recovery is proven.

Numbered subsections below are work-package IDs; the order above controls execution.

## Phase 0 — Immediate release gates

### 0.1 Fix embedded-frontend interception

Files:

- `backend/internal/web/embed_on.go`
- `backend/internal/web/embed_test.go` or adjacent middleware tests
- gateway route registration tests

Work:

1. Move all gateway compatibility roots into one shared predicate/manifest.
2. Include `/chat/completions`, `/embeddings`, and `/videos/*`.
3. Add method/path/content-type tests for every public alias.
4. Verify missing/invalid key -> `401`, valid disposable key -> gateway JSON, never HTML.

Acceptance:

```bash
curl -skS -o /dev/null -w '%{http_code} %{content_type}\n' \
  -H 'content-type: application/json' -d '{}' \
  https://sub2api.research.for-immortal.cn/chat/completions
```

Expected without key: `401` and JSON content type.

### 0.2 Reassess and replace spreadsheet dependency

Files:

- `frontend/package.json`
- `pnpm-lock.yaml`
- Usage export component/tests

Current evidence: SheetJS is dynamically imported to generate Usage exports; no production XLSX ingestion path was found.

Work:

1. Replace or upgrade SheetJS to a patched source despite export-only use.
2. Preserve export behavior and add spreadsheet-formula injection controls where cells may begin with formula markers.
3. If any ingestion path is introduced, add size/row/cell/string limits, formula/link handling, and a worker timeout before release.
4. Run `pnpm audit --prod` and targeted export regression tests.

### 0.3 Upgrade sanitizer/injection dependencies

1. Upgrade DOMPurify to a release patched for all reported advisories.
2. Update Mermaid/PostCSS/YAML/UUID transitives where reachable.
3. Move pnpm overrides to the supported configuration.
4. Test homepage/custom-menu/email-template rendering against XSS corpus.
5. Prefer removing arbitrary HTML/iframe surfaces in Phase 1.

### 0.4 Stop forbidden workers and classify ambiguous capabilities

Files:

- `backend/internal/service/wire.go`
- email queue, payment expiry, batch image, quota flusher providers
- provider/startup tests

Work:

1. Add product capability checks before service construction/start.
2. Do not start customer email queues or payment reconciliation in private mode.
3. Decide whether Batch Image and platform quota are retained gateway capabilities before disabling their routes/runtimes.
4. For each decision, change route, API, worker, UI, timer, and bundle roots atomically.
5. Assert zero Redis queue consumers/tickers for forbidden capability families.

### 0.5 Replace Host-based local administrator trust

Files:

- `backend/internal/handler/auth_handler.go`
- `backend/internal/server/middleware/public_control_plane_guard.go`
- Nginx/integration tests

Work:

1. Move local-admin login to a separate loopback/WireGuard listener or Unix socket, or require authenticated proxy/mTLS identity.
2. Never issue administrator privilege based on HTTP Host.
3. Keep public Nginx `/api` and `/admin` deny-by-default locations until replacement is deployed.
4. Test spoofed localhost/WireGuard Host from public, private-proxy, Docker-bridge, and direct transports.

### 0.6 Establish a tested recovery point

1. Configure scheduled off-host PostgreSQL backups.
2. Encrypt dump payloads independently of S3 transport credentials.
3. Replace `io.ReadAll` upload with bounded streaming/multipart upload.
4. Define Redis persistence/reconstruction for refresh-token, OAuth, concurrency, and scheduler state.
5. Perform and document an isolated restore drill with measured RPO/RTO.

### 0.7 Close scheduler, S3, and OAuth trust gaps

1. Add overlap prevention and atomic/distributed claiming to scheduled account tests.
2. Apply SSRF validation, destination pinning, redirect controls, and explicit private-endpoint allowlisting to backup S3 endpoints.
3. Require OAuth state and exact stored redirect binding for Grok.
4. Derive Gemini redirect origin only from canonical configuration or trusted-proxy-aware state.

## Phase 1 — Frontend capability rewrite

### 1.1 Introduce one versioned, default-deny product capability contract

Replace hard-coded hostname inference and mixed `isSimpleMode` checks with an authoritative server bootstrap loaded before router/app initialization.

Requirements:

1. Stable capability IDs and schema version for routes, public pages, panels, actions, API families, workers, timers, integrations, data dimensions, and build roots.
2. Server-side enforcement for route registration, API families, worker construction, and integrations; browser capabilities are not authorization.
3. A private-product frontend entry point or compile-time registry so disabled route/component roots are not emitted.
4. Generated/parity-tested Nginx, public-guard, embedded-SPA bypass, gateway registration, and security-header path classifications from one gateway contract.
5. Bidirectional tests: enabled capabilities map to implementations; disabled capabilities are absent from router, DOM, network, timer graph, backend route/worker graph, and generated assets.
6. Explicit inventory for Profile, Key Usage, Batch Image, custom pages, monitor/channel routes, setup/auth callbacks, and global header/sidebar components.

### 1.2 Remove onboarding from private mode

Files:

- `AppLayout.vue`
- `AppHeader.vue`
- `useOnboardingTour.ts`
- onboarding store/styles/steps/tests

Acceptance:

- No tour DOM after 5 seconds.
- No tour replay control.
- No `driver.js`/onboarding private chunk or CSS.
- No onboarding timer/listeners in private mode.

### 1.3 Rebuild retained pages

Dashboard:

- Keep gateway traffic, health, accounts, API keys, models, recent errors.
- Remove users, revenue, customer ranking.

Usage:

- Keep request/account/model/status/error/latency/token diagnostics.
- Remove user ranking, user balance, billing/customer dimensions.

API Keys:

- Keep create/revoke/mask/ACL/usage.
- Remove CCS import and customer group semantics unless approved.

Accounts/Proxies:

- Keep core upstream routing/authentication.
- Decide CRS integration, multiplier fields, and import/export explicitly.

Profile:

- Remove the private header link and direct route.
- Move required administrator password/TOTP controls into operator Security.
- Assert no customer OAuth/balance/public-settings requests.

Batch Image:

- Decide retain/remove first.
- If retained, rewrite customer key/group/balance-freeze/pricing language into an operator gateway contract.
- If removed, delete Dashboard/Sidebar discovery, route/API/worker/timer roots, and emitted chunk.

Public/deep routes:

- Rebuild `/key-usage` as gateway request/quota diagnostics or remove it.
- Remove `/custom/:id` HTML/Markdown/iframe support unless explicitly approved.
- Decide `/monitor`, `/admin/channels/pricing`, and `/admin/channels/monitor`; remove customer group/pricing semantics.
- Add an explicit private/public ingress matrix for login, register, recovery, verification, setup, OAuth/payment callbacks, and legal routes.

### 1.4 Replace Settings composites

Create operator-specific tabs instead of hiding legacy parents:

- General: identity/docs/display only, if needed.
- Security: API-key IP policy + admin integration key only.
- Gateway: routing/session/retry/monitor/Ops controls.
- Email: SMTP + operator alerts/reports/account-expiry only, if approved.
- Backup: retained.

Delete imports/state/handlers for registration, login providers, user defaults, subscriptions, balance, recharge, payment, affiliate, agreement, arbitrary homepage HTML, and custom iframe menus.

### 1.5 Add recursive acceptance tests

For each retained route/tab:

- Explicit component allowlist.
- Forbidden component/text taxonomy.
- Network recording for at least 5 seconds.
- Timer/listener assertions.
- Deep-link redirect and chunk-load assertions.
- Build manifest assertion excluding retired route roots.

## Phase 2 — Backend graph reduction and correctness

### 2.1 Remove dormant routes/services/jobs

Order:

1. Stop workers.
2. Remove route providers.
3. Remove service providers and DI dependencies.
4. Remove repositories and configuration.
5. Remove tests that describe unsupported products; replace with negative capability tests.

Families:

- payment/billing
- subscription
- affiliate
- promo/redeem
- customer OAuth/login/connect
- customer announcements/email
- customer channels/profile/defaults
- batch image, unless operator-approved
- updater/restart

### 2.2 Implement or remove proxy connectivity test

If retained:

- Resolve and reject loopback/link-local/metadata/private destinations unless explicitly allowed.
- Enforce timeout, response-size limit, protocol allowlist, and no redirect escalation.
- Return DNS/connect/TLS stage result.
- Never log password/proxy URL credentials.

### 2.3 Decide Risk Control

- If retained: expose a compact operator-specific route and add it to product manifest/navigation/tests.
- If removed: delete route, view, chunk, API loaders, and backend capabilities.

## Phase 3 — Secret and backup security

### 3.1 Classify and protect every secret class

Inventory scope:

- account credential JSON
- gateway authentication keys
- proxy passwords
- SMTP and anti-bot secrets
- customer/provider OAuth client secrets
- web-search provider keys/settings JSON
- JWT signing root and data-encryption roots

Design requirements:

- versioned keyed digest/verifier for gateway authentication keys, with short non-secret display prefix and key-version rotation
- versioned AEAD envelope for upstream/proxy/SMTP/OAuth/provider credentials that require replay
- external or wrapped root keys outside the database/backup compromise boundary
- associated data including table/row/field identity
- rotation/rewrap and dual-read migration
- cache invalidation and revoked-key behavior
- redacted logs/errors and recovery runbook

Do not use reversible encryption for gateway authentication keys merely for convenience; do not hash credentials that must be replayed upstream.

### 3.2 Encrypt backup payloads

1. Stream database dump through authenticated encryption before upload.
2. Store encryption metadata separately from ciphertext.
3. Test restore in an isolated database.
4. Document bucket IAM, retention, object lock/versioning, and key recovery.
5. Never include encryption keys in the same backup target.

### 3.3 Remove dormant schema

Only after a verified backup/restore and service graph cleanup:

- Inventory foreign keys and migration dependencies.
- Archive if required.
- Drop dormant tables in reversible batches.
- Run full migration on empty and restored databases.

## Phase 4 — Performance, observability, and hardening

### 4.1 Reduce frontend bundle

- Remove dormant static imports from `SettingsView.vue`.
- Remove unreachable router import roots.
- Rebuild and fail on forbidden chunks.
- Keep Settings <180 KiB and Ops <200 KiB after cleanup rather than raising budgets.

### 4.2 Reduce database churn

- Stop payment polling.
- Review scheduler-outbox query/index strategy.
- Monitor Ops log growth and retention.
- Consider `pg_stat_statements` only through a reviewed database change.

### 4.3 Harden containers

Incrementally add:

- explicit non-root users
- capability drops
- read-only root filesystem plus named writable mounts/tmpfs
- PID/memory/CPU limits
- `no-new-privileges`
- health/resource alerts

### 4.4 Add provenance

Image labels/artifacts:

- `org.opencontainers.image.revision`
- creation timestamp
- source URL
- version/build ID
- SBOM
- signed provenance/attestation if supported

## Final acceptance suite

```bash
cd /home/opc/src/sub2api/frontend
./node_modules/.bin/vitest run
./node_modules/.bin/vue-tsc -b
npm run build
npm run check:bundle
pnpm audit --prod

cd /home/opc/src/sub2api/backend
GOTOOLCHAIN=auto go test ./...
GOTOOLCHAIN=auto go vet ./...

sudo docker compose -f /opt/sub2api/docker-compose.local.yml ps
sudo docker inspect sub2api --format '{{.Image}} {{json .Config.Labels}}'
```

Then perform:

1. Public/private route matrix.
2. Every public compatibility endpoint with missing, invalid, and disposable valid key.
3. Retained route/tab DOM/network/timer census.
4. Forbidden chunk manifest test.
5. Backup and restore drill.
6. Secret migration/rotation drill.
7. Container health and resource check.
8. Independent product and security review.
