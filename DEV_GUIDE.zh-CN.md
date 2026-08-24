# ExAPI 开发指南

[English（默认）](DEV_GUIDE.md) | 简体中文

本文记录 ExAPI 贡献者使用的本地环境约定、常见问题和审查要求。当前路线图与强制阶段
门禁见 [`development.md`](development.md)，当前发布和部署基线见
[`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md)。

## 1. 项目信息

| 项目 | 当前值 |
|---|---|
| 上游仓库 | `Wei-Shaw/sub2api` |
| ExAPI 仓库 | `immortal-autumn/ExAPI` |
| 后端 | Go、Ent ORM、Gin |
| 前端 | Vue 3、TypeScript、pnpm |
| 数据服务 | PostgreSQL 16、Redis |
| 包管理器 | Go modules 和 **pnpm**（不是 npm） |

内部 module、服务、环境变量和数据路径可能有意保留 `sub2api` 标识。重命名之前必须阅读
[`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md)。

## 2. 本地环境

### Windows 上的 PostgreSQL 16

| 配置 | 值 |
|---|---|
| 端口 | `5432` |
| `psql` | `C:\Program Files\PostgreSQL\16\bin\psql.exe` |
| `pg_hba.conf` | `C:\Program Files\PostgreSQL\16\data\pg_hba.conf` |

只使用本地开发凭据，不要把生产凭据或受保护的环境文件复制到 checkout 中。

### Redis

本地默认端口为 `6379`。如果服务可能被开发机 loopback 之外的网络访问，必须配置认证。

### 工具

```bash
# golangci-lint v2.9
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9

# 前端包管理器
npm install -g pnpm
```

使用工作流和仓库元数据锁定的 Go、Node 和 pnpm 版本。本文更新时，CI 要求 Go 1.26.6，
前端使用 `pnpm install --frozen-lockfile`。

## 3. CI 和本地检查

| Workflow | 触发条件 | 检查范围 |
|---|---|---|
| `backend-ci.yml` | push、pull request | module tidy、单元/集成测试、race detector、前端、shell 和契约检查 |
| `security-scan.yml` | push、pull request、定时 | 漏洞和安全扫描 |
| `release.yml` | `v*` tag | 发布测试、不可变镜像构建、SBOM 和 provenance |

常用本地命令：

```bash
# 后端
cd backend
GOTOOLCHAIN=auto go test -tags=unit ./...
GOTOOLCHAIN=auto go test -tags=integration ./...
golangci-lint run ./...

# 前端
cd ../frontend
pnpm install --frozen-lockfile
pnpm lint:check
pnpm typecheck
pnpm test:coverage
pnpm build

# 仓库契约
cd ..
python3 tools/check_release_contract.py
bash deploy/test-production-rollout-contract.sh
git diff --check
```

先运行与变更直接相关的较小测试，再执行 [`development.md`](development.md) 要求的
完整门禁。

## 4. 常见问题

### 保持 `pnpm-lock.yaml` 同步

修改 `package.json` 后必须重新生成并提交 `pnpm-lock.yaml`；CI 使用
`--frozen-lockfile`，会拒绝不一致的锁文件。

```bash
cd frontend
pnpm install
git add -- pnpm-lock.yaml
```

### 不要混用 npm 和 pnpm 安装

npm 创建的 `node_modules` 可能与 pnpm 冲突并产生权限或链接错误。删除该生成目录后，
使用 pnpm 重新安装。

```bash
cd frontend
rm -rf node_modules
pnpm install
```

### 避免 PowerShell 展开 bcrypt hash

PowerShell 会解释双引号中的 `$`。不要内联执行包含 bcrypt hash 的 SQL。把 SQL 写入不会
插值的受保护文件并通过 `psql -f` 执行；不得把真实密码或 hash 写入 Git。

### Windows 数据库工具优先使用纯 ASCII 临时路径

部分 `psql`/shell 组合无法处理非 ASCII 路径。执行前可把一次性 SQL 文件复制到
`C:\temp.sql` 等路径，使用后删除。

### PostgreSQL 密码恢复

临时把 localhost 认证改为 `trust` 是高风险恢复操作。必须限制在 loopback，重启
PostgreSQL，重置所需密码，恢复 `scram-sha-256` 后再次重启。不得让 `trust` 持续启用，
也不得记录真实凭据。

