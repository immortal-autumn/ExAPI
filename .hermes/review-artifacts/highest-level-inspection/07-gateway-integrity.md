# Gateway Boundary and Integrity Checks

Captured: 2026-07-13

## Unauthenticated public checks

- `/health`: `200`.
- `/v1/models` without credentials: `401`.
- `/v1/models` with a synthetic invalid bearer token: `401`.
- Public control-plane and legacy customer paths: `404`.
- Private direct control-plane paths: expected `200`, `401`, or `405` depending method/auth.

No response body containing data or credentials was retained.

## Compatibility endpoints

Without credentials:

- `/v1beta/models`: `401`.
- `/backend-api/codex/responses`: `401`.
- `/responses`: `401`.
- `/images/generations`: `401`.
- `/antigravity/v1/chat/completions`: `404` because that exact route is not registered.
- `/chat/completions`: `200 text/html` (SPA interception defect).
- `/embeddings`: `200 text/html` (SPA interception defect).
- `/videos` and the `/videos/*` family are not bypassed by the embedded frontend predicate and can return SPA HTML instead of API authentication/handling.

The root-alias defect is described in `03-backend-workers.md`.

## Authenticated integrity

A fresh authenticated upstream request was not executed because:

- The inspection is read-only.
- Creating a disposable key would mutate runtime state.
- Extracting the existing real key from plaintext database storage would violate the plan’s no-secret rule.

Existing evidence shows one prior gateway request recorded successfully with account attribution, token counts, endpoint mapping, and cost data. This supports but does not replace a fresh end-to-end test.

Therefore authenticated integrity is **partially verified / freshness blocked**, not failed.

## API-key presentation

- Cockpit list masks the key to prefix/suffix.
- No raw key was exposed in browser snapshots or reports.
- Database storage is plaintext and is covered separately as a security finding.

## Product integrity

- Core operator paths for accounts, API keys, groups, usage, routing, policies, quotas, and errors remain operational.
- Public customer/control-plane boundaries are correct for sampled paths.
- The compatibility alias middleware defect makes some documented/public paths nonfunctional.
- API Keys cockpit still depends on customer-shaped key/group/public-settings/usage endpoints.

## Required acceptance test after remediation

With an explicitly approved disposable key:

1. Call `/v1/models` and one minimal completion.
2. Call each intended compatibility alias.
3. Verify successful upstream account selection and token accounting.
4. Verify invalid/missing keys remain `401`.
5. Revoke/delete the disposable key and verify it immediately fails.
6. Record only statuses, model names, and request IDs—not the key or response content.
