# Single-User Runtime and Database Evidence

Captured: 2026-07-13

## Frontend production chunks

After operator-tab and account-dialog splitting:

| Chunk | Baseline | Current | Decision |
| --- | ---: | ---: | --- |
| AccountsView | 624.18 KB | 153.34 KB | 180 KB enforced ceiling |
| SettingsView | 379.99 KB | 199.24 KB | 210 KB enforced ceiling |
| OpsDashboard | 203.51 KB | 222.38 KB | 230 KB enforced ceiling; increase is shared-chunk reassignment |
| KeysView | not separately budgeted | 80.44 KB | No additional split justified |

The build-time checker is `frontend/scripts/check-bundle-budget.mjs`, exposed by `npm run check:bundle`.

## PostgreSQL read-only observations

- Largest table: `ops_system_logs`, approximately 11 MB with about 10.9k live rows.
- `ops_system_metrics`: approximately 2.3 MB with about 5.7k live rows.
- Gateway tables (`accounts`, `usage_logs`, `api_keys`) are each below 400 KB.
- Hot gateway tables show substantial index use relative to sequential scans.
- The `users` table has a small dead-tuple count but is only approximately 232 KB.
- Many zero-scan indexes correspond to dormant SaaS tables or a one-account installation; this is not sufficient evidence for destructive index removal.

## Decision

Do not add indexes, remove indexes, rewrite queries, vacuum manually, or change retention based on this dataset. The installation is too small for database-level changes to provide measurable benefit, while speculative changes would add operational risk.

## Dormant compatibility code

Frontend customer/payment settings logic remains source-coupled inside `SettingsView.vue`, but its tabs are absent from the operator surface, its panels are conditionally unmounted, and its subscription/payment/affiliate loaders are disabled on private control-plane hosts. A wholesale deletion would require a high-risk rewrite of a 4,000-line component for no additional runtime-request benefit. Backend route families and the subscription-expiry worker are disabled by product mode while database schema and dependency constructors remain intact for migration compatibility.

## Final validation and deployment

- Backend: `GOTOOLCHAIN=auto go test ./...` passed.
- Frontend: 191 test files passed; 984 tests passed; 11 retired Payment/Users UI cases were explicitly skipped after those tabs left the product surface.
- `vue-tsc -b`, production build, and bundle-budget enforcement passed.
- Deployed image: `sha256:e8904927e22a7859a7d5ef50a036e542f79b900274d060811bce83b509803e49`.
- Only the `sub2api` application container was recreated; PostgreSQL and Redis remained healthy and untouched.
- Live private checks: `/health`, `/admin/dashboard`, `/admin/api-keys`, and `/admin/settings` returned `200` and rendered in the browser.
- Live retired surfaces: registration, payment administration, multi-user administration, announcements, public login, and updater checks returned `404`.
- Public `/v1/models` without credentials returned `401`, preserving the authentication boundary.
- Fresh browser resource inspection after the prior three-second timer window found no requests to `/subscriptions/active`, `/announcements`, or `/admin/payment/config`; console and JavaScript error counts were zero.
- Authenticated `/v1` content was not rerun because no raw API-key secret was retained or exposed. The earlier authenticated model-list and `codex-auto-review` completion regression remains the latest credentialed evidence.
