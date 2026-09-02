# ExAPI

ExAPI is a private AI API gateway for routing local tools, coding agents, and
application traffic through a manageable pool of upstream AI accounts.

This fork is based on `Wei-Shaw/sub2api`. Some internal modules, database,
cache, and service identifiers intentionally retain the `sub2api` name for
deployment compatibility. The ExAPI product direction is a private control
plane, visible quota diagnostics, resilient multi-account routing, and a
local-first integration experience.

English (default) | [简体中文](README.zh-CN.md)

## Current status

The current OPC production release is **ExAPI v0.2.14**, commit
`4b0352fa87720425bf4fb5c23aa91e2c0e212c9e`. Production uses only the immutable
OCI digest `sha256:e8a6d161a1acb5d454a13526ef2914533d077fd5aefae7a412bc45f58513857d`,
validated by the release workflow with SBOM and provenance attestations; mutable
tags such as `latest` are not used for production. The release/review branch
contains the hardening included in this deployed image.

- Current release, image digest, deployment validation, and provider status:
  [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md)
- Simplified Chinese status mirror: [`docs/PROJECT_STATUS.zh-CN.md`](docs/PROJECT_STATUS.zh-CN.md)
- Documentation map and maintenance rules: [`docs/README.md`](docs/README.md)
- Production promotion and rollback gates:
  [`deploy/PRODUCTION_ROLLOUT.md`](deploy/PRODUCTION_ROLLOUT.md)

## Features

- OpenAI-compatible `/v1` gateway.
- Client-compatible routing for Claude, Codex, Gemini, Antigravity, and other
  upstream integrations inherited from Sub2API.
- A localhost/WireGuard private control plane for single-user deployments.
- Upstream account pools, scheduled account tests, quota windows, and usage
  logs.
- Manual account probes are independent from scheduler state: failures are
  visible and do not silently disable an account.
- Forced real-time Antigravity quota refresh, per-model quota display, and
  Google 429 classification.
- API-key management for IDEs, agents, and automation scripts.
- Legacy upstream customer, subscription, payment, and redeem-code modules may
  remain in the source tree for schema and migration compatibility. They are
  not part of the ExAPI product, are not registered in private mode, and must
  not be enabled as an operator workflow. Routing groups remain supported
  because they are gateway primitives used by accounts and API keys.

## Deployment modes

### Private single-user mode

Recommended for a personal server:

- Expose only the AI gateway paths through the public domain.
- Keep the administration UI reachable only through localhost or
  WireGuard/VPN.
- Prioritize accounts, API keys, usage, proxies, operations, and settings in
  the sidebar.
- Show quota monitoring and local integration entry points directly in the
  cockpit.

### Legacy compatibility settings

ExAPI has one supported product mode: the private administrator control plane.
The upstream `standard`/`simple` setting is retained in source-compatible
schemas. New deployments must set `RUN_MODE=simple`; the backend keeps its
legacy `standard` fallback for compatibility and billing safety when an old or
malformed configuration is encountered.
Customer registration, subscriptions, payments, balances, redeem codes,
affiliates, and customer-management routes are retired and return a stable
retirement response. See [`docs/PRIVATE_PRODUCT_SURFACE.md`](docs/PRIVATE_PRODUCT_SURFACE.md)
for the allowlist and compatibility policy.

## Technology stack

- Backend: Go, Gin, Ent, PostgreSQL, and Redis.
- Frontend: Vue 3, TypeScript, Pinia, Vue Router, TailwindCSS, and Vite.
- Deployment: Docker/Compose or systemd, normally behind nginx, Caddy, or
  Cloudflare.

The Go module path and many runtime names remain `sub2api`. Read
[`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md) before
attempting to rename modules, services, databases, or data directories.

## Quick start

Start with the templates under [`deploy/`](deploy/). Unless you have completed
the migration, keep `sub2api` paths and service names used by upstream
deployment commands unchanged.

Recommended private deployment settings:

```env
RUN_MODE=simple
SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true
SUB2API_PUBLIC_HOST=your-public-ai-gateway.example.com
# Required external root; docker-deploy.sh generates these securely.
SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data-v1
SUB2API_DATA_ENCRYPTION_KEYS_JSON={"data-v1":"<base64-encoded-32-byte-key>"}
```

The external data keyring is mandatory and must be retained outside PostgreSQL
and ordinary data backups. Prefer `deploy/docker-deploy.sh` or
`deploy/install.sh`; they generate and permission it automatically. See
[`deploy/README.md`](deploy/README.md) for manual generation, rotation,
migration, and recovery guidance.

New installations enforce outbound host validation with HTTPS and public
destinations by default. Existing deployments temporarily retain
`SECURITY_OUTBOUND_MODE=compat`; review custom/private upstreams and migrate to
`enforce` using [`deploy/EDGE_SECURITY.md`](deploy/EDGE_SECURITY.md).

Only the AI gateway paths should be public. Keep `/admin`, `/login`, and
`/api/v1/*` control-plane APIs private. The control plane validates requests
from explicitly allowed WireGuard peers; a 404 from the server itself or an
unauthorized peer is an expected hidden response and does not mean that the
control process is down. See [`deploy/EDGE_SECURITY.md`](deploy/EDGE_SECURITY.md)
for the boundary details.

## Account probes and quota diagnostics

When an administrator tests an account manually, the latest result is stored in
`account.extra.account_test_probe` without directly changing `account.status`
or `schedulable`. Scheduled tests do not overwrite the manual result; material
credential, route, or proxy changes invalidate stale results.

Antigravity live usage queries use `force=true` to bypass the backend quota
cache. A successful quota query proves only that the quota endpoint is
reachable; it does not prove that inference is currently accepted. Run a
manual probe against a model currently advertised by the provider. The full
operator workflow is in [`docs/ACCOUNT_PROBES.md`](docs/ACCOUNT_PROBES.md).
If a fresh quota snapshot is already cached, an implicit probe selects a
recommended, non-exhausted text model without issuing another quota request;
an explicit model selection always wins. Probe errors expose only bounded
reasons such as `model_unsupported` or `quota_exhausted`, never raw provider
response bodies.

## Retired upstream customer documentation

The former payment and external payment-integration guides are retained as
archived upstream references only. They do not describe an enabled ExAPI
feature and must not be used to configure a live instance:

- [`docs/PAYMENT.md`](docs/PAYMENT.md) / [`docs/PAYMENT_CN.md`](docs/PAYMENT_CN.md)
- [`docs/ADMIN_PAYMENT_INTEGRATION_API.md`](docs/ADMIN_PAYMENT_INTEGRATION_API.md) /
  [`docs/ADMIN_PAYMENT_INTEGRATION_API.zh-CN.md`](docs/ADMIN_PAYMENT_INTEGRATION_API.zh-CN.md)

## Security notice

Using consumer AI accounts through a gateway may violate a provider's terms.
Read and follow the applicable terms and laws, and use only accounts and
traffic you are authorized to operate. This project is intended for technical
research and self-hosted infrastructure; deployment, account, and data risks
remain with the operator.

## Upstream acknowledgement

ExAPI derives from the Sub2API project maintained by Wei-Shaw and its
contributors. This README focuses on private operations rather than upstream
sponsorship or promotion. Compatibility details are documented in
[`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md).
