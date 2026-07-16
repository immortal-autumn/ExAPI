# Recursive Frontend and Product-Surface Inspection

Captured: 2026-07-13

## Inspection model

The frontend was reviewed as a graph:

`router -> layout -> global components -> route view -> retained tab -> nested panel -> store/API -> timer -> generated chunk`

This revealed product leakage that top-level route/tab checks missed.

## Product-mode implementation

`frontend/src/config/singleUserProduct.ts` provides:

- A small retained navigation allowlist.
- A long legacy route-prefix denylist.
- A five-item Settings tab allowlist.

It does **not** define nested panel capabilities, copy domains, API families, timer policies, or emitted-asset policy. Private mode is also inferred in the browser from a hard-coded host list (`localhost`, loopback, and one WireGuard address), rather than a server-injected product capability flag.

This is the central architectural weakness: top-level visibility is declarative; everything beneath a retained route is inherited wholesale.

## Confirmed reachable SaaS/customer surface

### Global layout and onboarding

- `AppLayout.vue` always imports onboarding CSS/store/composable.
- It instantiates `useOnboardingTour({ autoStart: true })` for every layout mount.
- The tour uses a 1-second auto-start timer and can wait/retry for missing elements for up to 8 seconds.
- `AppHeader.vue` correctly hides announcement/subscription/balance widgets using `privateGatewayControlPlane`.
- The replay-tour button uses a different predicate: `!authStore.isSimpleMode && admin`.
- This deployment is a private control plane but the legacy editable “backend/simple mode” setting is false, so the replay control remains visible and the tour auto-starts.
- The tour copy and steps describe account creation, customer users, and other removed SaaS flows.

### Dashboard

`DashboardView.vue` visibly retains:

- Total/new “Users” metric.
- User consumption ranking.
- Multi-user wording and billing/revenue concepts alongside useful gateway/account data.

### Usage

`UsageView.vue` visibly retains:

- “All users” wording.
- User search and user-ranking tab.
- User balance history modal.
- Billing type, billing mode, group/pricing, standard cost, and customer dimensions.

Request/account/model/error diagnostics are product-aligned; user/billing dimensions are not.

### API Keys

The retained key view visibly includes:

- “Import to CCS”.
- “Use key” workflow and customer-shaped group/rate terminology.
- Calls to `/api/v1/groups/available`, `/api/v1/groups/rates`, `/api/v1/settings/public`, and `/api/v1/usage/dashboard`.

Key creation/listing remains useful, but this route is coupled to the customer frontend/API model.

### Accounts and proxies

Operator-relevant account/proxy management remains, but hidden menus expose:

- CRS synchronization.
- Import/export.
- Account multiplier/billing concepts.
- Batch editing and bulk testing.
- Proxy import/export.

Credential-bearing exports need explicit security review. `ProxyService.TestConnection()` is a backend no-op despite the visible control.

## Settings: recursive retained-tab findings

### General

Visible panels include:

- Legacy backend/simple mode.
- Login/register subtitle and customer contact/support copy.
- Redeem/profile wording.
- Arbitrary homepage HTML with iframe/CSP warning.
- Custom user/admin iframe menus.
- CCS import configuration.

Most are outside a minimal private gateway cockpit.

### Security

`SecuritySettingsTab.vue` unconditionally mounts all of:

1. Admin integration API key — operator-relevant but high impact.
2. Customer registration policy and email-domain allowlist.
3. API-key access/IP controls — operator-relevant.
4. LinuxDo OAuth.
5. GitHub/Google login OAuth.
6. WeChat connect.
7. DingTalk connect.
8. Generic OIDC connect.

Only items 1 and 3 clearly belong. The rest are confirmed live DOM, not dormant source.

### Email

`EmailSettingsTab.vue` unconditionally mounts:

- SMTP and test email.
- Customer subscription-expiry notification.
- Full customer email-template editor (verification, password reset, subscription, balance/recharge, etc.).
- Balance-low notification.
- Account quota notification.

Only SMTP transport, test email, operator/account-limit alerts, security policy alerts, and Ops alert/report mail appear potentially product-aligned. The current tab is overwhelmingly SaaS-oriented.

### Gateway

Mostly product-aligned routing/account/session/retry/proxy/Ops behavior. It should still be decomposed and reviewed panel-by-panel because customer quota/notification concepts coexist with gateway controls.

### Backup

Product-aligned. Secret masking/encryption and dump confidentiality are separate backend findings.

## Additional retained-route leakage found by recursive review

### Customer Profile

`AppHeader.vue` always links to `/profile`, and private-mode route tests explicitly allow it. `ProfileView.vue` renders customer identity/OAuth status, support/contact, password, balance-notification preferences, and TOTP, and loads public settings for LinuxDo, DingTalk, WeChat, and OIDC enablement.

Disposition: remove `/profile` from private navigation and redirect it to operator Security or Dashboard. Move genuinely required administrator password/TOTP controls into an operator-specific security surface.

### Batch Image discovery and route

