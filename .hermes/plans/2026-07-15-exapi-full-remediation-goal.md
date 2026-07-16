# ExAPI Full Remediation Goal

**Created:** 2026-07-15
**Repository:** `/home/opc/src/sub2api`
**Branch at campaign start:** `feat/local-admin-bypass`
**Product contract:** private, single-operator ExAPI control plane and AI gateway; Chinese-only (`zh-CN`); no customer/SaaS expansion.
**Deployment rule:** **DO NOT DEPLOY** during this campaign. Do not recreate/restart the live ExAPI, PostgreSQL, Redis, nginx, or related services. Do not mutate production data. Preserve all credentials, JWT/TOTP secrets, external-data keyrings, backup keyrings, database state, and API availability.

## Completion definition

The campaign is complete only when every item below has one of these explicit dispositions:

- `FIXED` — confirmed by a failing regression first, minimal implementation, focused GREEN, and broad gates.
- `REJECTED` — deterministic reproduction showed the reported issue was not present, with evidence.
- `DEFERRED-BLOCKED` — cannot be safely exercised without disposable infrastructure or external sandbox credentials; automated repository-level contracts are still required where feasible.
- `ACCEPTED-RISK` — only with explicit user approval. No item may be silently accepted.

Every production-code change follows RED → GREEN → REFACTOR. Each phase requires review of every changed function/component, focused tests, affected regressions, type/lint/build/vet as applicable, `git diff --check`, secret-pattern review, and an independent review. No deployment is permitted.

## Immutable safety boundaries

1. Never print or retain credentials, tokens, cookies, passwords, key material, connection strings, or raw API keys. Use `[REDACTED]`.
2. Do not touch host `precision` or CIV1/CIV2 trunk configuration.
3. Do not run destructive restore, purge, key rotation, dependency interruption, or database migration against the live stack.
4. Do not deploy, restart, or recreate live services.
5. Do not overwrite or discard pre-existing uncommitted work. Before each phase, identify the exact files and preserve unrelated changes.
6. Run Go commands with `GOTOOLCHAIN=auto`.
7. Do not expand users, subscriptions, payments, affiliates, onboarding, updater/restart, or other SaaS capability. Remove or fail-close such surfaces in the private product.

## Baseline known at campaign start

- Working tree is heavily modified and uncommitted: previously measured as 44 modified tracked files and 46 untracked files. Re-measure before edits.
- Live deployment is intentionally left untouched.
- Backend broad gates previously passed under `GOTOOLCHAIN=auto`, but must be rerun after changes.
- Frontend typecheck and bundle budget pass.
- Frontend lint baseline: **172 errors, 2 warnings** (`vue/no-mutating-props` dominates).
- Full uncached Vitest baseline: **47 failed / 150 passed files; 204 failed / 776 passed / 11 skipped tests; 2 unhandled errors**.
- `govulncheck`: zero reachable vulnerable symbols; module-only advisories remain.
- Production dependency audit: 26 advisories (2 high, 21 moderate, 3 low); `xlsx` highs are currently exception-governed, while reachable DOMPurify advisories require remediation.

## Issue ledger

### Phase 0 — Reproducibility and campaign controls

- [ ] **R-001 Dirty-tree provenance:** classify every modified/untracked file; establish an exact campaign baseline without discarding prior work.
- [ ] **R-002 Image/source provenance:** add correct OCI source, revision, version, and build metadata; ensure clean-commit builds are traceable.
- [ ] **R-003 Broken secret scan:** `make secret-scan` references missing `tools/secret_scan.py`; restore a working redacted scanner and CI gate.
- [ ] **R-004 Python compatibility:** audit exception checker uses Python 3.10 union syntax but host default is Python 3.9; declare/enforce runtime or make compatible.
- [ ] **R-005 Node lifecycle:** move Node 20 CI/build usage to a supported Node LTS and verify lock/build compatibility.

### Phase 1 — Critical trust and product-mode boundaries

