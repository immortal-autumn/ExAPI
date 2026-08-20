# ExAPI Deployment Files

This directory contains files for deploying ExAPI on Linux servers.

Production promotion and rollback gates are documented in
[`PRODUCTION_ROLLOUT.md`](PRODUCTION_ROLLOUT.md). The runbook uses an isolated,
digest-pinned Compose project for canary validation before production changes.
The currently reviewed release, image digest, deployed state, and dated
provider conditions are recorded once in
[`../docs/PROJECT_STATUS.md`](../docs/PROJECT_STATUS.md); do not copy those
mutable facts into generic procedures.

## Deployment Methods

| Method | Best For | Setup Wizard |
|--------|----------|--------------|
| **Docker Compose** | Quick setup, all-in-one | Not needed (auto-setup) |
| **Apple container** | Native local stack on macOS 26 | Not needed (auto-setup) |
| **Binary Install** | Production servers, systemd | Web-based wizard |

## Files

| File | Description |
|------|-------------|
| `docker-compose.yml` | Docker Compose configuration (named volumes) |
| `docker-compose.local.yml` | Docker Compose configuration (local directories, easy migration) |
| `docker-deploy.sh` | **One-click Docker deployment script (recommended)** |
| `apple-container.sh` | Native Apple `container` lifecycle script |
| `APPLE_CONTAINER.md` | Apple `container` deployment and operations guide |
| `.env.example` | Container environment variables template |
| `DOCKER.md` | GHCR image verification and digest-pinned Compose guidance |
| `install.sh` | One-click binary installation script |
| `install-datamanagementd.sh` | datamanagementd 一键安装脚本 |
| `sub2api.service` | Systemd service unit file |
| `sub2api-datamanagementd.service` | datamanagementd systemd service unit file |
| `DATAMANAGEMENTD_CN.md` | datamanagementd 部署与联动说明（中文） |
| `config.example.yaml` | Example configuration file |
| `EDGE_SECURITY.md` | Reverse proxy, CDN/WAF, trusted proxy, and ingress hardening guide |
| `PRODUCTION_ROLLOUT.md` | Backup, restore, canary, promotion, and rollback gates |
| `docker-compose.canary-restored.yml` | Egress-denied overlay for restored production-data canaries |
| `ops/` | Executable recovery, independent restore, observation, and signed-manifest gates |

---

## Apple container Deployment

Apple-silicon Macs running macOS 26 can run the complete ExAPI, PostgreSQL, and Redis stack with Apple `container` 1.1.0 or newer:

```bash
./apple-container.sh init
./apple-container.sh up
./apple-container.sh status
./apple-container.sh logs app -f
```

The script uses Apple named volumes, starts dependencies in order, and performs live readiness checks. It does not provide a continuous restart supervisor; run `./apple-container.sh up` after a host reboot. Docker Compose remains the recommended production deployment path.

See [APPLE_CONTAINER.md](./APPLE_CONTAINER.md) for configuration, upgrades, persistence, networking behavior, and limitations.

---

## Docker Deployment (Recommended)

### Method 1: One-Click Deployment (Recommended)

Use the automated preparation script for the easiest setup:

```bash
# Use the exact reviewed tag from docs/PROJECT_STATUS.md.
EXAPI_RELEASE_TAG=vX.Y.Z
GITHUB_RAW_URL="https://raw.githubusercontent.com/immortal-autumn/ExAPI/${EXAPI_RELEASE_TAG}/deploy"
curl -fsSL "${GITHUB_RAW_URL}/docker-deploy.sh" -o docker-deploy.sh
chmod +x docker-deploy.sh
GITHUB_RAW_URL="$GITHUB_RAW_URL" ./docker-deploy.sh
```

Do not pipe an unpinned `main` script directly into a shell for a reviewed
deployment. Inspect the downloaded script and resolve the corresponding image
digest from the signed release before starting services.

