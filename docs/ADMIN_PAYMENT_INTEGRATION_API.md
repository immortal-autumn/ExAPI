# Admin payment integration API

English (default) | [简体中文](ADMIN_PAYMENT_INTEGRATION_API.zh-CN.md)

> **Archived upstream reference — not an ExAPI feature.** Customer recharge,
> redeem, balance, and payment-integration endpoints are retired in the
> administrator-only product and return `CUSTOMER_SURFACE_RETIRED` when reached
> through the private-mode backend. **Do not call the endpoints documented
> below against a live ExAPI instance.**

## Purpose

This document describes the minimal ExAPI Admin API surface for external
payment integrations such as `sub2apipay`, including:

- Recharge after payment success.
- User lookup.
- Manual balance correction.
- Purchase-page query-parameter forwarding.

## Base URL

- Production: `https://<your-domain>`
- Beta: `http://<your-server-ip>:8084`

## Authentication

Recommended headers:

- `x-api-key: admin-<64hex>`
- `Content-Type: application/json`
- `Idempotency-Key` for idempotent endpoints

An administrator JWT can also access admin routes, but an Admin API Key is
recommended for server-to-server integration.

## 1. Create and redeem in one step

`POST /api/v1/admin/redeem-codes/create-and-redeem`

Use case: atomically create a redeem code and redeem it to a target user.

Headers:

- `x-api-key`
- `Idempotency-Key`

Request body:

```json
{
  "code": "s2p_cm1234567890",
  "type": "balance",
  "value": 100.0,
  "user_id": 123,
  "notes": "sub2apipay order: cm1234567890"
}
```

Idempotency behavior:

- Same `code` and same `used_by`: `200`
- Same `code` but different `used_by`: `409`
- Missing `Idempotency-Key`: `400` (`IDEMPOTENCY_KEY_REQUIRED`)

curl example:

```bash
curl -X POST "${BASE}/api/v1/admin/redeem-codes/create-and-redeem" \
  -H "x-api-key: ${KEY}" \
  -H "Idempotency-Key: pay-cm1234567890-success" \
  -H "Content-Type: application/json" \
  -d '{
    "code":"s2p_cm1234567890",
    "type":"balance",
    "value":100.00,
    "user_id":123,
    "notes":"sub2apipay order: cm1234567890"
  }'
```

## 2. Query a user (optional pre-check)

`GET /api/v1/admin/users/:id`

```bash
curl -s "${BASE}/api/v1/admin/users/123" \
  -H "x-api-key: ${KEY}"
```

## 3. Adjust a balance

`POST /api/v1/admin/users/:id/balance`

Use case: manual correction with `set`, `add`, or `subtract`.

Request body example (`subtract`):

```json
{
  "balance": 100.0,
  "operation": "subtract",
  "notes": "manual correction"
}
```

```bash
curl -X POST "${BASE}/api/v1/admin/users/123/balance" \
  -H "x-api-key: ${KEY}" \
  -H "Idempotency-Key: balance-subtract-cm1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "balance":100.00,
    "operation":"subtract",
    "notes":"manual correction"
  }'
```

## 4. Purchase/custom-page URL query forwarding

When ExAPI opens `purchase_subscription_url` or a user-facing custom-page
iframe URL, it appends the same parameters for iframe and new-tab modes:

- `user_id`
- `token`
- `theme` (`light` or `dark`)
- `lang` (for example `zh` or `en`, passing the current UI language)
- `ui_mode` (fixed to `embedded`)

Example:

```text
https://pay.example.com/pay?user_id=123&token=<jwt>&theme=light&lang=en&ui_mode=embedded
```

Treat `token` as a credential. Do not log the complete URL or include a live
URL in documentation, tickets, analytics, or referrer-bearing navigation.

## 5. Failure handling

- Persist payment success and recharge success as separate states.
- Mark payment as successful immediately after a verified callback.
- Allow retries for orders where payment succeeded but recharge failed.
- Keep the same `code` for a retry and use a new `Idempotency-Key`.

## 6. Recommended `doc_url`

- View: `https://github.com/immortal-autumn/ExAPI/blob/main/docs/ADMIN_PAYMENT_INTEGRATION_API.md`
- Download: `https://raw.githubusercontent.com/immortal-autumn/ExAPI/main/docs/ADMIN_PAYMENT_INTEGRATION_API.md`