Dashboard and Sidebar both invoke `useBatchImageAccess()` in private mode. They page through API keys to discover entitlement and can surface a Batch Image quick action/documentation link. `/batch-image` itself remains an authenticated route and contains customer key/group, balance-freeze, pricing, billing, and job-settlement semantics. This is a hidden request and reachable product-decision path, not merely an emitted chunk.

Disposition: classify Batch Image as retained or removed before changing workers. If removed, disable entitlement discovery, route, action, API family, worker, and chunk through the shared capability contract.

### Public Key Usage

`/key-usage` is an unauthenticated route explicitly allowed by private-mode routing. The current view exposes subscription, plan, balance, and expiry concepts. If public key diagnostics are a gateway requirement, rebuild it around request/quota diagnostics; otherwise remove it. Test both private and public ingress.

### Custom content and channel deep links

`/custom/:id` remains authenticated and directly renders configured Markdown/HTML or an iframe; custom admin menu items are explicitly appended in private mode. `/monitor`, `/admin/channels/pricing`, and `/admin/channels/monitor` also remain directly routable. Channel pricing is a mutable customer group/model-pricing surface.

Disposition: make an explicit retain/remove decision for each route. Hidden navigation is not enforcement.

### Public auth/setup negative matrix

Login, registration, password recovery, email verification, OAuth callbacks, setup, payment callbacks, and legal routes remain registered. Acceptance must enumerate every public root at both private and public ingress and assert the intended `404`, redirect, or retained behavior.

## Hidden but bundled source

`SettingsView.vue` is still a ~3,800-line legacy orchestrator. It:

- Statically imports Agreement, Features, Users, Payment tabs and payment dialogs even though those tabs are not selected.
- Imports affiliate APIs/state and customer subscription/payment types.
- Keeps large dormant form state and handlers.
- Correctly guards `loadSubscriptionGroups()` and `loadProviders()` in private mode.
- Still loads all retained settings plus admin key, cooldown, timeout, rectifier, and beta-policy endpoints on mount.

The guarding prevents sampled dormant network calls but does not remove code/bundle/maintenance surface.

## Timers and network lifecycle

### Correctly disabled in private browser mode

`App.vue` guards:

- Subscription preload and 5-minute polling.
- Announcement preload, 3-second delayed refresh, route-change refresh, and visibility refresh.
- Announcement popup rendering.

Live browser performance entries confirmed no payment/subscription/announcement/redeem/customer-list calls during retained-route navigation.

### Still active

- Onboarding 1-second auto-start and DOM polling.
- Route prefetch: Dashboard load fetched the Accounts chunk.
- Route-specific operator polling/metrics where expected.

## Router/deep-link behavior

- Legacy customer and commercial routes redirect to retained control-plane pages.
- `/admin/risk-control` is defined and emits an ~89 KB chunk but redirects to Settings when `risk_control_enabled` is false.
- This conflicts with the stated desire to retain security/risk controls: the dedicated screen is not reachable in the current configuration, while a limited subset of controls appears in Settings.
- Public login/register/OAuth chunks remain emitted even though private route guards redirect them.

## Bundle evidence

Fresh build:

- Total: 4.71 MB / 145 files.
- 29 suspect customer/dormant-named assets: ~609 KB.
- Examples: Register, Login, customer Profile, OAuth authorization/callback, announcement bell, subscription progress, payment callback, Batch Image Guide, Risk Control, customer Email Settings.

Some channel assets may remain valid for upstream operator monitoring; the rest should be removed from private product roots rather than merely hidden.

## Test adequacy finding

`SettingsView.operator.spec.ts` checks:

- Five retained tabs are lazy-loaded.
- Four obsolete top-level tabs are not mounted via one exact `v-show` string.
- Two customer/commercial loaders sit inside a private-mode guard.

It does **not** recursively inspect retained-tab children. It therefore passes while Security renders registration/OAuth/connect panels and Email renders subscription/balance/customer templates.

Other source-string tests similarly assert the presence of guard code rather than runtime absence of reachable controls, requests, timers, and chunks.

## Required capability architecture

A runtime capability manifest is necessary but insufficient. It must be:

1. Versioned and default-deny, with stable IDs for routes, public pages, panels, actions, API families, workers, timers, integrations, data dimensions, and build roots.
2. Injected authoritatively by the server before router/app initialization; hostname inference and editable legacy mode must not select the product.
3. Enforced server-side for route registration, API families, worker construction, and integrations; browser flags are not authorization.
4. Enforced at build time through a private-product entry point or compile-time registry so retired roots are not emitted.
5. Bidirectionally tested: every enabled capability maps to an implementation, and every disabled capability is absent from router, DOM, network, timer graph, backend route/worker graph, and generated assets.

## Required frontend acceptance tests

1. Define a nested panel capability matrix for every retained tab.
2. Mount each retained route/tab in private mode and recursively assert an explicit component allowlist.
3. Assert forbidden text/control taxonomy across the rendered DOM.
4. Record network calls for >5 seconds and fail on forbidden API families.
5. Assert no onboarding timers/components/styles in private mode.
6. Assert generated manifest/chunk names exclude retired route roots.
7. Verify deep links redirect without loading forbidden chunks.
8. Test header predicates from the single product-mode capability source, not legacy simple-mode state.