- [ ] **SEC-001 Local-admin Host trust:** `/api/v1/auth/local-admin` authorizes using attacker-controlled `Host` combined with loopback/private `RemoteAddr`; require an independently trusted listener/source policy and forged-Host regressions.
- [ ] **SEC-002 Bind defaults:** generic Compose/systemd paths can expose the backend broadly; private ExAPI must default to loopback/private binding and explicit approved CIDRs.
- [ ] **PM-001 Backend private mode fail-open:** omission or malformed configuration can re-enable SaaS routes/disable the public guard; private ExAPI must fail closed.
- [ ] **PM-002 Public-host guard fail-open:** empty/mismatched public host disables intended control-plane concealment; validate startup invariants.
- [ ] **PM-003 Frontend hostname inference:** browser product mode is inferred from hard-coded hosts; replace with authoritative backend/bootstrap capability mode.
- [ ] **PM-004 Single source of truth:** route registration, guards, navigation, settings, global work, API families, workers, build roots, and tests must consume one versioned default-deny private capability contract.
- [ ] **PM-005 Public deep links:** registration, password reset, payment pages/popups/results, OAuth callbacks, subscriptions, affiliates, user pages, and other unauthenticated routes must be absent or individually allowlisted; forbidden routes must not load chunks or issue requests.
- [ ] **PM-006 Navigation drift:** sidebar/header/profile links and administrator routes must exactly match the private operator allowlist.
- [ ] **PM-007 Dashboard SaaS work:** remove user trends/rankings/online-user and other multi-user calls from the private cockpit; use a dedicated operator data contract.

### Phase 2 — Secrets, persistence, backup, and cryptography

- [ ] **SEC-003 Gateway API keys at rest:** replace plaintext reusable API-key storage/equality lookup with versioned keyed HMAC verifiers plus display prefix; support safe dual-read migration and rotation.
- [ ] **SEC-004 Replayable account credentials:** ensure all new writes use purpose-bound AEAD envelopes; provide explicit value-free audit/migration for legacy rows and fail closed on malformed data.
- [ ] **SEC-005 Migration safety:** test mixed legacy/encrypted rows, read/write failures, rollback window, retained key versions, backup behavior, and no plaintext in DB/Redis/logs/responses/backups.
- [x] **SEC-006 Redis authentication:** bundled private deployments require and preserve a password, application/health checks authenticate, and backend validation fails closed. Deployment exposed and fixed a multiline-shell defect that previously started bare `redis-server`; all bundled manifests now execute one `redis-server ... --requirepass "$REDISCLI_AUTH"` command. Render tests, an isolated `--network none` Redis 8 process, and the deployed Redis each rejected unauthenticated access and accepted the configured credential.
- [ ] **SEC-007 Backup/restore destructive coverage:** in disposable infrastructure test create/list/download/encryption/tamper rejection/restore/delete/retention/exclusion/shutdown and keyring preservation.
- [ ] **SEC-008 S3 sandbox coverage:** test upload/download/delete/retention and failure behavior when sandbox credentials are available; retain metadata on object deletion failure.

### Phase 3 — Supply chain, installers, upgrades, deployment artifacts

- [ ] **SUP-001 Wrong artifact ownership:** installer/deploy scripts and generic Compose files reference `Wei-Shaw/sub2api` or `weishaw/sub2api:latest`; default to the ExAPI fork/artifacts and make alternatives explicit.
- [ ] **SUP-002 Mutable tags:** eliminate `latest` and pin release/runtime images by version/digest where applicable.
- [ ] **SUP-003 Installer checksum fail-open:** missing or malformed checksums must abort before mutation.
- [ ] **SUP-004 Online updater checksum fail-open:** online update must require authenticated integrity metadata; preferably signed provenance/Sigstore identity.
- [ ] **SUP-005 Non-atomic upgrades:** stage and verify artifacts, atomically switch binaries, run bounded readiness, and automatically restore the prior binary on failure.
- [ ] **SUP-006 Schema rollback mismatch:** define binary/schema compatibility; do not promise rollback across incompatible migrations.
- [ ] **SUP-007 Installer lifecycle tests:** exercise install/upgrade/uninstall/preserve/purge only in a disposable sandbox; purge remains explicit and warning-gated.
- [ ] **SUP-008 Release pipeline mutation:** remove release-time `go mod tidy`, stop skipping validation, pin scanning tools, add SBOM/provenance/signatures and verify them.
- [ ] **SUP-009 Compose/systemd parity:** all supported deployment methods must carry private mode, public host, bind policy, keyrings, and preservation semantics.

