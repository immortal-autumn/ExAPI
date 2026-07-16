# Performance, Dependencies, and Container Posture

Captured: 2026-07-13

## Runtime footprint

At the sampled idle point:

- Application: ~122 MiB memory, ~0.14% CPU, 24 PIDs.
- PostgreSQL: ~71 MiB memory, ~0.13% CPU, 12 PIDs.
- Redis: ~5 MiB memory, ~0.13% CPU, 6 PIDs.
- Application image: ~109.9 MB.

These are acceptable for the host, but no cgroup memory/CPU/PID limits are configured.

## Frontend bundle

Fresh production build:

- 145 files.
- 4,707,867 bytes total.
- SettingsView: 204,095 bytes, 199.31 KiB against a 210 KiB budget.
- OpsDashboard: 227,718 bytes, 222.38 KiB against a 230 KiB budget.
- AccountsView: 157,022 bytes, 153.34 KiB against a 180 KiB budget.
- Main vendor UI chunk: 430,775 bytes.
- Main application chunk: 304,224 bytes.

Settings and Ops are close to their current budgets; a small change can exceed them.

A filename-based inventory identified 29 customer/dormant-looking chunks totaling ~609 KB. Some channel/monitor chunks may be operator-relevant, but clearly dormant outputs include Register, Login, customer Profile, OAuth authorization/callback, announcement bell, subscription progress, payment callback, and the unreachable RiskControl view.

Vite also warns that several modules are both static and dynamic imports, preventing intended chunk isolation.

## Dependency audit

`pnpm audit --prod --json` reported:

- Critical: 0
- High: 2
- Moderate: 21
- Low: 3

High findings:

1. `xlsx@0.18.5` prototype pollution — GHSA-4r6h-8v6p-xvw6.
2. `xlsx@0.18.5` regular-expression denial of service — GHSA-5pgg-2g8v-p4x9.

Source tracing found `xlsx` dynamically imported to generate Usage exports; no production spreadsheet ingestion path was found. The package should still be replaced/upgraded, but the audit severity does not establish a malicious-file exploit path in this deployment.

Moderate findings include multiple DOMPurify XSS/sanitization bypasses, PostCSS XSS, Mermaid injection/DoS issues, YAML stack exhaustion, and UUID bounds issues. DOMPurify is particularly relevant because the product retains configurable Markdown/HTML/homepage surfaces.

The current pnpm emitted a warning that the `pnpm.overrides` field in `package.json` is ignored by this pnpm version, so intended transitive pinning may not apply.

`govulncheck` is not installed, so Go advisory scanning was not performed. Go tests and vet passed.

## Database performance

- Ops logs dominate current storage (~12.9 MB).
- Scheduler outbox has very high sequential scan counts relative to one row.
- Empty payment orders are polled repeatedly.
- `pg_stat_statements` is absent, preventing query-level ranking without a database change.

No present capacity crisis was observed.

## Container hardening

All three containers are non-privileged and use `unless-stopped`, but currently have:

- No explicit non-root `User` in image configuration.
- Writable root filesystems.
- No explicit capability drops.
- No custom security options.
- No memory, CPU, or PID limits.

Network publication is appropriately narrow for the application: loopback and WireGuard bindings only. PostgreSQL and Redis are not externally published.

## Priority actions

1. Replace/upgrade `xlsx` while preserving export behavior; do not introduce ingestion without parser limits and malicious-file tests.
2. Upgrade DOMPurify and remove unnecessary arbitrary HTML/iframe surfaces.
3. Remove dormant frontend routes/import roots so Vite stops emitting customer chunks.
4. Split Settings and Ops before they exceed budgets.
5. Stop dormant pollers to eliminate empty-table churn.
6. Add non-root image users, cap drops, resource limits, and read-only filesystems where compatible.
7. Add reproducible build provenance labels tying image to source commit.
