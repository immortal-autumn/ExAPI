# Redaction fix evidence

Date: 2026-09-01

Scope:
- `frontend/src/components/user/UserErrorDetailModal.vue`
- `frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue`
- `frontend/src/utils/errorBodySummary.ts`

Validation:
- `pnpm exec vitest run src/components/user/__tests__/UserErrorDetailModal.spec.ts --reporter verbose`
- `pnpm exec vitest run src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts --reporter verbose`
- `pnpm exec vitest run src/utils/__tests__/errorBodySummary.spec.ts --reporter verbose`

Result:
- Provider response bodies are no longer rendered raw in the user or ops error detail modals.
- The UI now shows only redacted, bounded metadata from `error_body`.
