# ExAPI development guide

English (default) | [简体中文](DEV_GUIDE.zh-CN.md)

This guide records local environment conventions, recurring pitfalls, and
review checks for ExAPI contributors. The active roadmap and mandatory phase
gates live in [`development.md`](development.md); the current release and
deployment baseline lives in [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md).

## 1. Project information

| Item | Value |
|---|---|
| Upstream repository | `Wei-Shaw/sub2api` |
| ExAPI repository | `immortal-autumn/ExAPI` |
| Backend | Go, Ent ORM, Gin |
| Frontend | Vue 3, TypeScript, pnpm |
| Data services | PostgreSQL 16, Redis |
| Package managers | Go modules and **pnpm** (not npm) |

Internal module names, services, environment variables, and data paths may
intentionally retain the `sub2api` identifier. Read
[`docs/UPSTREAM_COMPATIBILITY.md`](docs/UPSTREAM_COMPATIBILITY.md) before
renaming any of them.

## 2. Local environment

### PostgreSQL 16 on Windows

| Setting | Value |
|---|---|
| Port | `5432` |
| `psql` | `C:\Program Files\PostgreSQL\16\bin\psql.exe` |
| `pg_hba.conf` | `C:\Program Files\PostgreSQL\16\data\pg_hba.conf` |

Use local-only development credentials. Do not copy production credentials or
protected environment files into the checkout.

### Redis

The local default is port `6379`. Configure authentication whenever the service
is reachable beyond a developer-only loopback network.

### Tools

```bash
# golangci-lint v2.9
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9

# Frontend package manager
npm install -g pnpm
```

Use the Go and Node/pnpm versions pinned by the workflows and repository
metadata. At the time of this guide, CI requires Go 1.26.6 and installs the
frontend with `pnpm install --frozen-lockfile`.

## 3. CI and local checks

| Workflow | Trigger | Coverage |
|---|---|---|
| `backend-ci.yml` | push, pull request | module tidy, unit/integration tests, race detector, frontend, shell, and contract checks |
| `security-scan.yml` | push, pull request, schedule | vulnerability and security scans |
| `release.yml` | `v*` tag | release tests, immutable image build, SBOM, and provenance |

Common local commands:

```bash
# Backend
cd backend
GOTOOLCHAIN=auto go test -tags=unit ./...
GOTOOLCHAIN=auto go test -tags=integration ./...
golangci-lint run ./...

# Frontend
cd ../frontend
pnpm install --frozen-lockfile
pnpm lint:check
pnpm typecheck
pnpm test:coverage
pnpm build

# Repository contracts
cd ..
python3 tools/check_release_contract.py
bash deploy/test-production-rollout-contract.sh
git diff --check
```

Run the narrower target appropriate to your change first, then the complete
required gate from [`development.md`](development.md).

## 4. Recurring pitfalls

### Keep `pnpm-lock.yaml` synchronized

If `package.json` changes, regenerate and commit `pnpm-lock.yaml`; CI uses
`--frozen-lockfile` and rejects drift.

```bash
cd frontend
pnpm install
git add -- pnpm-lock.yaml
```

### Do not mix npm and pnpm installations

An npm-created `node_modules` tree may conflict with pnpm and produce permission
or linking errors. Remove that generated directory and reinstall with pnpm.

```bash
cd frontend
rm -rf node_modules
pnpm install
```

### Protect bcrypt hashes from PowerShell interpolation

PowerShell interprets `$` inside double-quoted strings. Avoid inline SQL that
contains bcrypt hashes. Put the SQL in a protected file without interpolation
and execute it with `psql -f`; never place real passwords or hashes in Git.

### Prefer ASCII-only temporary paths for Windows database tools

Some `psql`/shell combinations fail on non-ASCII paths. Copy a disposable SQL
file to a path such as `C:\temp.sql` before executing it, then remove it.

### PostgreSQL password recovery

Temporarily changing localhost authentication to `trust` is a high-risk
recovery action. Restrict it to loopback, restart PostgreSQL, reset the required
passwords, restore `scram-sha-256`, and restart again. Never leave `trust`
enabled or document live credentials.

