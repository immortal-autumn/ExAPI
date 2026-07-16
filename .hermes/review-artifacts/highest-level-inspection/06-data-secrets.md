# Data, Secrets, Backups, and Redis Inspection

Captured: 2026-07-13

No database values, tokens, passwords, account identifiers, or Redis values were read or recorded.

## PostgreSQL inventory

The live database retains the complete SaaS-era schema. Active operational rows are concentrated in:

- Ops system logs and metrics.
- Settings and alert rules.
- One administrator, one upstream account, one API key, and one usage event.
- Gateway groups, scheduler snapshots/outbox, and aggregation watermarks.

Dormant/empty table families include:

- Payment orders/providers/audit logs and billing usage.
- Subscription plans and user subscriptions.
- Promo and redeem codes.
- Affiliates and affiliate ledgers.
- Announcements.
- Pending customer OAuth/auth sessions.
- Customer identity, quota, group-rate, avatar, and attribute tables.
- Batch-image jobs/items/events.
- Customer channels and channel pricing.

These migrations are not directly exploitable by being empty, but they enlarge maintenance, backup, and accidental-reactivation surface.

## Sensitive storage

### Stored directly in PostgreSQL

- Upstream account credentials: `accounts.credentials` JSONB.
- Gateway API keys: `api_keys.key` string.
- Proxy passwords: `proxies.password` string.
- SMTP password and Turnstile secret in generic settings.
- LinuxDo, DingTalk, OIDC, GitHub, Google, and WeChat OAuth client/application secrets in settings.
- Web-search provider API keys inside a settings JSON value.
- JWT signing root in `security_secrets.value`; bootstrap explicitly persists and then prefers the database value for cross-instance consistency.

Source tracing found response redaction for several fields but no comprehensive encryption transform before persistence. Repository and settings code use these values directly, confirming recoverable plaintext semantics at the database layer.

### Required cryptographic classification

- Gateway authentication keys do not need replay: store a versioned keyed digest/verifier and compare in constant time; retain only a short non-secret prefix for operator identification.
- Upstream, proxy, SMTP, OAuth, anti-bot, and provider credentials require reversible versioned envelope encryption.
- JWT/signing roots and data-encryption roots need an external key source or explicitly documented threat model; storing the protecting root in the same database/backup defeats dump-compromise protection.

### Encrypted or hashed

- Administrator/customer passwords are hashes.
- TOTP secrets use an encrypted field and a configured secret encryptor.
- S3 backup Secret Access Key is encrypted through `SecretEncryptor`, omitted from read responses, and preserved when an update omits the secret.
- Channel-monitor API keys use an encrypted field.

### Consequence for backups

Database dumps necessarily contain gateway-key verifiers or current plaintext keys, upstream credentials, proxy/SMTP/OAuth/provider secrets, and the JWT signing root. Encrypting the S3 transport credential does not encrypt dump contents. Backup confidentiality therefore requires independent payload encryption and an external/wrapped root key.

## Backup runtime

- No S3 backup configuration, schedule configuration, or backup records currently exist.
- No local backup artifact was found under `/opt/sub2api` by filename search.
- The backup service starts regardless, but cannot provide a recovery point while unconfigured.
- Redis authentication/session/scheduler state is not included in the PostgreSQL dump workflow.
- Backup payloads are gzip-compressed but not application-encrypted.
- Despite the streaming service API, `S3BackupStore.Upload()` calls `io.ReadAll`, loading the entire compressed dump into memory before `PutObject`; this can create a large memory spike as the database grows.

## Table size and scan observations

Largest relation:

- `ops_system_logs`: approximately 12.9 MB with about 11.9k live rows.

Notable scan patterns:

- `scheduler_outbox`: roughly 691k sequential scans with one live row.
- `usage_logs`: roughly 145k index scans with one live row.
- `ops_error_logs`: roughly 98k index scans with tens of rows.
- Empty `payment_orders`: more than 11k index scans, consistent with unconditional payment-expiry polling.
- Several aggregate/watermark tables contain dead tuples disproportionate to their tiny live row counts; normal autovacuum should handle them, but churn should be monitored.

`pg_stat_statements` is not installed, so statement-level latency and call attribution could not be inspected without changing the database.

## Redis

Read-only metadata inspection (no values) found:

- 106 keys; 54 with expiry.
- ~1.61 MiB logical memory and ~14.19 MiB RSS at sample time.
- `maxmemory=0` with `noeviction`.
- 67,280 hits versus 355,341 misses.
- Key families include refresh-token/token-family state, concurrency, scheduler, and OAuth state.

Redis is therefore not merely disposable performance cache. It carries authentication and scheduler state, has no memory ceiling, and uses a no-eviction policy that can turn host/container memory exhaustion into failed writes. The high miss ratio warrants per-key-family instrumentation before changing TTLs.

The container's direct `REDISCLI_AUTH` path did not authenticate during the parent inspection, although the application remained healthy; operational CLI authentication should be aligned without exposing credentials.

## Recommendations

1. Complete a field/key inventory, then use versioned keyed digests for gateway authentication keys and envelope encryption for credentials that require replay; keep root keys outside the database/backup boundary.
2. Configure scheduled off-host backups, add independent authenticated encryption for dump payloads, and perform a documented restore drill.
3. Define whether Redis state is reconstructible or requires infrastructure-level persistence/backup; set container and Redis memory limits appropriate to that decision.
4. Remove dormant SaaS tables only through a separate, backed-up migration phase after confirming rollback requirements.
5. Disable SaaS pollers before schema removal.
6. Correct the Redis CLI authentication environment for operational diagnostics without exposing credentials.
7. Add scan/churn alerts for scheduler outbox and Ops log growth.
