# ExAPI documentation map

Documentation is split into living project/operator guidance, feature
contracts, compatibility references, and historical design evidence.

## Living project and operations documents

- [`PROJECT_STATUS.md`](PROJECT_STATUS.md) — canonical dated release,
  deployment, validation, and known-external-condition record.
- [`ACCOUNT_PROBES.md`](ACCOUNT_PROBES.md) — manual probe and forced usage
  refresh semantics.
- [`../development.md`](../development.md) — active development priorities and
  mandatory quality gates.
- [`../deploy/README.md`](../deploy/README.md) — deployment entry point.
- [`../deploy/PRODUCTION_ROLLOUT.md`](../deploy/PRODUCTION_ROLLOUT.md) —
  recovery, canary, promotion, observation, and rollback gates.
- [`../deploy/EDGE_SECURITY.md`](../deploy/EDGE_SECURITY.md) — public/control
  listener and reverse-proxy security.

## Compatibility and upstream control

- [`UPSTREAM_COMPATIBILITY.md`](UPSTREAM_COMPATIBILITY.md) — retained `sub2api`
  runtime identifiers and migration boundaries.
- [`UPSTREAM_LOCK.md`](UPSTREAM_LOCK.md) — audited upstream baseline and lock
  verification.
- [`../backend/migrations/README.md`](../backend/migrations/README.md) — forward
  migration behavior.
- [`../backend/resources/model-pricing/README.md`](../backend/resources/model-pricing/README.md)
  — embedded pricing resource contract.

## Feature contracts

- [`ASYNC_IMAGE_TASKS.md`](ASYNC_IMAGE_TASKS.md)
- [`BATCH_IMAGE_MVP.md`](BATCH_IMAGE_MVP.md)
- [`COMPOSITE_GROUPS.md`](COMPOSITE_GROUPS.md)
- [`PAYMENT.md`](PAYMENT.md) and [`PAYMENT_CN.md`](PAYMENT_CN.md)
- [`ADMIN_PAYMENT_INTEGRATION_API.md`](ADMIN_PAYMENT_INTEGRATION_API.md)

Feature documents describe their named subsystem and may include optional
multi-user capabilities that are not enabled by the private deployment.

## Developer and design references

- [`../DEV_GUIDE.md`](../DEV_GUIDE.md) — repository development conventions.
- [`design/EXAPI_UI_DIRECTION.md`](design/EXAPI_UI_DIRECTION.md) — private
  operator UI direction.
- Component-local `README.md`, `INTEGRATION.md`, and `EXAMPLES.md` files under
  `frontend/src/` document only their owning router, store, view, or component.
- [`../skills/sub2api-admin/SKILL.md`](../skills/sub2api-admin/SKILL.md) and its
  references describe the checked-in admin automation skill.

## Historical evidence

Files under `openspec/changes/` are design, source-freeze, implementation, and
verification evidence for a particular change. Treat those as historical
records. When current behavior supersedes them, add a link to a living document
instead of editing the original evidence to resemble the present.

## Maintenance rules

- Put each changing fact in one canonical location and link to it elsewhere.
- Pin production artifacts by digest; do not document `latest` as a production
  deployment method.
- Never commit environment files, provider credentials, raw probe bodies,
  database dumps, signing keys, or private operator addresses.
- Date deployment observations and label external/provider conditions as
  transient.
- Update documentation in the same change as a public API, operator workflow,
  deployment invariant, or compatibility-boundary change.
