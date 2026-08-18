.PHONY: build build-backend build-frontend test test-backend test-frontend test-frontend-critical

FRONTEND_CRITICAL_VITEST := \
	src/api/__tests__/client.spec.ts \
	src/api/__tests__/tokenRefresh.spec.ts \
	src/api/__tests__/adminUIRequest.spec.ts \
	src/router/__tests__/singleUserGatewayRoutes.spec.ts \
	src/views/admin/__tests__/AccountsView.selectAllResults.spec.ts \
	src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts \
	src/views/admin/__tests__/AccountsView.usageWindowsHint.spec.ts \
	src/views/admin/settings/__tests__/PrivateSettingsView.spec.ts \
	src/views/admin/settings/__tests__/SettingsView.operator.spec.ts

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	cd frontend && pnpm run lint:check
	cd frontend && pnpm run typecheck
	cd frontend && pnpm run check:coverage-config
	cd frontend && NODE_ENV=test CI=true pnpm run test:coverage
	cd frontend && pnpm run build
	cd frontend && pnpm run check:bundle
	cd frontend && pnpm run check:private-bundle

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)