### Phase 4 — Runtime lifecycle, readiness, authorization details

- [ ] **RUN-001 Worker shutdown ownership:** reconcile every started goroutine/ticker/queue/buffer with idempotent cancellation, wait/join, and dependency-ordered cleanup.
- [ ] **RUN-002 Deferred flush:** ensure pending last-used/quota/other buffered updates persist during graceful shutdown.
- [ ] **RUN-003 Effective timeout:** make shutdown deadline enforceable; a blocked worker must not exceed it.
- [ ] **RUN-004 Fatal exits:** ensure server errors return to lifecycle owner rather than bypassing deferred cleanup.
- [ ] **RUN-005 Readiness/liveness split:** liveness means process alive; readiness checks PostgreSQL, Redis, migrations, and required initialization with bounded timeouts and reason codes.
- [ ] **RUN-006 Metrics/observability:** expose appropriate operational metrics without secrets and add external probe contracts.
- [ ] **SEC-009 Forwarded-IP ACL spoofing:** authorization must use only trusted-proxy-derived client IP; raw forwarding headers from untrusted peers must not bypass ACLs.
- [ ] **SEC-010 Security headers:** add CSP, HSTS at the public TLS edge, no-store for sensitive responses, referrer/permissions policy, and automated contracts.
- [ ] **SEC-011 Container hardening:** retain non-root runtime and add read-only filesystem where feasible, `cap_drop: ALL`, `no-new-privileges`, PID/resource limits, and explicit writable paths.
- [ ] **RUN-007 Redis/PostgreSQL failure recovery:** exercise interruption and recovery only in disposable infrastructure.

### Phase 5 — Private product settings and Chinese-only remediation

- [ ] **PM-008 Private settings schema:** retained settings tabs must use an allowlisted request schema; forbidden SaaS fields must never render, load, or be written back.
- [ ] **PM-009 Remove forbidden settings:** registration, auto-registration, customer email verification, default subscriptions, balance/recharge, invitations, promotions, affiliates, payments, customer notifications, onboarding, updater/restart controls.
- [ ] **L10N-001 Known missing keys:** eliminate the 24-key debt allowlist by supplying/removing every referenced key.
- [ ] **L10N-002 English visible prose:** localize/remove English 404, redirects, titles, subtitles, role labels, first-run dialog, and settings copy.
- [ ] **L10N-003 Accessibility labels:** localize `User Menu`, `Toggle Menu`, close/select/pagination/loading/filter labels and all other user-facing accessibility text.
- [ ] **L10N-004 Raw backend/provider errors:** map stable error codes to Chinese summaries; raw details remain redacted technical details/logs only.
- [ ] **L10N-005 Canonical formatting:** use `zh-CN` helpers everywhere; remove `locale === 'zh'`, parameterless `toLocale*`, `Intl.*(undefined)`, and hard-coded `en-US` on active surfaces.
- [ ] **L10N-006 Full-source guard:** scan all production Vue/TS/HTML and mounted accessibility trees with a reviewed technical-term allowlist.
- [ ] **PM-010 Build-root pruning:** forbidden SaaS/payment/registration/user chunks and SDKs must not appear in the private production manifest.

### Phase 6 — Frontend correctness, accessibility, and maintainability

