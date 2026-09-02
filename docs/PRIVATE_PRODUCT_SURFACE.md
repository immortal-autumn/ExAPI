# ExAPI private product surface

English (default) | [简体中文](PRIVATE_PRODUCT_SURFACE.zh-CN.md)

ExAPI has one supported product mode: a private, administrator-only control
plane for operating an upstream AI account pool and its API-key gateway.

## Supported operator capabilities

- Gateway health, model routing, usage, errors, and operational diagnostics.
- Upstream account and OAuth lifecycle, quota inspection, and account probes.
- API-key creation, rotation, revocation, limits, and routing-group binding.
- Gateway groups, channel pricing/monitoring, proxies, risk controls, and
  prompt auditing when configured.
- Encrypted backup/recovery and operator security settings.

## Retired customer capabilities

Registration, customer login/recovery, user self-service, subscriptions,
balances, payments, redeem/promo codes, affiliates, announcements, and
customer-management APIs are retired. In private mode they are neither linked
from the control plane nor registered as active backend routes; legacy API
prefixes return the stable `CUSTOMER_SURFACE_RETIRED` response.

The upstream source and database schema may still contain compatibility code and
tables. This is not an invitation to enable SaaS mode or to delete historical
schema. Any future removal requires a separate migration and recovery review.

## Route and deployment policy

The browser allowlist is maintained in
`frontend/src/config/singleUserProduct.ts`. The backend product-mode contract is
`backend/internal/config/product_mode.go`. New deployments must set
`RUN_MODE=simple` and `SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE=true`; the
public listener exposes gateway traffic only, while the control listener is
restricted to localhost/WireGuard peers.

See [`../deploy/EDGE_SECURITY.md`](../deploy/EDGE_SECURITY.md) and
[`../deploy/PRODUCTION_ROLLOUT.md`](../deploy/PRODUCTION_ROLLOUT.md) for the
network and release gates.