### Update every test double after changing a Go interface

Adding an interface method requires each stub and mock implementing the
interface to add that method. Search with `rg`, update all implementations, and
run the packages that consume the interface.

```bash
cd backend
rg -n 'type .*?(Stub|Mock).* struct' internal
```

### Use `127.0.0.1` when Windows localhost resolution is ambiguous

`psql` may try IPv6 `::1` before IPv4. Use `127.0.0.1` when the local service is
bound only to IPv4.

### Run the underlying Go commands when `make` is unavailable

Windows environments without Make can run the commands represented by the
Makefile targets directly:

```bash
go test -tags=unit ./...
go test -tags=integration ./...
```

### Regenerate Ent output after schema changes

```bash
cd backend
go generate ./ent
git status --short ent
```

Review and explicitly stage the generated files belonging to the schema
change. Do not stage unrelated worktree changes.

### Diagnose model mapping separately from UI account tests

An account can appear healthy in a UI test while API routing returns
`Service temporarily unavailable` when bulk edits have damaged a platform's
model allowlist or mapping. This is especially likely when accounts from
different providers are selected in one bulk operation.

- Inspect the affected account's current model mapping before changing it.
- Apply a verified passthrough or provider mapping only to the matching
  platform.
- Prefer provider-scoped bulk selections.
- If mappings are too damaged to audit safely, export protected evidence and
  rebuild the affected accounts through the approved operator workflow.
- Keep provider credentials and exported account payloads outside Git.

### Pull-request checklist

- [ ] Targeted backend tests pass.
- [ ] Required integration and race checks pass when applicable.
- [ ] Frontend lint, typecheck, tests, and build pass when frontend code changes.
- [ ] `pnpm-lock.yaml` matches `package.json`.
- [ ] Interface stubs/mocks and Ent generated code are complete.
- [ ] Release, branding, deployment, and documentation contracts pass.
- [ ] No credential, protected environment file, raw provider response, or
      production address is present in the diff.

## 5. Command reference

### Database

```bash
psql -U sub2api -h 127.0.0.1 -d sub2api
psql -U postgres -h 127.0.0.1 -c "\du"
psql -U postgres -h 127.0.0.1 -c "\l"
psql -U sub2api -h 127.0.0.1 -d sub2api -f migration.sql
```

### Git

The checkout has separate upstream and GitHub fork remotes. Inspect them before
fetching or pushing:

```bash
git remote -v
git fetch upstream
git fetch github-fork
git status -sb
```

Create focused commits and stage only paths reviewed for the change. Do not
rewrite shared release history or retag a published immutable release.

### Frontend

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm dev
pnpm build
```

### Backend

```bash
cd backend
GOTOOLCHAIN=auto go run ./cmd/server/
go generate ./ent
GOTOOLCHAIN=auto go test -tags=unit ./...
GOTOOLCHAIN=auto go test -tags=integration ./...
golangci-lint run ./...
```

## 6. Repository structure

```text
ExAPI/
├── backend/
│   ├── cmd/server/          # Main server entry point
│   ├── ent/                 # Generated Ent code and schemas
│   ├── internal/            # Handlers, services, repositories, server setup
│   └── migrations/          # Forward database migrations
├── frontend/
│   ├── src/                 # API, components, views, stores, i18n, types
│   ├── package.json
│   └── pnpm-lock.yaml
├── deploy/                  # Installers, Compose files, rollout runbooks
├── docs/                    # Living contracts and project status
├── openspec/changes/        # Historical change evidence
└── tmp/                     # Checkout-local scratch data (ignored)
```

## 7. References

- [ExAPI repository](https://github.com/immortal-autumn/ExAPI)
- [Sub2API upstream](https://github.com/Wei-Shaw/sub2api)
- [Ent documentation](https://entgo.io/docs/getting-started)
- [Vue documentation](https://vuejs.org/)
- [pnpm documentation](https://pnpm.io/)