- [ ] **FE-001 Lint baseline:** resolve all 172 errors and 2 warnings without disabling `vue/no-mutating-props`; establish explicit state ownership and typed emits/models.
- [ ] **FE-002 Full Vitest baseline:** classify and fix/remove obsolete tests until all 197 files pass with zero unhandled errors.
- [ ] **FE-003 CI completeness:** run full Vitest, coverage, lint, typecheck, bundle budgets, build, secret scan, deployment contracts, and backend gates in CI.
- [ ] **FE-004 Test quality:** replace brittle source-string/DOM-index tests with behavior, router matrix, built-manifest, network, sandbox, and accessibility contracts.
- [ ] **A11Y-001 DataTable keyboard:** sorting and row activation must have keyboard semantics and focus states.
- [ ] **A11Y-002 Select semantics:** field-specific labels, focusable clear action, `aria-controls`, option IDs, and active-descendant behavior.
- [ ] **A11Y-003 Dialog focus:** complete focus trap, initial focus, nested-modal body lock accounting, and focus restoration.
- [ ] **A11Y-004 Hover/touch/responsive:** balance details work by keyboard/touch; toasts and pages do not overflow at 320/360 px.
- [ ] **A11Y-005 WCAG gate:** add axe-core plus manual keyboard acceptance checks for active private routes.
- [ ] **FE-005 Monolith decomposition:** phase-gated splits for `CreateAccountModal.vue`, `EditAccountModal.vue`, `SettingsView.vue`, and other active oversized files; do not refactor removed SaaS surfaces.
- [ ] **BE-001 Backend monolith decomposition:** split active oversized config/handler/service files by bounded domain after behavior is green.
- [ ] **CFG-001 Strict configuration:** reject unknown keys with path/suggestion while supporting explicit deprecations.
- [ ] **FE-006 Bundle budgets:** gate initial, vendor, total, gzip/Brotli, and route-specific budgets; prove forbidden roots emit no chunks.

### Phase 7 — Dependencies, migrations, governance, final strict review

- [ ] **DEP-001 DOMPurify:** upgrade direct/transitive sanitizer versions and add mutation-XSS, URI, iframe, template, prototype and cross-realm regressions at every active `v-html` sink.
- [ ] **DEP-002 XLSX:** replace or isolate abandoned `xlsx` before exception expiry; preserve export-only behavior and test hostile inputs if import is retained.
- [ ] **DEP-003 Remaining advisories:** upgrade/remove reachable Mermaid/UUID/YAML/PostCSS dependencies and document non-reachable residuals with expiries/owners.
- [ ] **DEP-004 Go module advisories:** update safe transitive versions and rerun `govulncheck`, tests, vet and race-sensitive packages.
- [ ] **MIG-001 Migration IDs:** enforce globally unique monotonic identifiers and immutable historical checksums.
- [ ] **MIG-002 Upgrade matrix:** test every supported upgrade path from the oldest supported schema in disposable PostgreSQL.
- [ ] **GOV-001 License/CLA identity:** reconcile CLA/DCO authority and fork ownership; add third-party notices/license inventory.
- [ ] **REV-001 Full-code strict review:** inspect all active backend/frontend/deploy/CI code, route/capability matrices, secret flows, outbound clients, workers, migrations, generated assets, and tests after fixes.
- [ ] **REV-002 Independent closure review:** fresh reviewers must adjudicate every issue ID; no unresolved `REVISE` result.
- [ ] **REV-003 Final clean gates:** backend tests/vet/race-relevant tests, frontend lint/typecheck/full uncached Vitest/coverage/build/bundle/a11y, vulnerability scans, Compose rendering, shell checks, installer sandbox, `git diff --check`, and secret scan.
- [ ] **REV-004 No-deployment verification:** confirm live image/container/service state was not changed by this campaign.

## External/disposable blockers to track, not fake

- Actual restore, purge, key rotation, PostgreSQL/Redis interruption, migration downgrade/upgrade matrix, and dependency-failure tests require isolated disposable infrastructure.
- S3, SMTP, OAuth/OIDC, payments, provider refresh, and additional AI-provider flows require sandbox configuration/credentials.
- If unavailable, mark `DEFERRED-BLOCKED` with exact required resources and keep repository-level fail-closed contracts.

## Phase log

Append an entry after every phase containing:

- issue IDs addressed and dispositions;
- RED command/output summary;
- implementation scope;
- GREEN/focused and broad gate results;
- changed files;
- independent review verdict;
- confirmation that nothing was deployed.
