# Phase log

## 2026-09-01 — Phase 2.1 provider-error presentation boundary

Issues: provider-body UI exposure (frontend portion of `SEC-005`/`L10N-004`); backend
storage path independently reviewed and retained because it already sanitizes and
truncates bodies before persistence.

RED:

```text
pnpm exec vitest run src/components/user/__tests__/UserErrorDetailModal.spec.ts \
  src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts
  2 failed: raw provider body was rendered; user modal did not fetch on initial show.
```

Implementation:

- Added `errorBodySummary` allowlist helper that emits only format, bounded length,
  status and machine-readable type/code metadata.
- User and operator error-detail modals render the summary and an explicit localized
  redaction notice; raw response text is never inserted into the DOM.
- Made the user detail watcher immediate so opening an already-selected modal loads
  its detail.
- Added English and Chinese copy plus focused negative-leak tests and a checkout-local
  evidence note under `tmp/`.

GREEN:

```text
Focused frontend tests: 5 tests PASS
Full frontend Vitest: 264 files / 1,421 tests PASS
Frontend typecheck: PASS
Frontend build: PASS
Bundle and private-bundle budgets: PASS
Changed-file ESLint: PASS
Backend Ops sanitization focused suite: PASS
Release contract and upstream lock checks: PASS
```

Independent review: backend review found no additional response-boundary storage bug;
frontend implementation reviewed and committed as `403a6a928`.

Remote verification for the resulting documentation SHA `1f02097ca`:

- GitHub CI run `33559437114`: all jobs passed (backend unit/integration/race,
  frontend, golangci-lint, shell, and contracts).
- GitHub Security Scan run `33559437112`: passed.

Deployment: **NOT DEPLOYED**. No production service, database, Redis, or OPC files
were modified.

## 2026-07-15 — Phase 0 baseline

- Goal ledger created: `.hermes/plans/2026-07-15-exapi-full-remediation-goal.md`.
- Baseline stored in `baseline.yml`.
- Live no-deploy sentinel recorded.
- No service or production-data mutation.

## 2026-07-15 — Phase 1A local-admin source trust

Issues: `SEC-001` (partial; code boundary fixed, deployment parity continues).

RED:

```text
GOTOOLCHAIN=auto go test -tags=unit ./internal/handler -run '^TestIsLocalAdminBypassRequest$' -count=1
```

Failed as expected for forged localhost from Docker bridge, forged loopback from RFC1918 LAN, trusted-looking WireGuard Host from Docker bridge, and configured source denied by unrelated Host.

Implementation:

- Authorization now trusts only socket loopback or `SUB2API_LOCAL_ADMIN_BYPASS_CIDRS` source membership.
- Host can restrict direct loopback access but cannot grant source trust.
- Added adversarial source/Host matrix.

GREEN:

```text
GOTOOLCHAIN=auto go test -tags=unit ./internal/handler -run '^(TestIsLocalAdminBypassRequest|TestLocalAdminBypassEnabled)$' -count=1
ok
```

Independent review: PASS. Reviewer confirmed socket-peer-only trust, Host cannot grant authorization, configured CIDRs are explicit source trust, forwarded headers are ignored, and focused tests/diff hygiene pass.

Deployment: none.

## 2026-07-15 — Phase 1B fail-closed backend product guard

Issues: `PM-001`, `PM-002`, `SUP-009` (partial).

RED:

```text
GOTOOLCHAIN=auto go test ./internal/config ./internal/server/middleware -run '^(TestSingleUserPrivateControlPlaneEnabled|TestSaaSFeaturesEnabled|TestPublicControlPlaneGuard)' -count=1
```

Failed as expected because empty/unknown product mode selected standard SaaS behavior and missing/unknown Host bypassed the control-plane guard.

Implementation:

- Private single-user mode now fails closed; only explicit `0/false/no/off` selects legacy SaaS mode.
- Control plane is default-deny for all Hosts except `SUB2API_PRIVATE_CONTROL_HOSTS`.
- Gateway routes remain available on non-private/public Hosts.
- Added explicit private-host allowlist tests and unknown/missing-host regressions.
- Forwarded product mode, public host, private control hosts and local-admin source policy through main/local/standalone/dev Compose manifests.
- Updated `.env.example` to private fail-closed defaults.

GREEN:

```text
GOTOOLCHAIN=auto go test ./internal/config ./internal/server/middleware -run '^(TestSingleUserPrivateControlPlaneEnabled|TestSaaSFeaturesEnabled|TestPublicControlPlaneGuard)' -count=1
ok
```

Compose rendering passed for main, local, standalone and dev manifests using synthetic non-secret test values. `git diff --check` passed.

No-deploy sentinel after changes:

```text
image=sha256:3942f6908505fd7af19238b26a6102ddeab8696c53616b20380f3965661f5304
started=2026-07-15T01:44:15.758049584Z
restarts=0
```

Deployment: none.

## 2026-07-15 — Phase 1C authoritative browser product mode

Issues: `PM-003`, `PM-004` (authoritative mode transport; route/build enforcement continues).

RED:

```text
GOTOOLCHAIN=auto go test ./internal/handler/dto -run '^TestPublicSettingsInjectionPayload_SchemaDoesNotDrift$' -count=1
```

Failed as expected after adding `single_user_private_control_plane` to the public DTO because SSR injection did not carry the field.

Implementation:

- Added `single_user_private_control_plane` to API and SSR public-settings payloads.
- Both payloads derive the value from the backend fail-closed product-mode function.
- Browser private-mode logic now uses the injected/runtime backend contract instead of hostname inference.
- Missing or stale injection fails closed to private mode; only explicit `false` activates legacy SaaS behavior.
- Store fallback public settings also fail closed.
- Added tests for explicit private/standard values and missing/stale injection.

GREEN:

```text
Backend focused schema/config/guard tests: PASS
Backend focused go vet: PASS
Frontend typecheck: PASS
Frontend focused tests: 3 files, 31 tests PASS
```

Independent review: dispatched; closure pending.

Deployment: none.

## 2026-07-15 — Phase 1D private route registration allowlist

Issues: `PM-004` (route-registration portion), `PM-005` (partial build-graph pruning).

Implementation:

- Expanded the explicit operator route contract to include active channel, proxy and risk-control surfaces.
- Added explicit public and compatibility-redirect route lists.
- Router registration now filters the full legacy route table through this allowlist when authoritative product mode is private.
- Dormant registration, password recovery, OAuth callbacks, payment pages, customer profile/dashboard, custom pages and user-facing legacy routes are no longer registered in the private router.
- The catch-all route remains registered so removed deep links resolve through the private 404 surface.
- Added an exact full-route-matrix regression rather than sampling two routes.

GREEN:

```text
Frontend typecheck: PASS
Focused private-mode suite: 5 files, 36 tests PASS
```

Note: legacy route source definitions and lazy-import expressions remain in `router/index.ts`; registration is pruned, but full private build-graph elimination is still open under `PM-005`.

