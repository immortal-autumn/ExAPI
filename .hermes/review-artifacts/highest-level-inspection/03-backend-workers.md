# Backend Routes, Authorization, and Background Work

Captured: 2026-07-13

## Route registration

Private mode is implemented with registration-time guards, not only handler-level checks.

### Correctly omitted in private mode

- Registration, email-verification, password-recovery, promo/invitation validation.
- Customer OAuth providers and account-creation/binding flows.
- Payment and webhook routes.
- Announcements, redeem codes, subscriptions, affiliates, customer channels, and user-management administration.

Tests in `backend/internal/server/routes/single_user_route_matrix_test.go` verify representative positive and negative route families.

### Retained customer-shaped routes

The cockpit still depends on authenticated user APIs for:

- `/api/v1/keys`
- `/api/v1/groups/available`
- `/api/v1/groups/rates`
- `/api/v1/usage/*`
- `/api/v1/auth/me`, refresh, logout, and session revocation
- user profile/password/platform-quota endpoints

These are authenticated, but they preserve customer-domain coupling in a nominally operator-only appliance.

## Public/private host guard

`backend/internal/server/middleware/public_control_plane_guard.go` blocks every path on the configured public host except health and gateway compatibility prefixes.

The active Nginx site mirrors the allowlist and returns `404` for everything else. The actual public gateway contract includes:

- `/v1/*`
- `/v1beta/*`
- `/backend-api/codex/*`
- `/antigravity/*`
- root aliases for responses/chat/embeddings/images/videos
- `/health`

This is broader than a shorthand “only `/v1/*`” product statement but is intentional in both source and deployment.

## Confirmed embedded-frontend routing defect

`backend/internal/web/embed_on.go:300-311` bypasses the SPA middleware for:

- `/api/*`, `/v1/*`, `/v1beta/*`, `/backend-api/*`, `/antigravity/*`
- `/setup/*`, `/health`, `/responses*`, and `/images/*`

It omits:

- `/chat/completions`
- `/embeddings`
- `/videos/*`

As a result, the public compatibility aliases for these endpoints are intercepted by the SPA fallback and return `200 text/html` rather than reaching API-key authentication. `/responses` and `/images/generations` correctly return `401` without credentials.

This is a functional gateway defect, not a demonstrated authentication bypass.

## Local administrator bypass

`backend/internal/handler/auth_handler.go:220-318` requires:

1. Explicit `SUB2API_LOCAL_ADMIN_BYPASS` enablement.
2. A localhost Host with loopback/private transport source, or a Host inside configured trusted CIDRs with loopback/private/trusted transport source.
3. An existing active administrator.

Live configuration:

- Bypass enabled.
- Trusted CIDR restricted to the WireGuard subnet.
- Application port bound only to loopback and the WireGuard address.
- Public-host requests are rejected before the SPA/API control plane.

The decision intentionally ignores forwarded headers and uses `RemoteAddr`, but it also treats the HTTP `Host` header as part of the privilege boundary. Behind a catch-all reverse proxy, an attacker could submit `Host: localhost`; the backend would see the proxy or Docker bridge as a private `RemoteAddr`, allow the request past the public-host guard, and issue administrator tokens.

The **current deployment mitigates this exact Internet path at Nginx**: `/admin/*` and `/api/*` are not proxied by the public virtual host, and state-free live checks returned Nginx's identical 153-byte `404` for normal, `Host: localhost`, and WireGuard-Host requests. The application port is also bound only to loopback and WireGuard.

Therefore this is not a demonstrated current public takeover, but it is a high-severity deployment-fragile design. A future catch-all proxy, ingress rewrite, or direct untrusted private-network path would make it critical. Local administrator authentication should use a separate listener/socket or cryptographically authenticated proxy boundary, never a spoofable Host value.

## CORS and proxy trust

- CORS has no configured allowed origins and fails closed.
- Same-origin and hostile-origin preflights both returned `403`; browser same-origin calls do not require CORS.
- Wildcard-plus-credentials is explicitly prevented in middleware.
- Gateway API-key forwarded-client-IP trust is currently disabled, which is the safe default.
- Nginx forwards real IP headers, but API-key ACL evaluation does not trust them until the operator opts in.

## Background workers

### Product-aligned workers

- OAuth token refresh for upstream accounts.
- Account/proxy expiry.
- scheduler snapshots and deferred scheduling.
- concurrency and message-queue cleanup.
- dashboard/usage aggregation and cleanup.
- Ops metrics, aggregation, alerting, cleanup, reports, and log sink.
- backups.
- scheduled account tests.
- channel monitoring, when configured.

### SaaS/dormant workers still present

- `EmailQueueService` starts three workers unconditionally in its constructor. Live logs confirm startup. Its tasks are customer verification and password reset.
- `PaymentOrderExpiryService` starts an unconditional 60-second loop and can reconcile payment-provider orders. The empty `payment_orders` table has nevertheless accumulated more than 11,000 index scans.
- `BatchImageCleanupService` and `BatchImageWorkerRuntime` are configuration-gated, not product-mode-gated. Current empty tables indicate no active workload, but private mode does not itself disable them.
- `UserPlatformQuotaUsageFlusher` is feature-config gated, not private-product gated.
- Subscription expiry is the only inspected SaaS worker explicitly guarded by `config.SaaSFeaturesEnabled()`.
- Payment, affiliate, redeem, subscription, billing, and customer notification services remain constructed through Wire even when their routes are absent.

### Scheduled test overlap

`ScheduledTestRunnerService` fires every minute, allows each invocation to run for up to five minutes, and has no skip-if-running wrapper, atomic database claim, leader lease, or distributed lock before executing due plans. A slow run can overlap the next tick, and multiple application instances can run the same due plan before `UpdateAfterRun`.

Acceptance requires a same-process overlap test plus a two-runner/shared-repository test proving exactly one execution per due plan.

## Other backend findings

- `ProxyService.TestConnection()` is a no-op with a TODO and always returns success after loading the proxy. The visible “Test connection” operator control is therefore misleading unless another handler path bypasses this method.
- Update service is still constructed but does not start a background loop; updater routes/UI are absent.
- Runtime logging includes customer email addresses in EmailQueue success/failure messages; those workers should not exist in private mode.
- Configurable backup S3 endpoints are passed directly into the AWS client and tested without the project's existing SSRF validator. Because this is administrator-only today, severity is moderate; reject loopback, link-local, metadata, private/DNS-rebinding and redirect targets unless private object storage is explicitly approved.
- Grok OAuth accepts a bare authorization code without mandatory state and permits request redirect URI to override session state. Require state and exact stored redirect binding.
- Gemini OAuth redirect construction trusts `Origin` and `X-Forwarded-*` independently of configured trusted proxies. Use a canonical external origin or trusted-proxy-aware request state.

## Assessment

The backend boundary is materially safer than the frontend surface, but private mode is not a complete capability cut. Route omission is effective; dependency injection, schemas, periodic workers, and retained customer-shaped APIs still carry substantial SaaS architecture.