**What the script does:**
- Downloads `docker-compose.local.yml` and `.env.example`
- Automatically generates secure secrets (JWT, TOTP, external data encryption, PostgreSQL)
- Creates `.env` file with generated secrets
- Creates necessary data directories (data/, postgres_data/, redis_data/)
- Saves generated credentials to a mode-0600 `.env` without printing them

**After running the script:**
```bash
# Start services
docker compose -f docker-compose.local.yml up -d

# View logs
docker compose -f docker-compose.local.yml logs -f sub2api

# If admin password was auto-generated, find it in logs:
docker compose -f docker-compose.local.yml logs sub2api | grep "admin password"

# Access Web UI
# http://localhost:8080
```

### Method 2: Manual Deployment

If you prefer manual control:

```bash
# Clone repository
EXAPI_RELEASE_TAG=vX.Y.Z
git clone --branch "$EXAPI_RELEASE_TAG" --depth 1 https://github.com/immortal-autumn/ExAPI.git
cd sub2api/deploy

# Configure environment
cp .env.example .env
chmod 600 .env
nano .env  # Set POSTGRES_PASSWORD and other required variables

# Generate secure secrets (recommended)
JWT_SECRET=$(openssl rand -hex 32)
TOTP_ENCRYPTION_KEY=$(openssl rand -hex 32)
DATA_ENCRYPTION_KEY=$(openssl rand -base64 32 | tr -d '\r\n')
echo "JWT_SECRET=${JWT_SECRET}" >> .env
echo "TOTP_ENCRYPTION_KEY=${TOTP_ENCRYPTION_KEY}" >> .env
echo 'SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data-v1' >> .env
printf 'SUB2API_DATA_ENCRYPTION_KEYS_JSON={"data-v1":"%s"}\n' "$DATA_ENCRYPTION_KEY" >> .env

# Create data directories
mkdir -p data postgres_data redis_data

# Start all services using local directory version
docker compose -f docker-compose.local.yml up -d

# View logs (check for auto-generated admin password)
docker compose -f docker-compose.local.yml logs -f sub2api

# Access Web UI
# http://localhost:8080
```

### Deployment Version Comparison

| Version | Data Storage | Migration | Best For |
|---------|-------------|-----------|----------|
| **docker-compose.local.yml** | Local directories (./data, ./postgres_data, ./redis_data) | ✅ Easy (tar entire directory) | Production, need frequent backups/migration |
| **docker-compose.yml** | Named volumes (/var/lib/docker/volumes/) | ⚠️ Requires docker commands | Simple setup, don't need migration |

**Recommendation:** Use `docker-compose.local.yml` (deployed by `docker-deploy.sh`) for easier data management and migration.

### How Auto-Setup Works

When using Docker Compose with `AUTO_SETUP=true`:

1. On first run, the system automatically:
   - Connects to PostgreSQL and Redis
   - Applies database migrations (SQL files in `backend/migrations/*.sql`) and records them in `schema_migrations`
   - Generates JWT secret (if not provided)
   - Creates admin account (password auto-generated if not provided)
   - Writes config.yaml

2. No manual Setup Wizard needed - just configure `.env` and start

3. If `ADMIN_PASSWORD` is not set, check logs for the generated password:
   ```bash
   docker compose logs sub2api | grep "admin password"
   ```

### Private control-plane access

Private deployments use separate public and control listeners. The public
listener serves gateway traffic and `/ready`; the control listener serves the
operator UI and control APIs only to exact hosts and WireGuard peers configured
with `EXAPI_CONTROL_HOSTS` and `EXAPI_OPERATOR_PEER_IPS`.

Control API requests require `X-ExAPI-Control-Request: 1`. Unsafe browser/API
mutations also require an `Origin` whose authority matches the control Host.
Unknown hosts or peers receive 404 deliberately. In particular, a request made
from the server itself is not proof that an external operator peer is allowed;
run the final UI/API checks from an allowlisted WireGuard peer.

Never publish the control port through the public reverse proxy. See
[`EDGE_SECURITY.md`](EDGE_SECURITY.md) for the full boundary.

