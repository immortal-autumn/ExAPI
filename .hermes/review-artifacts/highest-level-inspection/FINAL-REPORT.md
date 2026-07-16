# ExAPI Highest-Level Application Inspection — Final Report

**Date:** 2026-07-13
**Source commit:** `44613bfaf950b76d066f01cd763444b52f80f7c5`
**Deployment:** `sub2api:single-user-private-control` / image `sha256:e8904927…`
**Mode:** private single-user control plane with authenticated public gateway

## Executive assessment

**Decision: HOLD for targeted remediation, not emergency shutdown.**

The primary public/private security boundary is working:

- Public control-plane and legacy SaaS paths return `404`.
- Missing/invalid gateway credentials return `401` on correctly routed gateway paths.
- Local administrator bypass is currently network-confined by Nginx and loopback/WireGuard bindings, but its Host-based application check is deployment-fragile.
- CORS fails closed.
- Containers are healthy and quality gates pass.

However, the application is not yet a clean single-user gateway appliance. Six release-blocking findings require action, with severity and priority tracked separately:

1. Several public compatibility aliases are broken because the embedded SPA intercepts them.
2. Production dependencies have known advisories; current XLSX use is export-only, while sanitizer risk depends on each reachable HTML sink.
3. Gateway keys, upstream/provider credentials, settings secrets, and the JWT signing root are stored recoverably in PostgreSQL and dumps.
4. Nested retained UI still exposes customer authentication, email, Profile, Batch Image, analytics, custom HTML/iframe, channel pricing, public key-usage, and onboarding capabilities.
5. Local-admin privilege depends partly on a spoofable Host and would become critical behind a catch-all proxy.
6. No configured application backup or recorded recovery point exists.

The product cleanup also remains incomplete: backend route omission is much stronger than frontend capability enforcement, while dormant workers, services, schemas, and bundles remain.

## Severity and priority model

| Finding class | Security / operational severity | Release priority |
|---|---|---|
| Demonstrated current public compromise | None found | — |
| Local-admin Host trust | High latent; currently deployment-mitigated | P0 before any ingress/binding expansion |
| No recovery point | High operational impact | P0 before destructive migration |
| Plaintext secret/signing inventory | Moderate likelihood / High confidentiality impact | P0 design, phased migration |
| Alias routing contract | Moderate correctness/availability | P0 release blocker |
| Nested product capabilities | Moderate product-boundary / future-reactivation risk | P0/P1 product blocker |
| Dependency advisories | Low–Moderate on evidenced export/admin-authored paths | P1 hygiene and sink-specific review |
| Remaining worker/SSRF/OAuth/Redis/container/test debt | Low–Moderate individually | P1/P2 by dependency order |

## Release-blocking findings

### RB-01 — Root compatibility gateway aliases are intercepted by the SPA

**Evidence:** `backend/internal/web/embed_on.go:300-311`; live public probes.

The embedded frontend bypass list includes `/responses` and `/images/*` but omits:

- `/chat/completions`
- `/embeddings`
- `/videos/*`

Live unauthenticated POSTs return `200 text/html` instead of reaching API-key middleware. `/responses`, `/images/generations`, `/v1beta/*`, and Codex-direct paths correctly reach gateway handling and return `401` without credentials.

**Impact:** advertised compatibility endpoints are nonfunctional and can return misleading success/HTML to API clients. This is not an auth bypass.

**Remediation:** add all registered root aliases to the bypass predicate; generate the bypass set from route registration or a shared gateway-prefix manifest; add table-driven method/path/content-type tests and public smoke tests.

### RB-02 — Reachable production dependencies have known security advisories

`pnpm audit --prod` found 2 high, 21 moderate, and 3 low package advisories. The two high advisories affect `xlsx@0.18.5`, but source tracing found the library dynamically imported only to **generate Usage exports**; no production XLSX ingestion path was found. The audit rating therefore does not establish a malicious-file exploit path in this deployment.

DOMPurify advisories require sink-specific analysis. Configurable HTML/Markdown/custom-page/preview surfaces are reachable, but much of the content is authored by the trusted operator. Mermaid, PostCSS, YAML, and UUID transitives also have advisories.

**Impact:** Low–Moderate on currently evidenced paths; higher only where less-trusted content reaches a vulnerable sanitizer sink. Retaining unnecessary vulnerable packages still increases future risk.

**Remediation:** replace or upgrade SheetJS despite export-only use; upgrade DOMPurify and test every reachable sink with relevant bypass payloads; remove arbitrary HTML/iframe features if not required; update transitives; fix pnpm override configuration; rerun audit and regression tests.

