# Single-User Cleanup Baseline

Captured: 2026-07-13

## Source and runtime

- Branch: `feat/local-admin-bypass`
- Source HEAD: `8d3aa83b2e3115f92028632f1515c368b1c50288`
- Live image tag: `sub2api:single-user-private-control`
- Live image ID: `sha256:76b7de819debd3568791f86c5be2bfd137f1115364139b07f0146e5ef7c0c8b4`
- Runtime health: healthy

## Largest frontend assets

| Bytes | Asset family |
|---:|---|
| 624088 | AccountsView |
| 430775 | vendor-ui |
| 379706 | SettingsView |
| 304224 | main index JS |
| 222906 | main CSS |
| 220820 | vendor-misc |
| 203999 | OpsDashboard |
| 178340 | vendor-chart |
| 155322 | secondary index JS |
| 111610 | vendor-vue |
| 82281 | RiskControlView |
| 79400 | BatchImageGuideView |
| 74170 | KeysView |
| 74016 | AppLayout |

## Known product-surface debt

- Private admin API-key navigation points to user route `/keys` rather than an admin route.
- Legacy SaaS routes remain as explicit route records backed by `SingleUserGatewayRedirectView.vue`.
- `AppSidebar.vue` constructs a large SaaS menu and filters it for private mode.
- `AppSidebar.vue` calls `adminSettingsStore.fetch()` from both an immediate watcher and `onMounted`.
- Payment/subscription stores and views remain in the source tree.
- Backend still registers payment, customer subscription, redeem, promo, affiliate, announcement, and user-management route families in private mode.
- Settings tab types still include stale `users` and `payment` values; active tabs still include `agreement` and `features`.

## Acceptance targets

- Private primary routes: dashboard, ops, accounts, admin API keys, usage, settings.
- No active payment/subscription/affiliate/redeem/promo/announcement/customer-management routes in private mode.
- No in-app update, rollback, or restart routes.
- Preserve accounts, API keys, routing groups, usage, upstream OAuth, ops, security, and backups.
- No database schema deletion in this cleanup.

## Final comparison

To be populated after implementation.