Deployment: none.

## 2026-07-15 — Phase 1E loopback-only deployment defaults

Issues: `SEC-001` (default exposure portion), `SUP-009` (bind parity portion).

Implementation:

- Main, local, standalone and development Compose manifests default host publication to `127.0.0.1` rather than `0.0.0.0`.
- Installer and systemd unit default the application listener to `127.0.0.1`.
- `.env.example` documents loopback as fail-closed and requires an exact WireGuard/private interface address for remote administration.
- Added `deploy/test-bind-defaults.sh` to reject future wildcard-binding regressions.

GREEN:

```text
Shell syntax checks: PASS
Bind-default contract test: PASS
Four rendered Compose manifests: host_ip=127.0.0.1 PASS
systemd unit syntax: PASS (expected absent live binary ignored)
```

Deployment: none.

## 2026-07-15 — Phase 2A gateway API-key verifier foundation

Issues: `SEC-003`, `SEC-005` (foundation only; repository dual-read/write and migration remain open).

Implementation:

- Added a dedicated fail-closed `gatewayAPIKeyDigester` using the external gateway-digest keyring and purpose `gateway.api_key`.
- Added structural digest recognition without exposing key material.
- Added tests for missing keyring rejection, one-way storage, wrong-key rejection, purpose binding and retained-key rotation.
- Extended the API-key schema/service model with nullable `key_digest` and non-secret `key_prefix`.
- Generated Ent bindings and added additive migration `212_api_key_hmac_verifiers.sql`.
- Migration is deliberately additive: no live migration was executed and legacy plaintext is not deleted before dual-read verification/rollback evidence exists.

GREEN:

```text
Focused secretcrypto/repository/service tests: PASS
Focused go vet: PASS
git diff --check: PASS
```

Deployment/database migration: none.

## 2026-07-15 — Phase 2B API-key secure new writes and mixed-row authentication

Issues: `SEC-003` (new-write and dual-read portion), `SEC-005` (mixed-row portion).

RED/GREEN implementation:

- Production `NewAPIKeyRepository` now fails startup when the gateway digest keyring is absent or invalid.
- New API-key rows store a versioned HMAC verifier plus a non-secret prefix; the legacy NOT NULL key column receives only a deterministic non-secret placeholder during the rollback window.
- Authentication and existence checks query by HMAC verifier, with an explicit legacy fallback only when `key_digest IS NULL`.
- Repository results clear the stored key field; only the request candidate is attached to the ephemeral service auth object/cache snapshot.
- Added disposable PostgreSQL integration coverage proving raw material is absent from new rows, correct/wrong-key behavior, existence checks, and mixed legacy/digest authentication.
- Regenerated Ent and Wire bindings.

GREEN:

```text
Disposable PostgreSQL mixed-row integration: PASS
Focused repository/service/middleware/server tests: PASS
Focused go vet: PASS
git diff --check: PASS
```

Open before `SEC-003` closure: backfill command, deleted-key audit redesign, cache invalidation without raw-key lists, API/list response redaction, and rollback/migration tests.

Deployment/database migration: none.

## 2026-07-15 — Phase 2C one-time API-key disclosure and response redaction

Issues: `SEC-003` (response/list redaction portion).

Implementation:

- Generic API-key DTO mapping never copies reusable `Key`; list/get/update/embedded-user/usage responses expose only `key_prefix`.
- The create handler explicitly attaches the raw key to the successful creation response exactly once.
- `key` now uses `omitempty`; `key_prefix` is a first-class DTO/frontend field.
- Entity-to-service mapping clears `Key` globally and derives a safe prefix for legacy rows whose prefix has not yet been backfilled.
- Frontend typing preserves the current string shape for compatibility while documenting that non-create responses contain an empty key.

GREEN:

```text
DTO redaction regression: PASS
Disposable PostgreSQL list/get/auth redaction integration: PASS
Frontend typecheck: PASS
Focused backend tests and go vet: PASS
git diff --check: PASS
```

Open: private UI must remove post-creation “use/copy/import” actions that assumed keys were recoverable; deleted-key audit/cache invalidation migration remains.

Deployment/database migration: none.

## 2026-07-15 — Phase 1F independent-review correction: Host spoofing

Issues: `PM-002`, `SEC-002`, `SUP-009`.

Independent review verdict was `REVISE`: private control access still trusted an allowlisted HTTP Host without authenticating the socket peer. The review predated the later loopback bind fix, but the structural Host-only authorization defect remained valid.

RED:

```text
remote RemoteAddr + Host localhost/127.0.0.1/WireGuard IP => expected 404, observed 204
```

Implementation:

- Private control-plane access now requires both an allowlisted Host and a socket peer that is loopback or inside `SUB2API_PRIVATE_CONTROL_CIDRS`.
- Forwarding headers are not consulted.
- Added spoofed-Host regressions and an intended WireGuard-peer positive test.
- Forwarded the new CIDR variable through all Compose manifests and documented it in `.env.example`.
- Existing loopback-only bind defaults remain in force.

GREEN:

```text
Focused guard/config tests: PASS
Focused go vet: PASS
Four Compose renders: PASS
git diff --check: PASS
Independent follow-up review: PASS — Host+socket-peer boundary confirmed; IPv4/IPv6/mapped-address parsing and malformed inputs fail closed; forwarding headers are ignored.
```

Operational caveat: do not trust a public reverse-proxy CIDR unless that proxy independently enforces the private Host/source boundary. Prefer direct loopback or WireGuard access.

Deployment: none.

## 2026-07-15 — Phase 2D unrecoverable-key frontend contract

Issue: `SEC-003` (private UI compatibility with one-time disclosure).

RED:

- Added a KeysView regression requiring persisted/listed keys to expose no use/import actions.
- The focused test showed both actions were rendered. Existing test harness setup also lacked current Pinia/onboarding dependencies; repaired the harness before adjudicating behavior.

Implementation:

- Removed post-creation “use key” and CC-Switch import actions, modal state, deep-link construction, and dead imports from `KeysView.vue`.
- Removed the copy button for persisted rows. The key column now displays only backend-provided `key_prefix` and never copies an empty/recoverable `key` field.
- Creation remains the sole raw-key disclosure path.

GREEN:

```text
Focused unrecoverable-key UI regression: PASS
Frontend typecheck: PASS
git diff --check: PASS
```

Known baseline: the remainder of the older KeysView suite has unrelated harness/stubbing failures when run wholesale; the focused new regression is green and typecheck is green. Full frontend gate repair remains tracked in Phase 6.

Deployment: none.

## 2026-07-15 — Phase 2E deleted-key audit verifier migration

Issues: `SEC-003`, `SEC-005` (deleted-key attribution portion).

RED/GREEN:

- Replaced the old integration expectation that deletion audits retain the raw key with a regression requiring a verifier and no reusable key material.
- Extended migration 212 with nullable `deleted_api_key_audits.key_digest` and a partial index.
- New protected-key deletion copies only the HMAC verifier and existing non-secret placeholder into the audit row.
- Deleted-key attribution computes the candidate verifier and queries it, with a legacy raw fallback only when `key_digest IS NULL`.
- Ops repository startup now fails closed on missing gateway-digest keyring, matching API-key repository behavior.
- Regenerated Wire bindings and updated disposable test constructors.

GREEN:

```text
Disposable PostgreSQL protected deletion audit: PASS
Disposable PostgreSQL deleted-key verifier lookup/latest selection: PASS
Focused repository/server tests: PASS
Focused go vet: PASS
git diff --check: PASS
```

Open: migrate existing plaintext API-key and deletion-audit rows transactionally, then erase raw legacy values after rollback/rotation validation.

Deployment/database migration: none.

## 2026-07-15 — Phase 2F transactional legacy key backfill core

Issues: `SEC-003`, `SEC-005` (disposable migration core; production invocation remains intentionally absent).

RED:

- Added disposable PostgreSQL tests requiring API-key and deleted-audit plaintext rows to be rewritten together, remain verifiable, be idempotent, and roll back earlier rewrites if a later malformed row fails.
- Tests initially failed because no migration implementation existed.

Implementation:

- Added row-locking (`FOR UPDATE`) migration over `api_keys` and `deleted_api_key_audits` where `key_digest IS NULL`.
- Each row is converted to a purpose-bound HMAC verifier, non-secret placeholder, and display prefix where applicable.
- Every conditional update must affect exactly one row; malformed/empty material fails closed.
- Savepoint wrapper guarantees invocation-level rollback inside a caller-owned transaction.
- Re-running after success migrates zero rows.

GREEN:

```text
Disposable migration success/preservation: PASS
Disposable idempotency: PASS
Disposable failure rollback: PASS
Protected create/delete/lookup regressions: PASS
Focused repository/service/server tests: PASS
Focused go vet: PASS
git diff --check: PASS
```

Safety gate: this is only the migration core. No startup auto-migration or production command was wired; an explicit operator command, dry-run/reporting, backup gate, and key-rotation/rollback rehearsal remain required before any live use.

Deployment/database migration: none.

## 2026-07-15 — Phase 2G retained-key rotation lookup

Issues: `SEC-003`, `SEC-005` (digest-key rotation correctness).

RED:

- Added a test requiring rotated keyrings to produce lookup candidates for both active and retained keys; it failed because only the active verifier could be derived.
- This exposed a correctness defect: rows written before active-key rotation could verify if already loaded, but indexed database lookup could not find them.

Implementation:

- Added deterministic `DigestCandidates`: active key first, retained key IDs sorted.
- API-key authentication/existence queries now use all active+retained verifier candidates.
- Deleted-key attribution uses the same candidate set through PostgreSQL `ANY`.
- New writes and migrations still use only the active key.

GREEN:

```text
Retained+active candidate unit tests: PASS
Disposable pre-rotation API-key authentication after rotation: PASS
Disposable existence lookup after rotation: PASS
Migration/create/delete/audit regressions: PASS
Focused secretcrypto/repository/service/server tests: PASS
Focused go vet: PASS
git diff --check: PASS
```

Operational contract: old digest roots must remain in the keyring until every stored verifier has been deliberately re-digested from still-available raw input or expired/deleted; removing a retained key earlier correctly makes those rows unauthenticatable.

Deployment/database migration: none.

## 2026-07-15 — Phase 2H fail-closed Redis authentication

Issue: `SEC-004`.

RED:

- Added private-mode config validation requiring `redis.password`; it failed because passwordless Redis was accepted.
- Added deployment contract checks requiring bundled Compose paths to refuse an empty password.

Implementation:

- Private single-operator backend validation now rejects an empty Redis password.
- Explicit legacy standard mode retains passwordless compatibility.
- Main and local bundled Redis Compose paths require `REDIS_PASSWORD`, always start Redis with `--requirepass`, and pass the same secret to application and healthcheck through environment variables.
- Updated `.env.example` and added `deploy/test-redis-auth.sh`.
- Standalone/dev manifests continue to pass configured external Redis credentials; they do not own the external Redis server policy.

GREEN:

```text
Private-mode passwordless rejection: PASS
Explicit standard-mode compatibility: PASS
Redis deployment contract: PASS
Four Compose renders with synthetic secrets: PASS
Focused config tests/vet: PASS
git diff --check: PASS
```

No production Redis password was changed and no Redis process/container was restarted.

Deployment/config mutation: none.

## 2026-07-15 — Phase 2I account-credential AEAD foundation and protected creates

Issues: `SEC-004`, `SEC-005` (new-create path and cryptographic foundation; update/migration paths remain open).

RED:

- Added purpose-binding, round-trip, malformed-envelope, and legacy-classification tests; they failed because no account-credential protector existed.
- Added disposable persistence tests requiring account creation to store no replayable token and to roll back if protection is unavailable.

Implementation:

- Added canonical JSON AES-256-GCM envelopes rooted in the external data-encryption keyring and bound to `accounts/<id>/credentials`.
- Envelopes have a single reserved JSON field; malformed/mixed shapes fail closed.
- Production account repository construction now requires the external data-encryption keyring.
- Account creation is transactional: create placeholder row to obtain ID, seal credentials with row-bound purpose, update within the same transaction, then commit.
- Repository account reads open protected rows and remain able to classify legacy plaintext for an explicit migration window.
- Usage-log account summaries use a redacted mapper and never decrypt/expose credentials.

GREEN:

```text
Protector round-trip/purpose/malformed tests: PASS
Disposable encrypted-create DB assertion: PASS
Disposable read-decrypt assertion: PASS
Disposable missing-protector rollback: PASS
Focused repository/server tests: PASS
Focused go vet: PASS
git diff --check: PASS
```

Open before `SEC-004` closure: transactional legacy backfill, controlled read-rewrap behavior, response redaction review, and rotation/rollback tests.

Deployment/database migration: none.

## 2026-07-15 — Phase 2J protected account credential updates

Issues: `SEC-004`, `SEC-005` (repository update paths).

Detected before implementation:

- Full account updates, token-refresh `UpdateCredentials`, and bulk credential merges still persisted plaintext JSON despite protected creates.
- Bulk updates could not merge directly into an encrypted JSON envelope and required row-specific decrypt/merge/reseal behavior.

Implementation:

- `Update` and `UpdateCredentials` seal the complete credential map with the account-ID-bound AEAD purpose before persistence.
- Bulk credential updates load/decrypt each target, merge the patch in memory, produce a distinct row-bound envelope, and apply all envelopes through one SQL `CASE id` update.
- Missing protectors fail closed before database mutation.

Disposable GREEN:

```text
Create encrypted/read decrypted: PASS
Token-refresh credential update encrypted: PASS
Full account update encrypted: PASS
Bulk merge preserves existing fields and encrypts patch: PASS
Missing-protector create rollback: PASS
Existing bulk-update regressions: PASS
Focused repository/service/server tests: PASS
Focused go vet: PASS
git diff --check: PASS
```

Open: operator migration command/dry-run, response-redaction confirmation, and review of direct Ent writes outside the repository boundary.

Deployment/database migration: none.

## 2026-07-15 — Phase 2K transactional legacy account-credential backfill

Issues: `SEC-004`, `SEC-005` (migration core and retained-key behavior).

RED:

- Added disposable tests for mixed plaintext/protected rows, idempotence, malformed-envelope rollback, and caller-transaction usability; the migration API did not exist.

Implementation:

- Added a caller-owned-transaction migration core protected by a savepoint.
- Locks all active account rows, classifies protected versus legacy credential JSON, and seals only rows requiring rewrite.
- Conditional updates compare the original JSONB value to detect concurrent mutation.
- Any malformed envelope or conflict rolls back every rewrite made by the invocation while preserving the caller transaction.
- Added retained-old-key read/rewrap unit coverage; no automatic read-side mutation occurs.

Disposable GREEN:

```text
Mixed legacy/protected rows: PASS
Plaintext absent after rewrite: PASS
Decrypted semantic equality: PASS
Idempotent second run: PASS
Malformed-envelope full rollback: PASS
Caller transaction remains usable: PASS
Retained-key read and explicit rewrap: PASS
Protected create/update/bulk regressions: PASS
Focused go vet: PASS
git diff --check: PASS
```

Safety gate: migration remains an unexported core with no startup hook or production command. Operator dry-run/reporting, backup gate, and explicit invocation remain required before any live use.

Deployment/database migration: none.

## 2026-07-15 — Phase 2L raw-key-free auth-cache invalidation

Issues: `SEC-003`, `SEC-005` (cache invalidation).

RED:

- Added a regression that makes raw-key listing fail if user/group invalidation calls it; the old implementation immediately invoked `ListKeysByUserID`.

Implementation:

- Auth-cache keys now include a process-local generation prefix before the SHA-256 raw-key fingerprint.
- User/group invalidation advances the generation and clears L1 instead of enumerating reusable keys.
- Existing L2 entries become unreachable and expire under their bounded TTL; no Redis key scan is required.
- A non-secret global invalidation Pub/Sub marker advances generations and clears L1 across instances.
- Single-key invalidation remains exact for operations that possess the submitted ephemeral key.

GREEN:

```text
No raw-key user/group enumeration: PASS
Generation changes on user invalidation: PASS
Generation changes on group invalidation: PASS
Focused API-key cache tests: PASS
Focused service/repository/server compile: PASS
Focused go vet: PASS
git diff --check: PASS
```

Open: remove the obsolete raw-key list methods/callers elsewhere in admin group flows and adapt their tests before deleting the repository interface methods.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2M removal of raw-key listing APIs

Issues: `SEC-003`, `SEC-005` (remaining callers and repository boundary).

Implementation:

- Group deletion now performs group-generation invalidation after the transactional delete; it does not prefetch keys.
- User-group replacement now performs user-generation invalidation after commit.
- Removed `ListKeysByUserID` and `ListKeysByGroupID` from the production `APIKeyRepository` interface and repository implementation.
- Updated the deletion regression to require group-ID invalidation and zero exact-key invalidations.

GREEN:

```text
Group deletion without raw-key listing: PASS
User/group generation invalidation: PASS
Admin API-key group update regressions: PASS
Focused service/server/middleware/repository tests: PASS
Focused package compilation: PASS
Focused go vet: PASS
git diff --check: PASS
```

Residual text matches are dead methods on test doubles left temporarily permissible by Go's structural typing; production interfaces and implementations no longer expose raw-key listing.

Deployment/runtime mutation: none.

## 2026-07-15 — Broad backend gate checkpoint after Phase 2M

Broad `go test ./...` and `go test -tags=unit ./...` were run without deployment. They exposed expected test-fixture incompatibilities introduced by fail-closed secret requirements plus one private-mode route expectation:

- config tests omitted the now-required Redis password;
- repository tests used unsecured constructor helpers without test keyrings;
- a legacy registration rate-limit test expected a SaaS route that private mode correctly omits.

Fixed the shared config fixture and bootstrap test to use a synthetic test-only Redis password. `go test ./internal/config -count=1` is now PASS.

Remaining broad-gate remediation was completed in the next checkpoint; no failure was hidden or treated as success.

Deployment/runtime mutation: none.

## 2026-07-15 — Broad backend gates restored after fail-closed fixture migration

Remediation:

- Added synthetic test-only digest/data-encryption roots to SQLite and parameter-limit repository fixtures.
- Marked the registration rate-limit contract as explicit legacy standard mode; private mode continues to omit registration.
- Updated API contracts for one-time create-key disclosure plus `key_prefix`, and list omission of `key`.
- Updated cache-invalidation tests to assert generation changes and the non-secret Pub/Sub marker instead of exact raw-key deletion.

GREEN:

```text
GOTOOLCHAIN=auto go test ./...: PASS
GOTOOLCHAIN=auto go test -tags=unit ./...: PASS
Full config package: PASS
Focused route/repository contracts: PASS
```

No fail-closed production requirement was relaxed to obtain green tests; only test fixtures and mode-specific expectations changed.

Deployment/runtime mutation: none.

## 2026-07-15 — SEC-006 disposable Redis authentication closure

Executed a short-lived Redis 8 container with `--network none`, no host ports, and a synthetic one-use test password. The container was removed by an EXIT trap.

```text
Unauthenticated PING: NOAUTH Authentication required
Authenticated PING: PONG
Network mode: none
Host ports: none
```

Combined with the backend fail-closed validation, required Compose interpolation, unconditional `--requirepass`, authenticated healthcheck, four-manifest rendering, and `deploy/test-redis-auth.sh`, `SEC-006` is closed for repository-owned Redis deployment paths. External Redis servers remain operator-managed and are validated only at the client credential boundary.

No production Redis instance, password, volume, or configuration was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2N S3 credential fail-closed persistence

Issues: `SEC-005`, `SEC-007`, `SEC-008`.

RED:

- Added a test proving that a stored plaintext S3 Secret Access Key was silently accepted and used after decryption failed.
- The focused test failed because `loadS3Config` returned the plaintext value.

Implementation:

- Stored S3 credentials now fail closed as corrupt secret-bearing configuration when decryption fails.
- Metadata-only S3 updates now decrypt and re-encrypt the retained secret before persistence. This fixes a second defect where the previous implementation copied decrypted plaintext back into the settings row.
- The retention test now asserts the persisted settings value remains encrypted.

GREEN:

