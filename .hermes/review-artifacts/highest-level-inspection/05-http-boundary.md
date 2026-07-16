# HTTP Route Boundary Evidence

Captured: 2026-07-13 20:31 GMT

Only HTTP methods, paths, and status codes were recorded. No credential or response-body data was saved.

| Scope | Method | Path | Status |
|---|---|---|---:|
| public | GET | `/` | 404 |
| public | GET | `/login` | 404 |
| public | GET | `/register` | 404 |
| public | GET | `/admin/dashboard` | 404 |
| public | GET | `/api/v1/auth/me` | 404 |
| public | GET | `/api/v1/auth/register` | 404 |
| public | GET | `/api/v1/admin/users` | 404 |
| public | GET | `/api/v1/admin/payment/config` | 404 |
| public | GET | `/api/v1/subscriptions/active` | 404 |
| public | GET | `/api/v1/announcements` | 404 |
| public | GET | `/api/v1/admin/system/check-updates` | 404 |
| public | POST | `/api/v1/admin/system/update` | 404 |
| public | POST | `/api/v1/admin/system/restart` | 404 |
| public | GET | `/v1/models` | 401 |
| local | GET | `/` | 200 |
| local | GET | `/admin/dashboard` | 200 |
| local | GET | `/admin/accounts` | 200 |
| local | GET | `/admin/api-keys` | 200 |
| local | GET | `/admin/settings` | 200 |
| local | GET | `/api/v1/auth/me` | 401 |
| local | GET | `/api/v1/auth/register` | 404 |
| local | GET | `/api/v1/admin/users` | 404 |
| local | GET | `/api/v1/admin/payment/config` | 404 |
| local | GET | `/api/v1/subscriptions/active` | 404 |
| local | GET | `/api/v1/announcements` | 404 |
| local | GET | `/api/v1/admin/system/check-updates` | 404 |
| local | POST | `/api/v1/admin/system/update` | 404 |
| local | POST | `/api/v1/admin/system/restart` | 404 |
| local | GET | `/v1/models` | 401 |

## Interpretation

- Public control/SaaS routes are hidden.
- Public and local `/v1/models` reject missing credentials.
- Private SPA routes are reachable.
- Raw local auth API does not grant anonymous admin access.
