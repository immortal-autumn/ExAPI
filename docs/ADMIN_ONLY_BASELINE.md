# ExAPI administrator-only baseline

English (default) | [简体中文](ADMIN_ONLY_BASELINE.zh-CN.md)

This is the non-sensitive Phase 0 baseline for the administrator-only product
review. It records the reviewed source, the current OPC black-box results, and
the route/feature boundary. Secrets, provider credentials, raw responses,
database rows, and private listener addresses are intentionally excluded.

## Review identity

| Item | Value |
|---|---|
| Review date | 2026-09-02 (Europe/London) |
| Review branch | `review/admin-only-baseline` |
| Reviewed source | `f43c4c150cb0a779b4e5766989617d98d304bba7` |
| Production release | v0.2.14 |
| Production source revision | `4b0352fa87720425bf4fb5c23aa91e2c0e212c9e` |
| Production application image | `ghcr.io/immortal-autumn/sub2api2personal@sha256:e8a6d161a1acb5d454a13526ef2914533d077fd5aefae7a412bc45f58513857d` |
| Production Compose provenance | `docker-compose.v0.2.14.yml` / `.env.v0.2.14` |
| Production state | healthy; zero application, PostgreSQL, and Redis restarts |

The review source is not yet a release tag and has no published OCI image. It
must not be substituted into production until the release workflow builds and
attests a digest for this exact source revision.

## OPC black-box acceptance

Checks were made through the authorized operator network path on 2026-09-02.
Only status codes and bounded summaries were retained:

| Surface | Result |
|---|---|
| Public `/health` | HTTP 200, `{"status":"ok"}` |
| Public `/ready` | HTTP 200, `{"status":"ready"}` |
| Public gateway `/v1/models` without a key | HTTP 401, machine-readable API-key-required error |
| Public control UI/API paths | HTTP 404 |
| Private control `/health` and `/ready` | HTTP 200 |
| Private `/api/v1/operator/me` with control marker | HTTP 200 |
| Private Cockpit summary with control marker | HTTP 200 |

The private control request marker and same-origin mutation policy were honored;
an unmarked control API request was rejected with `CONTROL_REQUEST_REQUIRED`.

## Enabled-provider probes

The account list and model lists were fetched through the private administrator
API without refreshing credentials. A single minimal manual inference probe was
then run for each configured provider. The probe endpoint writes only the
bounded manual-test snapshot described in [`ACCOUNT_PROBES.md`](ACCOUNT_PROBES.md);
it does not change scheduler eligibility.

| Provider | Accounts | Non-refresh model list | Minimal inference probe |
|---|---:|---:|---|
| Antigravity | 1 active/schedulable | 28 models | HTTP 200 SSE; provider returned HTTP 429 `quota_exhausted` for `gemini-2.5-flash`; account stayed active/schedulable |
| Grok | 2 active/schedulable | 13 models per account | HTTP 200 SSE; `grok-4.5` completed successfully (`reason=ok`) |

The Antigravity result is an upstream quota condition, not a routing or
authentication failure. Repeat the probe after the provider reset before
claiming inference availability for that account.

## Administrator route and handler inventory

### Browser navigation

The browser allowlist in
[`frontend/src/config/singleUserProduct.ts`](../frontend/src/config/singleUserProduct.ts)
contains 14 administrator pages:

`/admin/dashboard`, `/admin/ops`, `/admin/accounts`, `/admin/batch-images`,
`/admin/groups`, `/admin/api-keys`, `/admin/channels/pricing`,
`/admin/channels/monitor`, `/admin/proxies`, `/admin/risk-control`,
`/admin/usage`, `/admin/settings`, `/admin/audit-logs`, and
`/admin/prompt-audit`.

There are no public browser product routes. Compatibility redirects are `/`,
`/home`, `/admin`, `/admin/channels`, and `/keys`. Registration, customer
dashboard, payment, subscription, redeem, affiliate, profile, and legacy
batch-image paths resolve to the explicit retired-feature view.

### Active backend namespaces

The private listener registers these operator namespaces:

- `/api/v1/operator/me` and `/api/v1/settings/public`;
- `/api/v1/keys`, `/api/v1/groups`, `/api/v1/usage`,
  `/api/v1/operator/batch-images`, and `/api/v1/channel-monitors`;
- `/api/v1/admin/cockpit-summary` plus dashboard, groups, accounts, OAuth
  (OpenAI/Gemini/Antigravity/Grok), proxies, settings, data-management,
  backups, ops, system, usage, error-passthrough, TLS-fingerprint-profiles,
  channels, channel-monitors, monitor templates, risk-control, prompt-audit,
  and audit-log namespaces.

The public listener retains only health/readiness and API-key gateway families:
`/v1`, `/v1beta`, `/responses`, `/chat/completions`, `/embeddings`, image/video
routes, `/backend-api/codex`, and `/antigravity`. Gateway requests require an
API key and do not expose the administrator UI.

### Handler ownership

Active administrator handlers are Dashboard, Group, Account, OAuth,
OpenAIOAuth, GeminiOAuth, AntigravityOAuth, GrokOAuth, Proxy, Setting,
DataManagement, Backup, Ops, System, Usage, ErrorPassthrough,
TLSFingerprintProfile, AdminAPIKey, ScheduledTest, Channel, ChannelMonitor,
ChannelMonitorRequestTemplate, ContentModeration, PromptAudit, AuditLog, and
Compliance. User, Announcement, Redeem, Promo, Subscription, UserAttribute,
Affiliate, and Payment handler fields remain only as source-compatibility
seams; their customer routes are not registered in private mode.

Legacy customer API prefixes are covered by the stable `CUSTOMER_SURFACE_RETIRED`
410 contract in the backend. The public reverse proxy hides those paths with
404 before they reach the application.

## Language and quality baseline

- English is the default UI, installer, release, and unsuffixed documentation
  language; Simplified Chinese translations are paired under `.zh-CN.md` or
  locale `zh-CN`.
- Locale compilation, key integrity, production-copy, Cockpit bilingual, and
  subscription-validity tests pass.
- The current local quality run passed 267 frontend test files / 1,436 tests,
  the full Go test suite, frontend typecheck/lint/build/bundle budgets, and all
  deployment contract tests.

## Evidence and follow-up

The command-level evidence for this baseline is kept under
`tmp/usability-review/current/phase-0-baseline-2026-09-02.md` in the checkout;
it is intentionally ignored by Git. This committed summary contains no
credentials or raw provider data.

The next gate is to wait for GitHub CI and Security Scan on the review source,
then create a release candidate with a new version, signed SBOM/provenance,
fresh restored-data and synthetic-provider canaries, and a dry-run private
cutover report. Production remains on v0.2.14 until every gate passes.