```text
Plaintext stored credential rejection: PASS
New S3 credential encryption: PASS
Metadata-only retained-secret encryption: PASS
Encrypted backup create/restore/tamper rejection: PASS
Delete/retention failure metadata preservation: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live settings, S3 object, backup, database, or service was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2O retention partial-failure consistency

Issues: `SEC-007`, `SEC-008`.

RED:

- Added per-object failure injection and a two-object retention test.
- The older implementation deleted the first object, failed on the second, returned immediately, and persisted neither metadata change. The deleted object's stale record therefore survived.

Implementation:

- Retention now tracks each successful deletion.
- On a later deletion failure, the failed and unattempted records are retained.
- Successfully deleted records are removed from metadata before returning the S3 error.
- If metadata progress cannot be persisted, the persistence error is returned explicitly.

GREEN:

```text
Partial-success/later-failure consistency: PASS
First-delete failure keeps metadata: PASS
Manual delete failure keeps metadata: PASS
Successful manual delete removes object/metadata: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live S3 object, backup record, database, or service was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2P synchronous backup orphan prevention

Issues: `SEC-007`, `SEC-008`.

RED:

- Tightened the terminal-persistence regression to require no upload before an initial metadata record is durable.
- The old synchronous path uploaded a completed encrypted object first, then failed its only metadata write, producing an undiscoverable orphan.

Implementation:

- Synchronous backup now persists a `running`/`pending` record before dump, encryption, or upload.
- Initial persistence failure returns before object creation.
- Completion clears progress and updates the existing durable record.
- If the completion update fails, the prior durable running record remains available for startup reconciliation; the valid encrypted object is not silently deleted.

GREEN:

```text
Initial metadata failure prevents upload: PASS
Synchronous encrypted backup: PASS
Dump failure contract: PASS
Stop/admission atomicity: PASS
Asynchronous backup regression: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live S3 object, backup record, database, or service was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2Q backup metadata/object referential integrity

Issues: `SEC-007`, `SEC-008`.

RED:

- Added a 101-record regression with every completed record referencing an object key.
- The prior hard cap silently removed the oldest metadata row without deleting its object, creating a permanent orphan outside explicit retention policy.

Implementation:

- Removed implicit history truncation from `saveRecord`.
- Backup metadata is now removed only through explicit delete/retention flows that coordinate object deletion and preserve failed-deletion records.
- Retention settings remain the bounded lifecycle mechanism instead of an unrelated metadata-only cap.

GREEN:

```text
101 restorable records preserved: PASS
Concurrent record writes: PASS
Retention contracts: PASS
Manual deletion contracts: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live S3 object, backup record, database, or service was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2R backup key rotation compatibility

Issues: `SEC-005`, `SEC-007`.

Added an end-to-end staging regression proving the backup keyring rotation contract:

- an object encrypted with the former active backup key decrypts after rotation while that key remains retained;
- new objects after rotation embed and use only the new active key ID;
- gzip payload semantics survive old-key restore;
- tampering, missing keyrings, staging limits, and mode-0600 contracts remain green.

GREEN:

```text
Retained-key old backup restore: PASS
New-active-key backup write: PASS
Backup staging round trip/permissions: PASS
Tamper rejection: PASS
secretcrypto stream/envelope suite: PASS
Focused service/crypto vet: PASS
git diff --check: PASS
```

Operational constraint: a retained backup key cannot be removed until every backup encrypted under it is deleted or explicitly re-encrypted and verified. No automatic backup-object rotation was introduced.

No live keyring, backup object, database, or service was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2S PostgreSQL backup subprocess error hardening

Issues: `SEC-005`, `SEC-007`.

Detected during strict backup adapter review:

- restore failures returned raw combined `psql` output through service/handler error paths;
- the dump reader wrapper assumed non-nil process/reader state, making isolated failure handling brittle.

Implementation:

- Added injectable command construction for deterministic subprocess regressions without touching PostgreSQL.
- `psql` failure responses now retain the process exit error but discard raw command output, preventing server diagnostics or echoed SQL from reaching API errors.
- `cmdReadCloser.Close` is nil-safe while still waiting for a real `pg_dump` process and surfacing late producer failures.

GREEN:

```text
Restore subprocess output redaction: PASS
Dump-reader producer error propagation: PASS
Backup staging regressions: PASS
Focused repository/service vet: PASS
git diff --check: PASS
```

No database process, live backup, or service was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2T disposable PostgreSQL dump/restore transaction proof

Issues: `SEC-007`.

Executed PostgreSQL 18 in a short-lived container with `--network none`, no host ports, no production volumes, and a synthetic one-use password. The first readiness attempt exposed an initialization race (`source_db` not yet created); the corrected harness waited on an actual SQL query before mutation.

```text
pg_dump source -> psql target: PASS
Restored table/value query: PASS
Invalid restore statement exits nonzero: PASS
Earlier statement in failed --single-transaction restore rolled back: PASS
Network mode: none
Host ports: none
Disposable container cleanup: PASS
```

This proves the concrete `pg_dump --clean --if-exists` / `psql --single-transaction` contract used by the adapter without touching the live database. Full application/S3 encrypted-object restore remains separately covered by service/staging tests; external S3 sandbox remains unavailable.

No live database, backup object, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2U backup API diagnostic redaction

Issues: `SEC-005`, `SEC-007`.

Strict response-path review found that persisted `error_message` and `restore_error` strings were returned verbatim by create, list, get, and restore endpoints. These fields can contain database, subprocess, object-store, or credential diagnostics.

Implementation:

- Added value-copy API mappers that clear internal backup and restore diagnostics while preserving IDs, status, progress, timing, format, and object metadata.
- Applied the mapper to create, list, get, and restore accepted/success responses.
- Internal persisted records and server logs retain diagnostics for operator troubleshooting; API responses do not.

GREEN:

```text
Internal backup diagnostic redaction: PASS
Internal restore diagnostic redaction: PASS
Operational fields preserved: PASS
Admin handler/service compilation: PASS
Focused handler/service vet: PASS
git diff --check: PASS
```

No live backup record, database, object, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2V S3 connection diagnostic redaction

Issues: `SEC-005`, `SEC-008`, `L10N-004`.

RED:

- Added a handler regression with an internal endpoint and secret-like value in the object-store error.
- The prior handler returned `err.Error()` verbatim in a successful JSON envelope.

Implementation:

- S3 connection checks now return only stable Chinese summaries: `连接成功` or `连接失败`.
- Raw SDK/network errors are no longer serialized to the client. Service-level errors remain available to server-side callers/logging paths.

GREEN:

```text
S3 failure response diagnostic redaction: PASS
S3 success response Chinese summary: PASS
Focused admin-handler vet: PASS
git diff --check: PASS
```

No live S3 endpoint, credential, object, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2W backup schedule fail-closed reads

Issues: `SEC-007`, `SEC-008`.

RED:

- Added repository-error and corrupt-JSON schedule tests.
- The prior implementation converted both conditions into an empty disabled schedule, hiding storage failures and corrupt policy.

Implementation:

- Missing schedule remains a valid unconfigured state.
- Repository failures now propagate.
- Malformed persisted schedule returns `BACKUP_SCHEDULE_CORRUPT`.
- Scheduled backup execution now aborts when retention policy cannot be read instead of silently applying a 14-day default and producing objects outside known operator policy.

GREEN:

```text
Missing schedule remains unconfigured: PASS
Repository failure propagation: PASS
Corrupt schedule rejection: PASS
Cron validation regression: PASS
Async backup/retention regressions: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live schedule, backup, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2X synchronous backup terminal-state durability

Issues: `SEC-007`.

RED:

- Added a synchronous dump-failure test where initial metadata persisted but terminal failed-state persistence did not.
- The prior path discarded the persistence error and returned only the dump error, leaving a durable `running` record with no indication that state reconciliation failed.

Implementation:

- Synchronous failure clears progress and requires terminal failed-state persistence.
- If both backup production and metadata persistence fail, the returned error joins both causes so callers can identify either with `errors.Is`.

GREEN:

```text
Failure-state persistence error surfaced: PASS
Original dump error retained in chain: PASS
Ordinary dump-failure contract: PASS
Initial metadata/orphan prevention: PASS
Successful encrypted backup regression: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live backup, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2Y asynchronous backup progress durability

Issues: `SEC-007`, `SEC-008`.

RED:

- Added a failure injection on the first background progress write.
- The prior implementation discarded both `dumping` and `uploading` persistence errors and still uploaded the encrypted object, creating state/object divergence.

Implementation:

- Asynchronous backup now requires durable `dumping` and `uploading` transitions before proceeding.
- A failed progress write aborts before dump/upload, records a terminal failed state, clears progress, and logs if even failure-state persistence cannot be written.

GREEN:

```text
Progress persistence failure aborts upload: PASS
Terminal failed state persisted: PASS
Async terminal-metadata retry regression: PASS
Async successful backup regression: PASS
Graceful shutdown regression: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live backup, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2Z backup retention input validation

Issues: `SEC-007`, `SEC-008`.

RED:

- Added negative retention-day/count tests for schedules and synchronous/asynchronous manual backup entry points.
- Negative values were previously accepted and persisted; manual backups interpreted negative expiry as no expiry.

Implementation:

- Schedule retention days/count must be non-negative.
- Both backup creation entry points reject negative expiry before operation admission, metadata writes, dumping, or upload.
- Zero remains the explicit unlimited/no-expiry value.

GREEN:

```text
Negative schedule days rejected: PASS
Negative schedule count rejected: PASS
Negative synchronous expiry rejected: PASS
Negative asynchronous expiry rejected: PASS
Zero/default retention regressions: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live schedule, backup, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AA stale-operation recovery durability

Issues: `SEC-007`.

RED:

- Added failure injection for persistence while reconciling a stale `running` backup.
- The previous startup recovery silently ignored record read/write failures and logged successful recovery even when metadata remained stale.

Implementation:

- Stale backup/restore recovery now returns repository and persistence failures.
- Success is logged only after each corrected record is durable.
- Backup scheduler startup returns without applying a cron schedule when interrupted-operation reconciliation fails.

GREEN:

```text
Stale recovery persistence failure surfaced: PASS
Stale backup reconciliation: PASS
Stale restore reconciliation: PASS
Backup service start/stop regression: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live backup metadata, schedule, database, object, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AB schedule persistence/runtime parity

Issues: `SEC-007`.

RED:

- Added an enabled-schedule update with no runtime scheduler.
- The old path persisted the enabled configuration first, then failed installation, leaving durable state claiming a cron job existed when none was running.

Implementation:

- Enabled schedule updates verify that the scheduler exists and the backup service is not stopping before writing configuration.
- Unavailable/stopped runtime rejects the update with zero settings writes.
- Existing cron parsing and retention validation remain pre-persistence gates.

GREEN:

```text
Unavailable scheduler prevents persistence: PASS
Cron validation: PASS
Negative retention validation: PASS
Service start/stop regression: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live backup schedule, database, object, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2 broad backend checkpoint after backup hardening

Comprehensive backend gates were rerun after Phases 2N–2AB:

```text
GOTOOLCHAIN=auto go test ./...: PASS
GOTOOLCHAIN=auto go test -tags=unit ./...: PASS
GOTOOLCHAIN=auto go vet ./...: PASS
```

This includes repository subprocess adapters, backup staging/crypto, synchronous and asynchronous service paths, admin handlers, config, routes, migrations, and all other backend packages. External S3 sandbox coverage remains blocked by missing sandbox endpoint/credentials and is not represented as complete.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2 race-sensitive backup checkpoint

Race detector gates after the latest state-machine and scheduler changes:

```text
GOTOOLCHAIN=auto go test -race -tags=unit ./internal/service -run '<backup lifecycle set>': PASS
GOTOOLCHAIN=auto go test -race ./internal/repository -run '^TestPg': PASS
GOTOOLCHAIN=auto go test -race ./internal/security/secretcrypto: PASS
```

Coverage includes backup/restore admission, async progress/terminal persistence, stale reconciliation, retention/delete, graceful shutdown, PostgreSQL subprocess adapters, and streaming crypto/key rotation.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AC non-mutating legacy-secret migration preflight

Issues: `SEC-005`, `SEC-007`.

Added read-only inspection cores for the future safety-gated operator migration command:

- API-key preflight validates every unmigrated active key and deleted-key audit with the configured gateway digester and reports counts without `FOR UPDATE` or writes.
- Account preflight parses every active credential row, authenticates protected envelopes, classifies plaintext rows, and reports migration counts without writes.
- Disposable PostgreSQL integration proves preflight counts match later migration counts and raw rows remain byte-semantically unchanged before the explicit migration call.

```text
API-key dry-run count/non-mutation: PASS
Deleted-audit dry-run count: PASS
Account credential dry-run count/non-mutation: PASS
Protected envelope authentication during preflight: PASS
Subsequent transactional migrations/idempotency: PASS
Focused repository vet: PASS
git diff --check: PASS
```

No operator command or startup hook invokes migration yet. Backup preflight and explicit confirmation remain required before exposing an apply command.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AD independent-review restore atomicity correction

Independent backup review verdict: `REVISE`. High finding: `psql --single-transaction` lacked `ON_ERROR_STOP`, allowing statement errors to be ignored and a transaction to commit.

RED/implementation:

- Added an adapter argument regression requiring `--set ON_ERROR_STOP=on`.
- Restore now combines `--single-transaction` with fail-fast SQL-error handling.

Disposable PostgreSQL 18 proof (`--network none`, no ports/volumes, synthetic credentials):

```text
Middle statement error returns non-zero: PASS
Statements before the error roll back: PASS
Later statement is not committed: PASS
Container network mode none: PASS
Host ports zero: PASS
Cleanup trap: PASS
Focused repository tests/vet: PASS
git diff --check: PASS
```

No live database, backup, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AE synchronous terminal-metadata compensation