### Database Migration Notes (PostgreSQL)

- Migrations are applied in lexicographic order (e.g. `001_...sql`, `002_...sql`).
- `schema_migrations` tracks applied migrations (filename + checksum).
- Migrations are forward-only; rollback requires a DB backup restore or a manual compensating SQL script.

**Verify `users.allowed_groups` → `user_allowed_groups` backfill**

During the incremental GORM→Ent migration, `users.allowed_groups` (legacy `BIGINT[]`) is being replaced by a normalized join table `user_allowed_groups(user_id, group_id)`.

Run this query to compare the legacy data vs the join table:

```sql
WITH old_pairs AS (
  SELECT DISTINCT u.id AS user_id, x.group_id
  FROM users u
  CROSS JOIN LATERAL unnest(u.allowed_groups) AS x(group_id)
  WHERE u.allowed_groups IS NOT NULL
)
SELECT
  (SELECT COUNT(*) FROM old_pairs)           AS old_pair_count,
  (SELECT COUNT(*) FROM user_allowed_groups) AS new_pair_count;
```

### datamanagementd（数据管理）联动

如需启用管理后台“数据管理”功能，请额外部署宿主机 `datamanagementd`：

- 主进程固定探测 `/tmp/sub2api-datamanagement.sock`
- Docker 场景下需把宿主机 Socket 挂载到容器内同路径
- 详细步骤见：`deploy/DATAMANAGEMENTD_CN.md`

### Commands

For **local directory version** (docker-compose.local.yml):

```bash
# Start services
docker compose -f docker-compose.local.yml up -d

# Stop services
docker compose -f docker-compose.local.yml down

# View logs
docker compose -f docker-compose.local.yml logs -f sub2api

# Restart ExAPI only
docker compose -f docker-compose.local.yml restart sub2api

# Update to a reviewed immutable digest already written to .env
docker compose --env-file .env -f docker-compose.local.yml config --images
docker compose --env-file .env -f docker-compose.local.yml pull
docker compose --env-file .env -f docker-compose.local.yml up -d

# Remove all data (caution!)
docker compose -f docker-compose.local.yml down
rm -rf data/ postgres_data/ redis_data/
```

For **named volumes version** (docker-compose.yml):