### RB-03 — Operational secrets are plaintext at the database layer and in dumps

Confirmed recoverable classes include:

- Upstream credential JSON, gateway API keys, and proxy passwords.
- SMTP and Turnstile secrets.
- LinuxDo, DingTalk, OIDC, GitHub, Google, and WeChat client/application secrets.
- Web-search provider API keys inside settings JSON.
- The JWT signing root in `security_secrets`; bootstrap persists and prefers the database value.

Responses mask/redact several values and S3 transport credentials are encrypted, but there is no complete at-rest protection boundary.

**Impact:** database read access, snapshots, or dumps disclose replayable credentials and signing material. Likelihood depends on database/snapshot compromise; confidentiality impact is high.

**Remediation:** first complete a field/key inventory. Store gateway authentication keys as versioned keyed digests/verifiers, not recoverable ciphertext. Envelope-encrypt credentials that require replay. Keep/wrap signing and encryption roots outside the protected database/backup boundary. Include rotation, cache invalidation, independent backup encryption, and restore tests.

### RB-04 — Private product mode does not enforce nested capabilities

The product manifest limits routes/navigation/top-level Settings tabs but not nested panels. Live retained tabs expose:

- Registration policies and email-domain allowlists.
- LinuxDo, GitHub, Google, WeChat, DingTalk, and OIDC customer login/connect controls.
- Subscription expiry, customer verification/reset, balance/recharge, and customer mail templates.
- Multi-user dashboard metrics and rankings.
- User ranking/balance/billing dimensions in Usage.
- Customer Profile navigation and its OAuth/balance-notification/public-settings requests.
- Batch Image entitlement discovery, direct route, customer billing/group semantics, and conditional Dashboard/Sidebar action.
- Unauthenticated `/key-usage` subscription/plan/balance/expiry surface.
- Direct `/custom/:id`, `/monitor`, `/admin/channels/pricing`, and `/admin/channels/monitor` routes.
- Arbitrary homepage/custom-page HTML, Markdown, and iframe menus.
- Legacy onboarding and replay control.

**Impact:** Moderate product-boundary failure and future-reactivation risk inside a private authenticated operator plane; not a demonstrated public privilege escalation. Tests give false confidence by checking only top-level tabs.

**Remediation:** use a versioned, default-deny capability schema injected before app startup and shared by frontend routing, backend route/API registration, worker construction, integrations, timers, and data dimensions. Add a private-product build entry/compile-time registry so disabled roots are not emitted. Replace retained composites with operator-specific components and add bidirectional absence tests across router, DOM, network, timers, backend graph, and generated assets.

### RB-05 — Local-admin bypass trusts a spoofable Host and is safe only because of current Nginx routing

The local-admin handler issues the first active administrator's normal token pair when Host/source tests pass. `Host: localhost` plus a private proxy/Docker `RemoteAddr` satisfies those application checks, and the public-host guard only blocks the configured public hostname.

A catch-all reverse proxy that forwards attacker-controlled Host would therefore create a critical public administrator-login vulnerability.

The active deployment currently prevents that path: Nginx does not proxy `/admin/*` or `/api/*` on the public virtual host, state-free Host-spoof checks returned the same Nginx 153-byte `404`, and the app port is bound only to loopback/WireGuard. The finding is therefore **High latent risk, deployment-mitigated now**, rather than a demonstrated current Critical vulnerability.

**Remediation:** move local administration to a separate listener/Unix socket or require an authenticated proxy/mTLS boundary. Never grant privilege based on HTTP Host. Add active Nginx and application tests for spoofed Host from a private proxy source.

### RB-06 — No configured application backup or recorded recovery point exists

Read-only settings/record checks found no S3 backup configuration, no schedule configuration, and no backup records. PostgreSQL-only backup also excludes Redis authentication/session/scheduler state. The implemented dump path gzip-compresses but does not encrypt payloads, and the S3 store reads the full compressed dump into memory before upload.

**Impact:** volume loss, migration failure, or operator error may have no application-managed recovery point; future backups would contain recoverable operational secrets unless encrypted.

**Remediation:** configure scheduled encrypted off-host backups, define Redis recovery/persistence policy, and perform a documented restore drill with measured RPO/RTO before destructive schema work.

## Additional findings

### M-01 — Forbidden customer/payment workers still start; Batch Image/quota require classification