Independent review medium finding: a successfully uploaded object could remain inaccessible if the completed metadata transition failed.

RED:

- Injected failure only on the completed-record write after a successful encrypted upload.
- Prior behavior retained the object while durable metadata remained `running`, making normal restore/download/delete contracts inconsistent.

Implementation:

- Completion metadata remains the discoverability boundary.
- On failed completion persistence, synchronous backup deletes the just-uploaded object.
- After successful compensation, it persists a terminal failed record with cleared progress.
- Object-deletion or compensated-state persistence failures are joined with the original metadata error rather than hidden.

GREEN:

```text
Completion write failure reproduced: PASS
Uploaded object compensation: PASS
Terminal failed metadata persisted: PASS
Compensated record manually deletable: PASS
Successful encrypted backup regression: PASS
Initial/failure metadata regressions: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live backup, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AF asynchronous terminal-metadata compensation

Independent review medium finding, asynchronous half.

RED:

- Injected failure only on the asynchronous completed-state write after encrypted upload.
- Prior behavior persisted `failed` but retained the S3 object; normal restore/download rejected the record and deletion skipped the object because status was not `completed`.

Implementation:

- Async completion failure compensates by deleting the just-uploaded object.
- Successful compensation persists a terminal failed record with cleared progress and a stable internal diagnostic.
- Compensation deletion/persistence failures are logged and retained in internal metadata for operator diagnosis; API redaction prevents those diagnostics escaping.

GREEN:

```text
Async completion failure reproduced: PASS
Async uploaded object compensation: PASS
Terminal failed state persisted: PASS
Compensated record manually deletable: PASS
Progress-write abort regression: PASS
Successful async backup regression: PASS
Graceful shutdown regression: PASS
Synchronous compensation regression: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live backup, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AG backup shutdown deadline ownership

Independent review medium finding: `BackupService.Stop()` could block for more than five minutes inside an application cleanup advertised as ten seconds, delaying Redis/PostgreSQL closure.

Implementation:

- Added `StopContext(ctx)` with caller-owned deadline.
- Shutdown marks admission closed, stops cron, immediately cancels the backup parent context, and joins active operations only until the supplied context expires.
- Application cleanup wiring now passes its global ten-second context into backup shutdown in both Wire source and generated wiring.
- The standalone `Stop()` compatibility wrapper retains a bounded five-minute context but cooperative operations are cancelled immediately.

GREEN:

```text
Uncooperative operation honors caller deadline: PASS
Cooperative blocked backup cancels and joins: PASS
Admission/Stop atomicity: PASS
Service start-after-stop regression: PASS
cmd/server compilation: PASS
Focused service/server vet: PASS
git diff --check: PASS
```

Infrastructure closure still follows service cleanup; on deadline the backup step returns a timeout instead of blocking the cleanup routine indefinitely.

No live backup, database, cache, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AH backup API topology redaction

Independent review low finding: backup record responses still exposed internal S3 object keys and filenames derived from the database name.

RED/implementation:

- Expanded the response-mapper regression with an internal database-derived filename and private object prefix.
- `BackupRecordForAPI` now clears `file_name` and `s3_key` in addition to error diagnostics.
- Create/list/get/restore handler paths already share this mapper, so operational IDs, status, size, format, timestamps, expiry, progress, and restore state remain available without storage topology.

GREEN:

```text
Database-derived filename redacted: PASS
Internal S3 key/prefix redacted: PASS
Backup/restore diagnostics redacted: PASS
Operational fields preserved: PASS
Admin handler suite: PASS
Focused service/handler vet: PASS
git diff --check: PASS
```

Download continues to resolve the object key server-side by opaque backup ID; no client API requires the internal key.

No live backup metadata, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AI actual backup handler response contracts

Independent review missing-test item: response redaction had mapper tests but no actual Gin handler serialization assertions.

Added HTTP-level tests using a real `BackupService` over an isolated in-memory settings stub:

```text
GET backup record response: PASS
LIST backup records response: PASS
Opaque backup ID retained: PASS
Database-derived filename absent: PASS
Internal S3 key/prefix absent: PASS
Backup diagnostic absent: PASS
Restore diagnostic absent: PASS
Focused admin handler tests/vet: PASS
git diff --check: PASS
```

The tests exercise the standard response envelope and real handler methods rather than only the response mapper.

No live request, backup metadata, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AJ retention delete/metadata crash consistency

Independent review missing-test item: object deletion could succeed while metadata persistence failed, leaving a completed record pointing at an absent object.

RED:

- Injected failure after successful object deletion but before removal of its metadata record.
- Required retry to converge without falsely presenting the record as restorable.

Implementation:

- Retention now durably persists a `deleting` tombstone before object deletion.
- After idempotent object deletion succeeds, metadata is removed and persisted.
- If metadata removal fails, the tombstone survives; the next cleanup retries object deletion and completes removal.
- Object deletion failures likewise retain the tombstone for safe retry.

GREEN:

```text
Deletion-intent persistence: PASS
Object deletion success + metadata failure: PASS
Durable non-restorable tombstone: PASS
Retry convergence/removal: PASS
Partial multi-object failure regression: PASS
Object-delete failure retention: PASS
Manual deletion regressions: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live backup metadata, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2 post-review broad/race checkpoint

After correcting all concrete findings from the first independent backup review:

```text
GOTOOLCHAIN=auto go test ./...: PASS
GOTOOLCHAIN=auto go test -tags=unit ./...: PASS
GOTOOLCHAIN=auto go test -race -tags=unit ./internal/service -run '<backup lifecycle set>': PASS
GOTOOLCHAIN=auto go vet ./...: PASS
git diff --check: PASS
```

The checkpoint includes restore fail-fast atomicity, sync/async upload compensation, retention tombstone convergence, shutdown deadline ownership, API topology redaction, handler serialization, and all unrelated backend packages. A follow-up independent read-only review was dispatched against the current snapshot before ledger closure.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AK manual delete/metadata crash consistency

Strict follow-through applied the retention tombstone contract to manual deletion.

RED:

- Injected failure after manual object deletion but before metadata removal.
- Prior manual deletion could leave a `completed` record pointing at an absent object and retry would skip object cleanup for non-completed compensation states.

Implementation:

- Manual deletion persists `deleting` before object storage mutation.
- Object deletion is retried for both completed and existing deleting records.
- Metadata removal is a separate persisted step; failure leaves a durable non-restorable tombstone.
- Retrying deletion converges through idempotent object deletion to metadata removal.

GREEN:

```text
Manual deletion-intent persistence: PASS
Object success + metadata failure: PASS
Durable deleting tombstone: PASS
Manual retry convergence: PASS
Object-deletion failure retention: PASS
Normal manual deletion: PASS
Sync/async compensation deletability: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live backup metadata, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AL retention timestamp corruption fail-closed

Strict retention review found malformed persisted `started_at` values were silently ignored, indefinitely bypassing age retention without surfacing corrupt metadata.