```bash
# Start services
docker compose up -d

# Stop services
docker compose down

# View logs
docker compose logs -f sub2api

# Restart ExAPI only
docker compose restart sub2api

# Update to a reviewed immutable digest already written to .env
docker compose --env-file .env config --images
docker compose --env-file .env pull
docker compose --env-file .env up -d

# Remove all data (caution!)
docker compose down -v
```

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `EXAPI_IMAGE` | **Production: yes** | - | Immutable ExAPI GHCR digest reference |
| `POSTGRES_IMAGE` | **Production: yes** | - | Reviewed immutable PostgreSQL digest reference |
| `REDIS_IMAGE` | **Production: yes** | - | Reviewed immutable Redis digest reference |
| `POSTGRES_PASSWORD` | **Yes** | - | PostgreSQL password |
| `SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID` | **Yes** | - | Active external data-encryption key ID |
| `SUB2API_DATA_ENCRYPTION_KEYS_JSON` | **Yes** | - | JSON keyring of base64-encoded 32-byte data keys; keep outside PostgreSQL/backups |
| `SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE` | No | `false` | Temporarily allow recognized pre-encryption `.sql.gz` records to restore; disable again after controlled recovery |
| `JWT_SECRET` | **Recommended** | *(auto-generated)* | JWT secret (fixed for persistent sessions) |
| `TOTP_ENCRYPTION_KEY` | **Recommended** | *(auto-generated)* | TOTP encryption key (fixed for persistent 2FA) |
| `SERVER_PORT` | No | `8080` | Server port |
| `ADMIN_EMAIL` | No | `admin@sub2api.local` | Admin email |
| `ADMIN_PASSWORD` | No | *(auto-generated)* | Admin password |
| `TZ` | No | `Asia/Shanghai` | Timezone |
| `EXAPI_PUBLIC_LISTEN_ADDR` | Private mode: yes | `0.0.0.0:8080` | Container/public gateway listener |
| `EXAPI_CONTROL_LISTEN_ADDR` | Private mode: yes | `0.0.0.0:8027` | Container/private operator listener |
| `EXAPI_CONTROL_BIND_HOST` | Private mode: yes | `127.0.0.1` | Host address used to publish the control port; use the host WireGuard address for remote operators |
| `EXAPI_CONTROL_HOSTS` | Private mode: yes | loopback hosts | Exact allowed Host authorities for the control listener |
| `EXAPI_OPERATOR_PEER_IPS` | Private mode: yes | loopback peers | Exact direct WireGuard/operator peer IPs |
| `EXAPI_ALLOW_CONTAINER_WILDCARD_CONTROL_BIND` | Bridge Compose: yes | `false` | Explicit acknowledgement that wildcard binding is confined to the container namespace |
| `UPDATE_GITHUB_TOKEN` | No | *(empty)* | Token for `api.github.com` release checks only; asset downloads remain anonymous. |
| `GEMINI_OAUTH_CLIENT_ID` | No | *(builtin)* | Google OAuth client ID (Gemini OAuth). Leave empty to use the built-in Gemini CLI client. |
| `GEMINI_OAUTH_CLIENT_SECRET` | No | *(builtin)* | Google OAuth client secret (Gemini OAuth). Leave empty to use the built-in Gemini CLI client. |
| `GEMINI_OAUTH_SCOPES` | No | *(default)* | OAuth scopes (Gemini OAuth) |
| `GEMINI_QUOTA_POLICY` | No | *(empty)* | JSON overrides for Gemini local quota simulation (Code Assist only). |

See `.env.example` for all available options.

Production `EXAPI_IMAGE`, `POSTGRES_IMAGE`, and `REDIS_IMAGE` values must be
digest references. Verify release labels and attestations as described in
[`DOCKER.md`](DOCKER.md); do not replace them with `latest` or another mutable
tag.

> **Note:** `docker-deploy.sh` also generates the required external data-encryption keyring. Preserve `.env` securely: losing every retained data key makes protected secrets unrecoverable. During rotation, add the new key alongside old keys, switch the active ID, and remove old keys only after rewrap verification.

Backups created by current releases carry an explicit authenticated-encryption format marker. Older records without that marker are restored only when both their file/object names end in `.sql.gz` and `SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE=true`. This compatibility path fully validates and stages the legacy gzip before invoking PostgreSQL; it is never used as fallback after an encrypted backup fails authentication.

### Easy Migration (Local Directory Version)

When using `docker-compose.local.yml`, all data is stored in local directories, making migration simple:

```bash
# On source server: stop services and archive data without external roots
cd /path/to/deployment
docker compose -f docker-compose.local.yml down
cd ..
tar --exclude='deployment/.env' -czf sub2api-data.tar.gz deployment/

# Encrypt the key-bearing .env separately (prompts for a strong passphrase)
openssl enc -aes-256-cbc -pbkdf2 -salt \
  -in deployment/.env -out sub2api-keyring.env.enc

# Transfer the data archive normally. Transfer the encrypted key bundle through
# a separately controlled channel.
scp sub2api-data.tar.gz user@new-server:/path/to/destination/

# On new server: extract data, then restore the key bundle with mode 0600
cd /path/to/destination
tar xzf sub2api-data.tar.gz
umask 077
openssl enc -d -aes-256-cbc -pbkdf2 \
  -in sub2api-keyring.env.enc -out deployment/.env
cd deployment/
docker compose -f docker-compose.local.yml up -d
```

The data archive alone cannot decrypt protected secrets. Treat the separately
encrypted `.env` as disaster-recovery key material, restrict access, and retain
old key IDs through rotation until all protected data has been rewrapped.

---

## Gemini OAuth Configuration

