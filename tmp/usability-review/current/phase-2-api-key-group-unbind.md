# Admin API key group unbind / 管理员 API Key 分组解绑

Date: 2026-09-01

## Scope / 范围

The private `/admin/api-keys` route now wraps the shared `KeysView` in
`AdminAPIKeysView` and passes `operatorMode`. Group changes in that mode use
the existing admin endpoint `PUT /api/v1/admin/api-keys/:id`; an explicit
unbind sends `{"group_id": 0}`. The legacy shared view keeps using
`PUT /api/v1/keys/:id` when `operatorMode` is false.

私有 `/admin/api-keys` 路由现在通过 `AdminAPIKeysView` 包装共享的
`KeysView` 并传入 `operatorMode`。管理员模式的分组变更使用已有的管理员
接口 `PUT /api/v1/admin/api-keys/:id`；显式解绑发送 `{"group_id": 0}`。
`operatorMode=false` 时共享视图仍使用旧的 `PUT /api/v1/keys/:id`，不改变普通
用户路由语义。

## Evidence / 证据

- `frontend/src/api/__tests__/admin.apiKeys.spec.ts`: verifies `null` maps to
  JSON `group_id: 0`, while positive IDs remain unchanged.
- `frontend/src/views/user/__tests__/KeysView.spec.ts`: opens the group menu
  in operator mode, chooses “No group”, verifies the admin API is called with
  `(keyID, null)`, and verifies the legacy update function is not called.
- Focused command:
  `./node_modules/.bin/vitest run src/api/__tests__/admin.apiKeys.spec.ts src/views/user/__tests__/KeysView.spec.ts`
  Result: 2 files, 15 tests passed.
- Type check:
  `./node_modules/.bin/vue-tsc --noEmit` passed.

## Risk / 风险

The admin endpoint may auto-grant group access to the singleton operator when
binding an exclusive group; that is existing backend contract behavior and is
not changed here. Unbinding is represented by the admin sentinel `0`, not a
JSON `null`, because the legacy request DTO cannot distinguish null from an
omitted field. The shared view still exposes the old path only outside the
private wrapper.

管理员接口在绑定专属分组时可能按既有契约自动授予单例管理员分组权限；本次
没有改变该行为。解绑使用管理员契约的 `0` 哨兵，而不是 JSON `null`，因为旧
请求 DTO 无法区分 `null` 与省略字段。共享视图在私有包装器之外仍保留旧路径。
