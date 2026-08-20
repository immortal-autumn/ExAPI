# Account probes and provider usage refresh

This document describes the operator-visible account diagnostics introduced in
ExAPI v0.2.5. It distinguishes manual diagnostics from scheduler health and
from provider usage/quota metadata.

## Three independent signals

| Signal | Source | Purpose | May change scheduling? |
|---|---|---|---|
| Account status and `schedulable` | Scheduler, recovery, and explicit admin operations | Determines whether the account may receive traffic | Yes |
| Manual test snapshot | Operator action: `POST /api/v1/admin/accounts/:id/test` | Records what the latest manual provider probe observed | No |
| Usage/quota snapshot | Passive headers or active provider usage query | Displays provider windows, model quotas, entitlement, and refresh errors | No, unless a separate provider policy explicitly does so |

Do not infer one signal from another. A provider may return quota metadata while
inference is rate-limited, and a transient manual failure must not silently
remove an account from the scheduler.

## Manual snapshot schema

The latest manual result is stored under
`account.extra.account_test_probe`. The bounded JSON object contains:

```json
{
  "status": "success | failed",
  "checked_at": "RFC3339 UTC timestamp",
  "model": "resolved provider model",
  "reason": "ok | quota_exhausted | authentication_failed | request_failed",
  "http_status": 429
}
```

`http_status` is present only when a status code can be classified. The object
must never contain credentials, access/refresh tokens, raw upstream bodies, or
provider-private metadata.

Persistence is identity-checked and transactional. It uses proxy-before-account
lock ordering, tolerates short-lived OAuth token rotation, rejects concurrent
credential/type/proxy/routing changes, and uses a detached bounded context so a
completed probe is not lost merely because the browser request closes.

## Lifecycle and invalidation

- Only an explicit/manual account test writes the snapshot.
- Scheduled and background tests leave the last manual result unchanged.
- Material credential, platform/type, routing, or proxy changes clear it.
- Proxy edits, expiry, and fallback transitions invalidate affected accounts.
- Duplicated accounts do not inherit the source account's result.
- Admin create/update/bulk and CRS inputs cannot inject the managed field.
- CRS refresh preserves it only when stable provider identity is unchanged.

## Admin UI behavior

The account list renders **Probe Failed** separately from the normal account
status badge. Its localized tooltip contains the coarse reason, resolved model,
HTTP status when known, and checked time. The tooltip is available by keyboard,
pointer, and touch interaction. Closing the test modal reloads the account list
so the latest snapshot is visible.

## Antigravity forced refresh

The account table refresh action and the per-row live-query action request:

```http
GET /api/v1/admin/accounts/:id/usage?source=active&force=true
X-ExAPI-Control-Request: 1
```

For Antigravity accounts, `force=true` bypasses both normal cache checks and
uses a force-specific singleflight key. This prevents a concurrent cached
request from satisfying an operator's live query.

A successful response proves that the usage endpoint was reachable. It does
not prove that inference for every listed model is currently accepted. Confirm
inference separately with a manual test using a model advertised by the live
quota result.

## Troubleshooting sequence

1. Confirm the application and control listener are reachable from an
   allowlisted operator peer.
2. Run a forced active usage refresh and note entitlement, advertised models,
   reset times, and any machine-readable error.
3. Select a currently advertised model and run a manual account test.
4. Reload the account list and compare the manual snapshot with account status
   and `schedulable`.
5. If the probe reports 401/403, reauthorize or repair credentials; do not copy
   the raw provider response into tickets or documentation.
6. If it reports 429/quota exhaustion, wait for the documented reset or resolve
   entitlement with the provider. Repeated retries can extend provider-side
   throttling.
7. If usage succeeds but inference repeatedly returns 429, record both facts:
   quota metadata and inference admission are separate provider surfaces.

## Regression coverage

Backend coverage includes classification, credential-free persistence,
scheduler independence, background-test isolation, OAuth-rotation identity,
repository CAS/lock ordering, admin lifecycle invalidation, CRS behavior, and
forced Antigravity cache bypass. Frontend coverage verifies the probe badge,
accessible tooltip, list reload, and `force=true` wiring.