RED/implementation:

- Added a completed object-backed record with a malformed RFC3339 timestamp.
- Age-based cleanup now returns an identified parsing error before any deletion intent, object mutation, or metadata rewrite.
- The record remains completed and its object remains intact for operator repair.

GREEN:

```text
Malformed timestamp detected: PASS
No object deletion on corrupt metadata: PASS
No metadata state mutation: PASS
Valid age/count retention regressions: PASS
Tombstone retry regressions: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live backup metadata, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AM count-retention completed-set semantics

Strict retention review found `RetainCount` used the index of every record after sorting. Newer failed/running records consumed retention slots and could delete the newest valid completed backup.

RED/implementation:

- Added a newer failed record followed by two completed backups under `RetainCount: 1`.
- Count retention now increments its quota only for completed backups.
- Tombstones remain unconditional retry candidates and failed/running records neither consume completed-backup quota nor get deleted by count policy.

GREEN:

```text
Newer failed record does not consume quota: PASS
Newest completed backup retained: PASS
Older completed backup deleted: PASS
Failed record retained: PASS
Age/count/tombstone regressions: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live backup metadata, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AN compensation-delete failure reconciliation

Follow-up independent review medium finding: if completion metadata and compensating object deletion both failed, sync metadata stayed `running` and async metadata became `failed`; neither state was retried by retention.

RED/implementation:

- Added sync and async failure injection for completed-state persistence plus object compensation deletion.
- Both paths now persist a `deleting` cleanup tombstone with stable internal diagnostics when compensation deletion fails.
- Retention processes tombstones regardless of configured age/count, retries idempotent object deletion, and removes metadata after success.

GREEN:

```text
Sync double-failure tombstone: PASS
Async double-failure tombstone: PASS
Object retained while cleanup pending: PASS
Retry deletion convergence: PASS
Successful compensation regressions: PASS
Retention tombstone regressions: PASS
Focused service vet: PASS
git diff --check: PASS
```

A hard crash before any post-upload state write cannot be made fully atomic across independent settings and S3 stores; pre-upload durable identity plus explicit tombstones cover all observable error paths. Real S3 sandbox remains unavailable.

No live backup metadata, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-15 — Phase 2AO enforceable global cleanup deadline

Follow-up independent review high finding: the global ten-second cleanup context did not bound non-backup cleanup because orchestration unconditionally waited on a `WaitGroup`.

Implementation:

- Extracted deadline-aware parallel cleanup orchestration.
- The orchestrator returns when all application cleanup finishes or when the global context expires.
- Explicit infrastructure policy: if any application cleanup remains active at deadline, Redis/Ent closure is skipped to avoid use-after-close races; process exit reclaims those clients.
- If all application cleanup completes, Redis then Ent close sequentially as before.
- Wire source and generated wiring use the same helper.

GREEN:

```text
Blocked non-backup step bounded by deadline: PASS
Completed-step result collection: PASS
cmd/server compile/tests: PASS
cmd/server vet: PASS
git diff --check: PASS
```

No service cleanup was invoked against the live process; no Redis/PostgreSQL client was closed.

Deployment/runtime mutation: none.

## 2026-07-16 — Phase 2AP restart reconciliation for interrupted upload

Follow-up independent review crash-gap finding: startup converted every stale `running` backup to inert `failed`, including records durably marked `uploading` whose external object may already exist.

RED/implementation:

- Added a restart record with `status=running`, `progress=uploading`, and an object key.
- Startup reconciliation now converts that shape to durable `deleting`, clears progress, and records cleanup-pending diagnostics.
- Earlier `pending`/`dumping` interruptions remain failed because no completed external upload is implied.
- Retention processes the tombstone independently of age/count and retries object cleanup.

GREEN:

```text
Interrupted-upload tombstone: PASS
Pending/dumping stale failure regression: PASS
Stale restore failure regression: PASS
Tombstone retention convergence: PASS
Double-failure compensation regressions: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live restart, backup metadata, object, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-16 — Phase 2AQ protected-key deletion cache invalidation

Strict API-key path review found deletion fetched the persisted `key` field. Protected rows contain an HMAC-derived placeholder, not the submitted raw key, so exact cache invalidation targeted the placeholder fingerprint and left cached authentication for the deleted key reachable until TTL.

RED/implementation:

- Added deletion regression with a protected placeholder and an unrelated submitted-key cache fingerprint.
- Protected-row deletion now advances user/global authentication-cache generation instead of performing fake exact invalidation.
- Legacy plaintext rows retain exact invalidation during controlled migration compatibility.

GREEN:

```text
Protected deletion advances generation: PASS
Placeholder not used as raw cache key: PASS
Legacy exact invalidation regression: PASS
Delete failure/no-invalidation regression: PASS
Ownership and touch-cache regressions: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live API key, cache, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-16 — Phase 2AR synchronous upload crash state and detached compensation

Final backup closure review verdict remained `REVISE` for two synchronous/scheduled-path gaps.

Implementation:

- Synchronous backup now durably transitions `pending -> uploading` before dump/encryption/external upload.
- Startup therefore converts a crash-interrupted synchronous upload into a retryable `deleting` tombstone rather than inert `failed`.
- Completion-state failure compensation now uses a detached bounded ten-second context for object deletion and failed/tombstone persistence, so an expired caller/operation context cannot prevent cleanup.

GREEN:

```text
Synchronous uploading state durable before external work: PASS
Scheduled-path success regression: PASS
Sync completion compensation: PASS
Sync compensation-delete tombstone: PASS
Restart upload tombstone reconciliation: PASS
Retention convergence: PASS
Focused service vet: PASS
git diff --check: PASS
```

No live scheduled backup, object, metadata, database, service, or deployment was touched.

Deployment/runtime mutation: none.

## 2026-07-16 — Deployment follow-up: executable Redis authentication contract

The explicitly authorized deployment exposed a defect not caught by render-only Compose tests: the multiline `sh -c` body launched bare `redis-server` as its first blocking command, so later option lines including `--requirepass` were never arguments.

Remediation:

- Reopened `SEC-006` before changing code.
- Replaced the multiline shell body in standard, local, and development manifests with one `exec redis-server ... --requirepass "$REDISCLI_AUTH"` command.
- Made the development Redis credential fail closed as well.
- Upgraded the render regression to parse Compose JSON and assert the exact command vector.
- Rotated the accidentally exposed deployment Redis credential before final verification.

Verification:

```text
Compose Redis authentication contract: PASS
Disposable Redis 8, network=none, host ports=0: PASS
Disposable unauthenticated PING rejected: PASS
Disposable authenticated PING accepted: PASS
Deployed unauthenticated PING rejected: PASS
Deployed authenticated PING accepted: PASS
Application health after coordinated Redis/application restart: PASS
```

`SEC-006` is closed again. The deployment backup is retained under `/home/opc/archive/sub2api-deployments/20260716T105059Z/`.
