# Independent Audit Adjudication

Date: 2026-07-13

Three independent read-only auditors reviewed frontend/product surface, backend/security/workers, and data/performance/deployment. Their complete raw summaries are stored under `/home/opc/.hermes/cache/delegation/` and contain no secrets.

## Frontend/product audit — accepted additions

Accepted and incorporated:

1. **Customer Profile remains reachable** from `AppHeader.vue` in private mode. It loads customer OAuth/provider, support, balance-notification, password, and TOTP surfaces. Recommendation: remove route/link and move required administrator password/TOTP controls into operator Security.
2. **Batch Image discovery remains active** from Dashboard and Sidebar through `useBatchImageAccess()`, can page API keys, and can reveal a quick action. Recommendation: disable discovery/action unless explicitly retained.
3. Risk/Backup/Proxies and the canonical retained-route manifest are inconsistent.
4. Existing tests positively encode the wrong contracts for Security, Email, Profile, and stale prefetch routes.
5. Customer auth/recovery/payment/profile chunks remain emitted even when top-level private navigation hides them.

These strengthen the existing product-boundary High finding rather than creating unrelated duplicates.

## Backend/security audit — accepted additions

Accepted and incorporated:

1. **Host-based local-admin trust is deployment-fragile.** Source logic can issue administrator tokens when a spoofed local Host and private proxy `RemoteAddr` are accepted.
2. Payment-order worker remains active in private mode.
3. Scheduled test runner can overlap locally and duplicate across instances because no atomic claim/lease precedes execution.
4. Backup S3 endpoint lacks SSRF validation/destination pinning.
5. Grok OAuth does not require state for bare codes and permits redirect override.
6. Gemini redirect derivation trusts Origin/forwarded headers outside trusted-proxy policy.

### Critical-severity adjudication

The backend reviewer rated Host spoofing Critical under a catch-all reverse proxy assumption. That assumption does not match the active deployment:

- Public Nginx proxies only explicit gateway compatibility locations.
- `/api/*` and `/admin/*` fall through to Nginx `404` before the app.
- State-free live checks against `/admin/dashboard` produced an identical Nginx 153-byte `404` for normal Host, `Host: localhost`, and WireGuard Host.
- The app port is bound only to loopback and WireGuard.

Therefore:

- **Current deployed exploit:** not demonstrated; Critical rejected.
- **Application design:** High, because a future catch-all ingress/proxy or untrusted private path would make the flaw Critical.
- **Operational constraint:** do not broaden Nginx proxying or app-port exposure before replacing Host-based trust.

No POST to local-admin was issued, so no token or Redis session state was created during verification.

## Data/performance audit — accepted additions

Accepted and incorporated:

1. No S3 backup configuration, schedule configuration, or backup records exist.
2. Redis holds authentication/session/scheduler state and has `maxmemory=0` with `noeviction`; key metadata shows substantially more misses than hits.
3. Deployed binary/image provenance lacks an immutable Git revision despite frontend assets matching a clean current-source build.
4. Index inventory is large relative to the small database; collect a representative observation window before removal.
5. Integration coverage remains skipped/environment-dependent for provider E2E, Docker/testcontainers, backup/restore, and dependency failure.

### Backup encryption clarification

The independent summary mentioned encryption support generally. Direct source review confirms:

- S3 Secret Access Key is encrypted.
- Database backup payload is only gzip-compressed.
- `S3BackupStore.Upload()` calls `io.ReadAll` and uploads the plaintext gzip object.
- Restore directly gzip-decompresses the object into `psql`.

Thus the original dump-confidentiality finding remains valid.

## Final report review pair

A second independent pair reviewed the completed report itself. Both returned **REVISE**.

### Product/frontend reviewer — incorporated

- Added directly reachable `/key-usage`, `/custom/:id`, `/monitor`, `/admin/channels/pricing`, and `/admin/channels/monitor` surfaces.
- Added a complete private/public ingress negative matrix for auth, setup, callback, payment, and legal roots.
- Reclassified nested UI as Moderate product-boundary/future-reactivation risk while retaining P0/P1 product priority.
- Replaced the coarse runtime-only capability proposal with a versioned default-deny contract enforced server-side, at build time, and at runtime with bidirectional absence tests.

### Security/operations reviewer — incorporated

- Corrected XLSX framing: current use generates exports; no ingestion path was found, so malicious-file parsing risk was unsupported.
- Expanded secret inventory to settings credentials/provider keys and the database-persisted JWT signing root.
- Corrected cryptographic design: keyed digest/verifier for gateway authentication keys; reversible envelope encryption only for credentials that require replay; external/wrapped roots.
- Credited payment worker timeout/graceful-stop/leader-lock controls while retaining unnecessary-startup finding.
- Changed Batch Image/quota from presumed dormant to explicit product decisions.
- Separated severity from remediation priority and reordered the backlog around route contract, forbidden workers, key/backup design, product enforcement, and then dependency/schema/hardening work.

The reviewers did not re-review the amended draft. Their requested independent review is complete; their substantiated corrections are incorporated and traceable in this artifact.

## Final disposition

The independent audits materially improved the report. The final release posture remains:

- No emergency shutdown based on current sampled boundaries.
- Hold expansion/cleanup-complete declaration.
- Do not modify public proxy/app-port exposure until local-admin trust is replaced.
- Prioritize route-contract unification, forbidden worker shutdown, complete key/backup design, product-capability enforcement, then sink-specific dependency and schema/container hardening.