### 修改 Go interface 后更新所有测试替身

interface 新增方法后，每个实现它的 stub 和 mock 都必须增加该方法。使用 `rg` 搜索并
更新所有实现，然后运行依赖该 interface 的 package 测试。

```bash
cd backend
rg -n 'type .*?(Stub|Mock).* struct' internal
```

### Windows localhost 解析不明确时使用 `127.0.0.1`

`psql` 可能先尝试 IPv6 `::1`。本地服务只绑定 IPv4 时，显式使用 `127.0.0.1`。

### 没有 `make` 时直接运行底层 Go 命令

未安装 Make 的 Windows 环境可以直接运行 Makefile target 对应的命令：

```bash
go test -tags=unit ./...
go test -tags=integration ./...
```

### 修改 Ent schema 后重新生成代码

```bash
cd backend
go generate ./ent
git status --short ent
```

审查生成文件，并只显式暂存属于本次 schema 变更的路径，不要暂存工作区中的无关变更。

### 把模型映射问题和 UI 账号测试分开诊断

批量编辑破坏某个平台的模型白名单或映射后，账号在 UI 测试中可能看似健康，但 API
路由仍返回 `Service temporarily unavailable`。同时选择不同服务商账号执行批量操作时，
尤其容易出现这个问题。

- 修改前检查受影响账号的当前模型映射。
- 只向匹配的平台应用经过验证的透传或服务商映射。
- 批量选择尽量限制在同一服务商。
- 如果映射已无法安全审计，按批准的运维流程保存受保护证据并重建受影响账号。
- 服务商凭据和导出的账号 payload 必须保存在 Git 之外。

### Pull request 检查表

- [ ] 目标后端测试通过。
- [ ] 适用时，所需集成测试和 race 检查通过。
- [ ] 修改前端时，lint、typecheck、测试和 build 通过。
- [ ] `pnpm-lock.yaml` 与 `package.json` 一致。
- [ ] interface stub/mock 和 Ent 生成代码完整。
- [ ] release、品牌、部署和文档契约通过。
- [ ] diff 中没有凭据、受保护环境文件、原始服务商响应或生产地址。

## 5. 命令速查

### 数据库

```bash
psql -U sub2api -h 127.0.0.1 -d sub2api
psql -U postgres -h 127.0.0.1 -c "\du"
psql -U postgres -h 127.0.0.1 -c "\l"
psql -U sub2api -h 127.0.0.1 -d sub2api -f migration.sql
```

### Git

checkout 使用不同 remote 分别表示上游和 GitHub fork。fetch 或 push 前先检查：

```bash
git remote -v
git fetch upstream
git fetch github-fork
git status -sb
```

每个 commit 只包含一个明确变更，并且只暂存已审查的路径。不得重写共享 release 历史，
也不得重新标记已发布的不可变 release。

### 前端

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm dev
pnpm build
```

### 后端

```bash
cd backend
GOTOOLCHAIN=auto go run ./cmd/server/
go generate ./ent
GOTOOLCHAIN=auto go test -tags=unit ./...
GOTOOLCHAIN=auto go test -tags=integration ./...
golangci-lint run ./...
```

## 6. 仓库结构

```text
ExAPI/
├── backend/
│   ├── cmd/server/          # 主服务入口
│   ├── ent/                 # Ent 生成代码和 schema
│   ├── internal/            # Handler、service、repository、server 配置
│   └── migrations/          # 前向数据库迁移
├── frontend/
│   ├── src/                 # API、组件、视图、store、i18n、类型
│   ├── package.json
│   └── pnpm-lock.yaml
├── deploy/                  # 安装器、Compose 文件和发布 runbook
├── docs/                    # 持续维护的契约和项目状态
├── openspec/changes/        # 历史变更证据
└── tmp/                     # checkout 本地临时数据（已忽略）
```

## 7. 参考资料

- [ExAPI 仓库](https://github.com/immortal-autumn/ExAPI)
- [Sub2API 上游](https://github.com/Wei-Shaw/sub2api)
- [Ent 文档](https://entgo.io/docs/getting-started)
- [Vue 文档](https://vuejs.org/)
- [pnpm 文档](https://pnpm.io/)