ExAPI supports three methods to connect to Gemini:

### Method 1: Code Assist OAuth (Recommended for GCP Users)

**No configuration needed** - always uses the built-in Gemini CLI OAuth client (public).

1. Leave `GEMINI_OAUTH_CLIENT_ID` and `GEMINI_OAUTH_CLIENT_SECRET` empty
2. In the Admin UI, create a Gemini OAuth account and select **"Code Assist"** type
3. Complete the OAuth flow in your browser

> Note: Even if you configure `GEMINI_OAUTH_CLIENT_ID` / `GEMINI_OAUTH_CLIENT_SECRET` for AI Studio OAuth,
> Code Assist OAuth will still use the built-in Gemini CLI client.

**Requirements:**
- Google account with access to Google Cloud Platform
- A GCP project (auto-detected or manually specified)

**How to get Project ID (if auto-detection fails):**
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Click the project dropdown at the top of the page
3. Copy the Project ID (not the project name) from the list
4. Common formats: `my-project-123456` or `cloud-ai-companion-xxxxx`

### Method 2: AI Studio OAuth (For Regular Google Accounts)

Requires your own OAuth client credentials.

**Step 1: Create OAuth Client in Google Cloud Console**

1. Go to [Google Cloud Console - Credentials](https://console.cloud.google.com/apis/credentials)
2. Create a new project or select an existing one
3. **Enable the Generative Language API:**
   - Go to "APIs & Services" → "Library"
   - Search for "Generative Language API"
   - Click "Enable"
4. **Configure OAuth Consent Screen** (if not done):
   - Go to "APIs & Services" → "OAuth consent screen"
   - Choose "External" user type
   - Fill in app name, user support email, developer contact
   - Add scopes: `https://www.googleapis.com/auth/generative-language.retriever` (and optionally `https://www.googleapis.com/auth/cloud-platform`)
   - Add test users (your Google account email)
5. **Create OAuth 2.0 credentials:**
   - Go to "APIs & Services" → "Credentials"
   - Click "Create Credentials" → "OAuth client ID"
   - Application type: **Web application** (or **Desktop app**)
   - Name: e.g., "ExAPI Gemini"
   - Authorized redirect URIs: Add `http://localhost:1455/auth/callback`
6. Copy the **Client ID** and **Client Secret**
7. **⚠️ Publish to Production (IMPORTANT):**
   - Go to "APIs & Services" → "OAuth consent screen"
   - Click "PUBLISH APP" to move from Testing to Production
   - **Testing mode limitations:**
     - Only manually added test users can authenticate (max 100 users)
     - Refresh tokens expire after 7 days
     - Users must be re-added periodically
   - **Production mode:** Any Google user can authenticate, tokens don't expire
   - Note: For sensitive scopes, Google may require verification (demo video, privacy policy)

**Step 2: Configure Environment Variables**

```bash
GEMINI_OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
GEMINI_OAUTH_CLIENT_SECRET=GOCSPX-your-client-secret

# 可选：如需使用 Gemini CLI 内置 OAuth Client（Code Assist / Google One）
# 安全说明：本仓库不会内置该 client_secret，请在运行环境通过环境变量注入。
# GEMINI_CLI_OAUTH_CLIENT_SECRET=GOCSPX-your-built-in-secret
```

**Step 3: Create Account in Admin UI**

1. Create a Gemini OAuth account and select **"AI Studio"** type
2. Complete the OAuth flow
   - After consent, your browser will be redirected to `http://localhost:1455/auth/callback?code=...&state=...`
   - Copy the full callback URL (recommended) or just the `code` and paste it back into the Admin UI

### Method 3: API Key (Simplest)

1. Go to [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Click "Create API key"
3. In Admin UI, create a Gemini **API Key** account
4. Paste your API key (starts with `AIza...`)

### Comparison Table

| Feature | Code Assist OAuth | AI Studio OAuth | API Key |
|---------|-------------------|-----------------|---------|
| Setup Complexity | Easy (no config) | Medium (OAuth client) | Easy |
| GCP Project Required | Yes | No | No |
| Custom OAuth Client | No (built-in) | Yes (required) | N/A |
| Rate Limits | GCP quota | Standard | Standard |
| Best For | GCP developers | Regular users needing OAuth | Quick testing |

---

## Binary Installation

For production servers using systemd.

### Pinned installation script

```bash
EXAPI_RELEASE_TAG=vX.Y.Z
curl -fsSL "https://raw.githubusercontent.com/immortal-autumn/ExAPI/${EXAPI_RELEASE_TAG}/deploy/install.sh" -o install.sh
sudo bash install.sh
```

The installer generates `/etc/sub2api.env` as `root:root` mode `0600`, loads it
through a systemd drop-in, and preserves it across upgrades and rollbacks. Back
it up separately from PostgreSQL; losing it makes encrypted roots unrecoverable.

### Manual Installation

1. Download the reviewed tag identified in
   [`../docs/PROJECT_STATUS.md`](../docs/PROJECT_STATUS.md) from
   [GitHub Releases](https://github.com/immortal-autumn/ExAPI/releases)
2. Extract and copy the binary to `/opt/sub2api/`
3. Create the mandatory root-only keyring file:
   ```bash
   sudo bash
   umask 077
   key=$(openssl rand -base64 32 | tr -d '\r\n')
   printf "SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=data-v1\nSUB2API_DATA_ENCRYPTION_KEYS_JSON='{\"data-v1\":\"%s\"}'\n" "$key" > /etc/sub2api.env
   unset key
   exit
   ```
4. Copy `sub2api.service` to `/etc/systemd/system/`
5. Run:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable sub2api
   sudo systemctl start sub2api
   ```
6. Open the Setup Wizard in your browser to complete configuration

### Commands

```bash
# Install
sudo ./install.sh

# Upgrade
sudo ./install.sh upgrade

# Uninstall
sudo ./install.sh uninstall
```

### Service Management

```bash
# Start the service
sudo systemctl start sub2api

# Stop the service
sudo systemctl stop sub2api

# Restart the service
sudo systemctl restart sub2api

# Check status
sudo systemctl status sub2api

# View logs
sudo journalctl -u sub2api -f

# Enable auto-start on boot
sudo systemctl enable sub2api
```

### Configuration

#### Server Address and Port

During installation, you will be prompted to configure the server listen address and port. These settings are stored in the systemd service file as environment variables.

To change after installation:

1. Edit the systemd service:
   ```bash
   sudo systemctl edit sub2api
   ```

2. Add or modify:
   ```ini
   [Service]
   Environment=SERVER_HOST=0.0.0.0
   Environment=SERVER_PORT=3000
   ```

3. Reload and restart:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart sub2api
   ```

#### Gemini OAuth Configuration

If you need to use AI Studio OAuth for Gemini accounts, add the OAuth client credentials to the systemd service file:

1. Edit the service file:
   ```bash
   sudo nano /etc/systemd/system/sub2api.service
   ```

2. Add your OAuth credentials in the `[Service]` section (after the existing `Environment=` lines):
   ```ini
   Environment=GEMINI_OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
   Environment=GEMINI_OAUTH_CLIENT_SECRET=GOCSPX-your-client-secret
   ```

   如需使用“内置 Gemini CLI OAuth Client”（Code Assist / Google One），还需要注入：
   ```ini
   Environment=GEMINI_CLI_OAUTH_CLIENT_SECRET=GOCSPX-your-built-in-secret
   ```

3. Reload and restart:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart sub2api
   ```

> **Note:** Code Assist OAuth does not require any configuration - it uses the built-in Gemini CLI client.
> See the [Gemini OAuth Configuration](#gemini-oauth-configuration) section above for detailed setup instructions.

#### Application Configuration

The main config file is at `/etc/sub2api/config.yaml` (created by Setup Wizard).

### Prerequisites

- Linux server (Ubuntu 20.04+, Debian 11+, CentOS 8+, etc.)
- PostgreSQL 14+
- Redis 6+
- systemd

### Directory Structure

```
/opt/sub2api/
├── sub2api              # Main binary
├── sub2api.backup       # Backup (after upgrade)
└── data/                # Runtime data

/etc/sub2api/
└── config.yaml          # Configuration file
```

---

## Troubleshooting

For account-test failures and Antigravity live quota refresh, follow
[`../docs/ACCOUNT_PROBES.md`](../docs/ACCOUNT_PROBES.md). A successful usage
query and a successful inference probe are independent signals.

### Docker

For **local directory version**:

```bash
# Check container status
docker compose -f docker-compose.local.yml ps

# View detailed logs
docker compose -f docker-compose.local.yml logs --tail=100 sub2api

# Check database connection
docker compose -f docker-compose.local.yml exec postgres pg_isready

# Check Redis connection
docker compose -f docker-compose.local.yml exec redis redis-cli ping

# Restart all services
docker compose -f docker-compose.local.yml restart

# Check data directories
ls -la data/ postgres_data/ redis_data/
```

For **named volumes version**:

```bash
# Check container status
docker compose ps

# View detailed logs
docker compose logs --tail=100 sub2api

# Check database connection
docker compose exec postgres pg_isready

# Check Redis connection
docker compose exec redis redis-cli ping

# Restart all services
docker compose restart
```

### Binary Install

```bash
# Check service status
sudo systemctl status sub2api

# View recent logs
sudo journalctl -u sub2api -n 50

# Check config file
sudo cat /etc/sub2api/config.yaml

# Check PostgreSQL
sudo systemctl status postgresql

# Check Redis
sudo systemctl status redis
```

### Common Issues

1. **Port already in use**: Change `SERVER_PORT` in `.env` or systemd config
2. **Database connection failed**: Check PostgreSQL is running and credentials are correct
3. **Redis connection failed**: Check Redis is running and password is correct
4. **Permission denied**: Ensure proper file ownership for binary install

---

## TLS Fingerprint Configuration

ExAPI supports TLS fingerprint simulation to make requests appear as if they come from the official Claude CLI (Node.js client).

> **💡 Tip:** Visit **[tls.sub2api.org](https://tls.sub2api.org/)** to get TLS fingerprint information for different devices and browsers.

### Default Behavior

- Built-in `claude_cli_v2` profile simulates Node.js 20.x + OpenSSL 3.x
- JA3 Hash: `1a28e69016765d92e3b381168d68922c`
- JA4: `t13d5911h1_a33745022dd6_1f22a2ca17c4`
- Profile selection: `accountID % profileCount`

### Configuration

```yaml
gateway:
  tls_fingerprint:
    enabled: true  # Global switch
    profiles:
      # Simple profile (uses default cipher suites)
      profile_1:
        name: "Profile 1"

      # Profile with custom cipher suites (use compact array format)
      profile_2:
        name: "Profile 2"
        cipher_suites: [4866, 4867, 4865, 49199, 49195, 49200, 49196]
        curves: [29, 23, 24]
        point_formats: 0

      # Another custom profile
      profile_3:
        name: "Profile 3"
        cipher_suites: [4865, 4866, 4867, 49199, 49200]
        curves: [29, 23, 24, 25]
```

### Profile Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Display name (required) |
| `cipher_suites` | []uint16 | Cipher suites in decimal. Empty = default |
| `curves` | []uint16 | Elliptic curves in decimal. Empty = default |
| `point_formats` | []uint8 | EC point formats. Empty = default |

### Common Values Reference

**Cipher Suites (TLS 1.3):** `4865` (AES_128_GCM), `4866` (AES_256_GCM), `4867` (CHACHA20)

**Cipher Suites (TLS 1.2):** `49195`, `49196`, `49199`, `49200` (ECDHE variants)

**Curves:** `29` (X25519), `23` (P-256), `24` (P-384), `25` (P-521)
