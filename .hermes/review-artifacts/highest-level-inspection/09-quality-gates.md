# Quality Gates and Test Adequacy

Captured: 2026-07-13 20:41–20:44 GMT

## Results

- Frontend Vitest: **191 files passed; 984 tests passed; 11 skipped**.
  - Authoritative completed-process duration: **121.37 seconds**.
- Frontend TypeScript/Vue type-check: **passed**.
- Frontend production build: **passed**.
- Frontend bundle budget: **passed**.
  - AccountsView: 153.34 KB / 180 KB
  - SettingsView: 199.31 KB / 210 KB
  - OpsDashboard: 222.38 KB / 230 KB
- Backend `go test ./...`: **passed**.
- Backend `go vet ./...`: **passed**.

## Warnings

- Browserslist/caniuse data is seven months old.
- Vite reports multiple modules that are both dynamically and statically imported, so dynamic imports do not isolate them into separate chunks.
- Component tests emit many Vue warnings when stubbing select components (`Failed setting prop "options" on <select>`). Tests still pass, but warning volume reduces failure-signal quality.
- Several expected-error tests write stack traces to stderr.

## Test-portfolio finding

The passing suite is not evidence that the appliance surface is minimal. The suite actively executes tests for dormant or out-of-product features, including:

- Registration settings and registration-email policy.
- LinuxDo, WeChat, DingTalk, OIDC, GitHub/Google-style OAuth flows.
- Payment providers and checkout/result pages.
- Subscription store, subscription plans, expiry reminders, and balance alerts.
- User-default subscriptions and platform quotas.
- Redeem-code administration.
- Announcements.
- User profile/TOTP/customer identity bindings.
- Customer email templates and SMTP flows.
- System rollback API.
- Dedicated RiskControl view even though the route redirects in the deployed product.

The current tests primarily validate that inherited SaaS functionality continues to work, while the appliance tests validate top-level route/tab restrictions and dormant fetch suppression. They do not recursively assert that retained views contain only allowlisted operator capabilities.

## Missing acceptance coverage

Add future tests that:

1. Mount every retained route and recursively reject customer-registration, payment, subscription, promotion, redeem, user-management, updater, and restart text/components.
2. Assert `SecuritySettingsTab` contains only approved operator panels.
3. Assert Email settings either disappear or expose only approved operator alerts.
4. Assert Dashboard and Usage omit user-ranking/billing/customer dimensions.
5. Assert the onboarding tour never mounts in private appliance mode.
6. Assert retained routes never request retired API families after delayed timers fire.
7. Assert built chunks for the initial/retained route graph do not include forbidden feature chunks.
8. Assert `/admin/risk-control` behavior matches the product contract.

## Deployment drift

- Source commit: `44613bfaf950b76d066f01cd763444b52f80f7c5`.
- Deployed image ID: `sha256:e8904927e22a7859a7d5ef50a036e542f79b900274d060811bce83b509803e49`.
- Container label revision and a deterministic source-to-image provenance label are absent, so exact binary provenance cannot be cryptographically established from image metadata alone.
- Runtime behavior and built source are consistent on the sampled routes, but that is weaker than reproducible provenance.
