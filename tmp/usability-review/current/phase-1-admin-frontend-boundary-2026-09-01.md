# Phase 1 — administrator frontend boundary

Date: 2026-09-01 (Europe/London)
Branch: `revision/exapi-v0.2.1`

## Scope

- Removed the global Driver.js onboarding entry from `AppLayout` and the sidebar
  route-click bridge.
- Removed the onboarding store/composable/step definitions and onboarding-only
  stylesheet; removed the now-unused `driver.js` dependency and lock entries.
- Removed onboarding callbacks from administrator group and API-key workflows.
- Removed onboarding locale payloads from English and Chinese bundles.
- Renamed remaining stable UI hooks from `data-tour` to `data-testid` so admin
  workflows keep deterministic test selectors without a tour runtime.

## Verification

```text
Focused Vitest (layout, key, group, account workflows): 8 files / 97 tests PASS
Frontend full Vitest after this boundary change: 264 files / 1,421 tests PASS
Frontend typecheck: PASS
Frontend production build: PASS (672 modules transformed)
Bundle budget: PASS
Private bundle budget: PASS
Changed-file ESLint: PASS
git diff --check: PASS
```

Static boundary checks:

- No `driver.js`, `useOnboardingTour`, `useOnboardingStore`, `onboarding.css`,
  or `data-tour` references remain in `frontend/src`.
- Private layout no longer creates timers, listeners, local-storage keys, or
  replay callbacks for onboarding.
- No backend route, API, database, provider credential, or OPC production state
  was changed.

Deployment: **NOT DEPLOYED**. This is a review-branch-only change; wait for
remote CI and security scanning before considering promotion.
