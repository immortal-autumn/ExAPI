# Live Browser Runtime Inspection

Captured: 2026-07-13 20:32–20:42 GMT

Target: deployed private cockpit at `http://127.0.0.1:8027`.

Method: direct route navigation, accessibility-tree inspection, five-second delayed-timer wait where relevant, resource-path inspection, loaded-asset inspection, and console/JavaScript-error inspection. No forms were submitted and no mutating control was invoked. Account identifiers and credential-adjacent values are intentionally omitted.

## Global navigation

Visible sidebar:

- Dashboard
- Ops monitoring
- Accounts
- IP management (upstream proxies)
- Usage
- API Keys
- Settings

No public login, registration, payment, subscription, affiliate, promotion, redeem, updater, rollback, or restart navigation appeared.

## Dashboard

### Retained/operator-relevant

- Single-user cockpit summary.
- Account readiness, quota watch, errors, upstream routing, endpoint-copy helpers.
- Request/token/performance and model distribution.

### Product-surface mismatches

- “Users” total/new-user card remains visible.
- “User consumption ranking” remains visible.
- Significant English cockpit copy remains despite the intended Chinese-only frontend.
- Route prefetch loaded the Accounts view chunk during Dashboard load.

### API paths

```text
/api/v1/auth/local-admin
/api/v1/admin/compliance
/api/v1/admin/settings
/api/v1/keys
/api/v1/admin/dashboard/snapshot-v2
/api/v1/admin/dashboard/users-trend
/api/v1/admin/dashboard/users-ranking
/api/v1/admin/accounts
```

No console or JavaScript errors. No subscription/announcement/payment-config request was observed after five seconds.

## Ops monitoring

Operator-relevant monitoring, diagnostics, throughput, latency, errors, account availability, alerts, runtime logging, and system logs rendered successfully.

Potential review items:

- “Switch to user view” remains present in concurrency/queue controls.
- Alerting is described as email-only, coupling Ops to retained email infrastructure.
- Log cleanup/runtime logging settings are powerful controls and need explicit authorization/audit classification.

API paths were limited to Ops, settings, groups, compliance, keys, and auth. No suspect SaaS chunk was loaded. Console/JavaScript errors: zero.

## Accounts

Operator-relevant account, scheduling, usage-window, proxy, TLS fingerprint, error passthrough, and account test controls rendered successfully.

Review items:

- Header copy still says account and Cookie management.
- “Privacy status” terminology remains.
- “More operations” contains CRS sync, import, export, account multiplier, and bulk-edit functionality.
- Import/export may handle OAuth or proxy credentials and requires secret/authorization review; it was not invoked.
- CRS synchronization requires a product decision as an external integration.

Observed API paths were account, proxy, group, usage, and batch-stat endpoints only. Console/JavaScript errors: zero.

## API Keys

- Key list rendered and displayed only a masked key prefix/suffix.
- Create, disable, edit, delete, group, usage, and endpoint-use controls are operator-relevant.

Review items:

- “Import to CCS” remains visible.
- The operator route reuses customer/user APIs: `/groups/available`, `/groups/rates`, `/settings/public`, and `/usage/dashboard/api-keys-usage`.
- This creates source/runtime coupling to customer-facing route families despite the operator URL.

No raw key was read or recorded. Console/JavaScript errors: zero.

## Usage

Core request/account/model/endpoint/token/latency/error diagnostics are operator-relevant.

Visible non-product dimensions:

- Subtitle says “all users.”
- User ranking and email-based user search.
- Billing type, billing mode, group distribution, actual/cost/standard price displays.
- User column and customer-centric usage ranking.
- Excel export and cleanup require authorization/data-leak review.

API paths were admin usage/dashboard/group endpoints. No dormant SaaS API request or suspect chunk was observed. Console/JavaScript errors: zero.

## Settings — General

Visible non-product controls:

- Backend mode framed around registration/public pages/self-service.
- Site subtitle described as login/register copy.
- CCS import configuration.
- Customer support field described for redeem/profile surfaces.
- Public documentation and homepage controls.
- Arbitrary homepage Markdown/HTML/iframe configuration.
- Custom iframe menu pages with user/admin visibility.

These are visible and editable, not merely dormant source.

## Settings — Security

Operator-relevant controls:

- Administrator integration API key (high privilege; needs explicit product decision).
- Gateway API-key client-IP/trusted-proxy behavior.

Visible out-of-product controls:

- Open registration.
- Email verification and registration-domain whitelist.
- Promo-code and invitation-code registration.
- Customer 2FA.
- Turnstile for login/registration.
- LinuxDo login.
- GitHub/Google email-based login and automatic registration.
- WeChat login.
- DingTalk login.
- OIDC customer login.

This is the confirmed root mismatch: `SecuritySettingsTab` combines two operator controls with the complete customer-authentication stack.

## Settings — Gateway

The tab is large but mostly gateway/operator behavior: overload/429 cooldown, timeouts, request rectification, Anthropic beta policy, OpenAI service-tier policy, model/routing behavior, usage records, and hardening.

Source inspection must classify notification/quota panels and any subscription/balance references that were below the accessibility snapshot truncation boundary.

## Settings — Email

The visible tab is predominantly customer/SaaS infrastructure:

- Email verification dependency warning.
- Subscription-expiry reminder switch.
- Verification-code template.
- Password-reset template.
- Subscription activation/expiry templates.
- Low-balance and recharge-success templates.
- Registration/OAuth/TOTP customer copy.

Potentially operator-relevant templates:

- Account-limit alert.
- Content/security policy notices.
- Ops alert.
- Ops scheduled report.

The tab should be removed or reduced to operator alerts after a product decision.

## Settings — Backup

Product-aligned controls rendered:

- S3-compatible storage.
- Scheduled backup.
- Retention count/days.
- Manual backup records.

Credential input values were not read. Source inspection must verify masking, encrypted storage, API serialization, backup contents, and authorization.

## Settings network/chunks

After opening all retained tabs, API paths included operator settings, gateway policy, proxies, email templates/preview, and backup endpoints. The private-mode loader guard prevented subscription/payment APIs, but the Email tab still requested SaaS-oriented template data.

Loaded settings chunks included:

```text
GeneralSettingsTab
GatewaySettingsTab
EmailSettingsTab
BackupSettingsTab
SecuritySettingsTab
registrationEmailPolicy
```

Console/JavaScript errors: zero.

## Risk-control deep link

Direct navigation to `/admin/risk-control` redirected to `/admin/settings`. The dedicated Risk Control view is not reachable even though the product description says security/risk controls are retained. This requires a product decision and route/source reconciliation.

## IP management

The route is upstream proxy management and is operator-relevant. Visible controls include connection tests, batch quality checks, import/export, and add/delete operations. Import/export requires credential-handling review. No proxy credentials were read.

## Browser conclusions

1. Public/customer route removal is not enough: substantial customer/SaaS functionality remains visible inside retained routes.
2. The largest mismatches are Security, Email, General Settings, Usage user/billing dimensions, Dashboard user metrics, onboarding, and API Keys’ customer-route reuse.
3. Runtime is currently stable: no console errors and no repeated retired-SaaS 404 requests were observed.
4. The minimal appliance goal requires nested-component allowlists and browser DOM/network acceptance tests, not only top-level route/tab tests.