- Three customer email workers start unconditionally.
- Payment-order expiry/reconciliation starts unconditionally, runs immediately and every 60 seconds, and can call payment providers. It does have timeouts, graceful stop, and a multi-instance leader lock.
- Batch-image routes and runtime may be part of a retained gateway contract; platform-quota behavior likewise requires explicit product classification before shutdown.
- Subscription expiry is the only inspected SaaS worker explicitly guarded by product mode.

**Action:** immediately remove customer email and payment workers from the private graph. Classify Batch Image and quota capabilities first; then use the shared capability contract to retain or remove route, API, worker, UI, and bundle roots together.

### M-02 — Full SaaS service/schema graph remains

Payment, affiliate, redeem, subscription, customer identity, customer channel, billing, announcement, and image-job services/tables remain. Most live tables are empty, but the graph increases accidental-reactivation, migration, backup, and test burden.

**Action:** remove dependency-injection providers first, then jobs/routes, then migrations/tables in a backed-up phase.

### M-03 — Tests validate source strings/top-level visibility, not recursive behavior

`SettingsView.operator.spec.ts` passes while retained Security/Email tabs render forbidden content. Existing tests do not assert recursive component allowlists, delayed requests, timers, or generated asset absence.

**Action:** add mounted private-mode route/tab tests, forbidden taxonomy assertions, >5-second network/timer tests, and manifest/chunk checks.

### M-04 — `ProxyService.TestConnection()` is a no-op

The visible operator control returns success without making a connection.

**Action:** implement a bounded DNS/connect/TLS test with SSRF restrictions and explicit result details, or remove the button.

### M-05 — Container isolation is minimal

Containers are non-privileged but have no explicit non-root user, read-only root filesystem, cap drops, resource/PID limits, or custom security options.

**Action:** harden incrementally and verify writable paths/health after each restriction.

### M-06 — Dedicated Risk Control is emitted but unreachable

`/admin/risk-control` exists and emits ~89 KB but redirects to Settings because `risk_control_enabled` is false. Product intent says security/risk controls should remain, so current behavior is ambiguous.

**Action:** decide whether to retain a dedicated operator risk center or only a minimal Settings panel; remove the other implementation and chunk.

### M-07 — Settings and Ops are near bundle budgets

- Settings: 199.31 KiB / 210 KiB.
- Ops: 222.38 KiB / 230 KiB.
- Fresh bundle: 4.71 MB / 145 files.
- Suspect customer/dormant named chunks: ~609 KB.

**Action:** remove dormant imports/state before raising budgets; split operator sections by genuine navigation boundaries.

### M-08 — Authenticated upstream integrity was not freshly tested

A fresh authorized request requires a disposable key. Creating one violates read-only scope; extracting the live key violates the no-secret rule. Existing usage data proves a prior successful account-attributed gateway request but is not a fresh test.

**Action:** run the defined disposable-key acceptance test during remediation deployment.

### M-09 — Deployment provenance is incomplete

Deployed browser chunk hashes match the fresh source build, and live behavior matches reviewed source. The image has no revision/created-version OCI labels, so backend provenance cannot be cryptographically tied to commit `44613bf` from metadata alone.

**Action:** add `org.opencontainers.image.revision`, source date, build ID, and SBOM/provenance artifacts.

### M-10 — Scheduled account tests can overlap or duplicate across instances

The minute cron allows five-minute runs and performs no skip-if-running or atomic/distributed claim before execution.

**Action:** add local overlap prevention and a database/Redis lease or atomic due-plan claim; test two runner instances.

### M-11 — Backup S3 endpoint is an administrator-controlled SSRF sink

The configured endpoint is passed directly to the AWS SDK and contacted during connection tests without the existing URL/SSRF validator.

**Action:** validate schemes, resolve and pin destinations, reject loopback/link-local/metadata/private and redirect/rebinding targets unless explicitly allowlisted. Replace `io.ReadAll` upload with bounded multipart/streaming upload.

### M-12 — Redis carries authoritative state without a memory ceiling

Sample: 106 keys, 54 expiring, `maxmemory=0`, `noeviction`, and substantially more misses than hits. Key families include refresh tokens, token families, OAuth, concurrency, and scheduling.

**Action:** classify authoritative versus cache state, define persistence/reconstruction, set container/Redis memory limits and suitable policy, and instrument hit/miss by family.

### M-13 — OAuth state/origin handling is inconsistent

Grok accepts bare authorization codes without mandatory state and permits redirect URI override. Gemini redirect derivation trusts Origin/forwarded headers independently of trusted-proxy settings.

**Action:** require state and exact stored redirect binding for every provider; derive external origin from canonical config or trusted-proxy-aware state.

## Low / informational findings

1. Redis application health is good, but container `REDISCLI_AUTH` does not authenticate operational `redis-cli` diagnostics.
2. Scheduler outbox has ~691k sequential scans for one live row.
3. Ops logs are the largest relation (~12.9 MB / ~11.9k rows); retention appears active but should be capacity-monitored.
4. Dashboard and Usage contain mixed Chinese/English and customer-oriented copy.
5. API Keys uses customer-shaped user/group/public-settings APIs and exposes CCS import.
6. Route prefetch loads Accounts from Dashboard; harmless at current scale but contrary to strict minimal-loading goals.

## Product-surface disposition matrix

| Surface | Current status | Recommended disposition |
|---|---|---|
| Dashboard | Retained, customer metrics visible | Keep; rebuild around gateway/account/key/health/usage only |
| Ops | Retained, useful | Keep; split/optimize |
| Accounts | Retained, CRS/multiplier/import/export | Keep core; explicitly decide integrations; secure exports |
| Proxies/IP management | Retained | Keep; fix test; secure import/export |
| API Keys | Retained, CCS/customer API coupling | Keep core; remove CCS and customer semantics unless explicitly approved |
| Usage | Retained, multi-user/billing dimensions | Keep request/account/model/error diagnostics; remove user/customer billing views |
| General Settings | Retained, mostly legacy | Replace with compact operator identity/docs/display config or remove |
| Security Settings | Mixed operator and customer auth | Split; retain API-key ACL/admin integration only; remove customer login/connect |
| Gateway Settings | Mostly aligned | Keep; recursively classify panels |
| Email Settings | Mostly SaaS | Remove or rebuild as operator alert transport/templates only |
| Backup Settings | Aligned | Keep; add dump encryption/restore verification |
| Risk Control | Emitted but unreachable | Product decision required |
| Customer Profile | Header link retained; loads OAuth/balance settings | Remove route/link; move admin password/TOTP to operator Security |
| Batch Image | Authenticated route, entitlement discovery, billing/group semantics, workers | Explicit retain/remove decision; change all layers atomically |
| Public Key Usage | Unauthenticated subscription/plan/balance/expiry view | Rebuild as gateway diagnostics or remove; test public/private ingress |
| Custom pages | Authenticated HTML/Markdown/iframe route and menu entries | Remove unless explicitly required; otherwise strict origin/content policy |
| Channel pricing/monitor | Deep routes remain directly navigable | Decide operator need; remove customer group/pricing semantics |
| Public auth/setup/callback roots | Broad route set remains registered | Explicit negative matrix at both ingress paths |
| Onboarding | Legacy SaaS | Remove from private mode entirely |
| Payment/subscription/users/promos/redeem | Backend routes hidden; code/schema remain | Remove in phased backend/schema cleanup |

## Quality and deployment verification

Passed on reviewed source:

- Frontend Vitest: 191 files, 984 passed, 11 skipped.
- Vue/TypeScript type-check.
- Frontend production build.
- Bundle budget check.
- Backend `go test ./...`.
- Backend `go vet ./...`.

Tracked source remained unchanged by the inspection. Only `.hermes` plans/review artifacts are untracked.

## Release decision

The current deployment can remain running because the sampled Nginx/public boundary is intact and existing `/v1/*` authentication works. This is a **product-release HOLD**, not an emergency security shutdown. Do not broaden Nginx proxy locations or publish the app port before RB-05 is replaced.

Recommended order:

1. Unify and test the gateway route contract; fix RB-01 across router, embed bypass, public guard, security headers, and Nginx parity.
2. Remove clearly forbidden customer-email/payment workers; classify Batch Image and quota before changing them.
3. Complete RB-03 secret/key inventory and cryptographic design together with RB-06 encrypted backup/restore.
4. Remove/rebuild RB-04 product capabilities using shared server/build/runtime enforcement.
5. Reassess and remediate RB-02 by actual package sink.
6. Replace RB-05 before any ingress/binding expansion.
7. Add scheduled-run claiming, S3 endpoint validation, and OAuth state/origin fixes.
8. Then prune service/schema graphs and harden containers/provenance.

## Evidence index

- `01-baseline.md`
- `02-frontend-surface.md`
- `03-backend-workers.md`
- `04-browser-runtime.md`
- `05-http-boundary.md`
- `06-data-secrets.md`
- `07-gateway-integrity.md`
- `08-performance-security.md`
- `09-quality-gates.md`
- `10-remediation-backlog.md`
- `11-independent-audit-adjudication.md`
