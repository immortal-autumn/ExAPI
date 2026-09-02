#!/bin/bash
# =============================================================================
# ExAPI Docker Deployment Preparation Script
# =============================================================================
# This script prepares deployment files for ExAPI:
#   - Downloads docker-compose.local.yml and .env.example
#   - Generates secure secrets (JWT, TOTP, Redis/PostgreSQL, three independent keyrings)
#   - Creates necessary data directories
#
# After running this script, you can start services with:
#   docker compose -f docker-compose.local.yml up -d
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# GitHub raw content base URL
GITHUB_RAW_URL=${GITHUB_RAW_URL:-"https://raw.githubusercontent.com/immortal-autumn/ExAPI/main/deploy"}

# Print colored message
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Generate random secret
generate_secret() {
    openssl rand -hex 32
}

generate_base64_key() {
    openssl rand -base64 32 | tr -d '\r\n'
}

# Production Compose files intentionally reject mutable image tags. Require the
# operator to provide all three reviewed digest references up front instead of
# silently writing REPLACE_WITH_* placeholders that fail only at first startup.
validate_immutable_image_reference() {
    local name="$1" value="$2"
    if [[ ! "$value" =~ ^[^[:space:]@]+@sha256:[0-9a-f]{64}$ ]]; then
        print_error "$name must be an immutable image reference ending in @sha256:<64 lowercase hex digits>"
        return 1
    fi
}

validate_runtime_images() {
    validate_immutable_image_reference EXAPI_IMAGE "$EXAPI_IMAGE" || return 1
    validate_immutable_image_reference POSTGRES_IMAGE "$POSTGRES_IMAGE" || return 1
    validate_immutable_image_reference REDIS_IMAGE "$REDIS_IMAGE" || return 1
    case "$POSTGRES_IMAGE" in
        postgres@sha256:*) ;;
        *) print_error "POSTGRES_IMAGE must use the postgres@sha256:<digest> image"; return 1 ;;
    esac
    case "$REDIS_IMAGE" in
        redis@sha256:*) ;;
        *) print_error "REDIS_IMAGE must use the redis@sha256:<digest> image"; return 1 ;;
    esac
}

# Read an existing .env value without evaluating shell syntax.
read_env_value() {
    local name="$1" file="${2:-.env}"
    [ -f "$file" ] || return 1
    python3 - "$file" "$name" <<'PY'
import sys
path, name = sys.argv[1:]
for raw in open(path, encoding="utf-8"):
    if raw.startswith(name + "="):
        value = raw.split("=", 1)[1].strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "'\"":
            value = value[1:-1]
        print(value, end="")
        raise SystemExit(0)
raise SystemExit(1)
PY
}

validate_keyring_pair() {
    local active="$1" keys="$2" label="$3"
    KEYRING_ACTIVE="$active" KEYRING_JSON="$keys" KEYRING_LABEL="$label" python3 <<'PY'
import base64, json, os
active = os.environ["KEYRING_ACTIVE"]
raw = os.environ["KEYRING_JSON"]
label = os.environ["KEYRING_LABEL"]
if not active or not raw:
    raise SystemExit(f"{label} keyring is empty")
try:
    keys = json.loads(raw)
except Exception as exc:
    raise SystemExit(f"{label} keyring is malformed: {exc}")
if not isinstance(keys, dict) or not keys:
    raise SystemExit(f"{label} keyring must contain at least one key")
if active not in keys:
    raise SystemExit(f"{label} active key is missing from the keyring")
seen = set()
for key_id, encoded in keys.items():
    try:
        material = base64.b64decode(encoded + "=" * (-len(encoded) % 4), validate=True)
    except Exception as exc:
        raise SystemExit(f"{label} key {key_id!r} is malformed: {exc}")
    if len(material) != 32:
        raise SystemExit(f"{label} key {key_id!r} must decode to exactly 32 bytes")
    if material in seen:
        raise SystemExit(f"{label} keyring reuses key material")
    seen.add(material)
PY
}

validate_independent_keyrings() {
    if [ "$#" -ne 9 ]; then
        print_error "validate_independent_keyrings expects three keyring triples"
        return 1
    fi
    KEYRING_DATA_ACTIVE="$1" KEYRING_DATA_JSON="$2" \
    KEYRING_DIGEST_ACTIVE="$4" KEYRING_DIGEST_JSON="$5" \
    KEYRING_BACKUP_ACTIVE="$7" KEYRING_BACKUP_JSON="$8" python3 <<'PY'
import base64, json, os
seen = {}
domains = (
    (os.environ["KEYRING_DATA_ACTIVE"], os.environ["KEYRING_DATA_JSON"], "data-encryption"),
    (os.environ["KEYRING_DIGEST_ACTIVE"], os.environ["KEYRING_DIGEST_JSON"], "gateway-digest"),
    (os.environ["KEYRING_BACKUP_ACTIVE"], os.environ["KEYRING_BACKUP_JSON"], "backup-encryption"),
)
for active, raw, label in domains:
    keys = json.loads(raw)
    if active not in keys:
        raise SystemExit(f"{label} active key is missing from the keyring")
    for key_id, encoded in keys.items():
        material = base64.b64decode(encoded + "=" * (-len(encoded) % 4), validate=True)
        previous = seen.get(material)
        if previous is not None:
            raise SystemExit(f"key material is reused between {previous} and {label}")
        seen[material] = f"{label}/{key_id}"
PY
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Main installation function
main() {
    echo ""
    echo "=========================================="
    echo "  ExAPI Deployment Preparation"
    echo "=========================================="
    echo ""

    # Check if openssl is available
    if ! command_exists openssl; then
        print_error "openssl is not installed. Please install openssl first."
        exit 1
    fi

    local compose_file="docker-compose.local.yml"

    # Check if deployment already exists. The legacy docker-compose.yml name is
    # also detected so rerunning after an older script cannot overwrite state
    # without an explicit confirmation.
    if { [ -f "$compose_file" ] || [ -f "docker-compose.yml" ]; } && [ -f ".env" ]; then
        print_warning "Deployment files already exist in current directory."
        read -p "Overwrite existing files? (y/N): " -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Cancelled."
            exit 0
        fi
    fi

    # Download docker-compose.local.yml without renaming it. Documentation and
    # operator commands use this local-directory variant consistently.
    print_info "Downloading ${compose_file}..."
    if command_exists curl; then
        curl -sSL "${GITHUB_RAW_URL}/docker-compose.local.yml" -o "$compose_file"
    elif command_exists wget; then
        wget -q "${GITHUB_RAW_URL}/docker-compose.local.yml" -O "$compose_file"
    else
        print_error "Neither curl nor wget is installed. Please install one of them."
        exit 1
    fi
    print_success "Downloaded ${compose_file}"

    # Download .env.example
    print_info "Downloading .env.example..."
    if command_exists curl; then
        curl -sSL "${GITHUB_RAW_URL}/.env.example" -o .env.example
    else
        wget -q "${GITHUB_RAW_URL}/.env.example" -O .env.example
    fi
    print_success "Downloaded .env.example"

    # Resolve and validate immutable image references before writing .env. On
    # redeploy, values are read from the existing file; on a fresh install the
    # operator must export them explicitly (for example, from the signed
    # release record).
    EXAPI_IMAGE="${EXAPI_IMAGE:-$(read_env_value EXAPI_IMAGE 2>/dev/null || true)}"
    POSTGRES_IMAGE="${POSTGRES_IMAGE:-$(read_env_value POSTGRES_IMAGE 2>/dev/null || true)}"
    REDIS_IMAGE="${REDIS_IMAGE:-$(read_env_value REDIS_IMAGE 2>/dev/null || true)}"
    if ! validate_runtime_images; then
        print_error "Set EXAPI_IMAGE, POSTGRES_IMAGE, and REDIS_IMAGE to reviewed immutable digests before rerunning."
        exit 1
    fi

    # Generate .env file with auto-generated secrets
    print_info "Generating secure secrets..."
    echo ""

    # Generate state-coupled credentials only for a fresh deployment. On
    # redeploy they must remain aligned with the retained database and encrypted
    # TOTP/JWT state.
    if [ -f .env ]; then
        JWT_SECRET=$(read_env_value JWT_SECRET)
        TOTP_ENCRYPTION_KEY=$(read_env_value TOTP_ENCRYPTION_KEY)
        POSTGRES_PASSWORD=$(read_env_value POSTGRES_PASSWORD)
        REDIS_PASSWORD=$(read_env_value REDIS_PASSWORD || true)
        [ -n "$JWT_SECRET" ] || { print_error "existing JWT_SECRET is empty"; exit 1; }
        [ -n "$TOTP_ENCRYPTION_KEY" ] || { print_error "existing TOTP_ENCRYPTION_KEY is empty"; exit 1; }
        [ -n "$POSTGRES_PASSWORD" ] || { print_error "existing POSTGRES_PASSWORD is empty"; exit 1; }
        if [ -z "$REDIS_PASSWORD" ]; then
            REDIS_PASSWORD=$(generate_secret)
            print_info "Generated missing Redis password for existing deployment"
        fi
        DATA_ENCRYPTION_ACTIVE_KEY_ID=$(read_env_value SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID || true)
        DATA_ENCRYPTION_KEYS_JSON=$(read_env_value SUB2API_DATA_ENCRYPTION_KEYS_JSON || true)
        GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=$(read_env_value SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID || true)
        GATEWAY_KEY_DIGEST_KEYS_JSON=$(read_env_value SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON || true)
        BACKUP_ENCRYPTION_ACTIVE_KEY_ID=$(read_env_value SUB2API_BACKUP_ENCRYPTION_ACTIVE_KEY_ID || true)
        BACKUP_ENCRYPTION_KEYS_JSON=$(read_env_value SUB2API_BACKUP_ENCRYPTION_KEYS_JSON || true)
        validate_keyring_pair "$DATA_ENCRYPTION_ACTIVE_KEY_ID" "$DATA_ENCRYPTION_KEYS_JSON" data-encryption

        if [ -z "$GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID" ] && [ -z "$GATEWAY_KEY_DIGEST_KEYS_JSON" ]; then
            GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=digest-v1
            GATEWAY_KEY_DIGEST_KEY=$(generate_base64_key)
            GATEWAY_KEY_DIGEST_KEYS_JSON="{\"digest-v1\":\"${GATEWAY_KEY_DIGEST_KEY}\"}"
            print_info "Generated missing gateway-digest keyring for existing deployment"
        fi
        validate_keyring_pair "$GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID" "$GATEWAY_KEY_DIGEST_KEYS_JSON" gateway-digest

        if [ -z "$BACKUP_ENCRYPTION_ACTIVE_KEY_ID" ] && [ -z "$BACKUP_ENCRYPTION_KEYS_JSON" ]; then
            BACKUP_ENCRYPTION_ACTIVE_KEY_ID=backup-v1
            BACKUP_ENCRYPTION_KEY=$(generate_base64_key)
            BACKUP_ENCRYPTION_KEYS_JSON="{\"backup-v1\":\"${BACKUP_ENCRYPTION_KEY}\"}"
            print_info "Generated missing backup-encryption keyring for existing deployment"
        fi
        validate_keyring_pair "$BACKUP_ENCRYPTION_ACTIVE_KEY_ID" "$BACKUP_ENCRYPTION_KEYS_JSON" backup-encryption
        validate_independent_keyrings \
            "$DATA_ENCRYPTION_ACTIVE_KEY_ID" "$DATA_ENCRYPTION_KEYS_JSON" data-encryption \
            "$GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID" "$GATEWAY_KEY_DIGEST_KEYS_JSON" gateway-digest \
            "$BACKUP_ENCRYPTION_ACTIVE_KEY_ID" "$BACKUP_ENCRYPTION_KEYS_JSON" backup-encryption
        print_info "Preserving existing validated encryption keyrings"
    else
        JWT_SECRET=$(generate_secret)
        TOTP_ENCRYPTION_KEY=$(generate_secret)
        POSTGRES_PASSWORD=$(generate_secret)
        REDIS_PASSWORD=$(generate_secret)
        DATA_ENCRYPTION_ACTIVE_KEY_ID=data-v1
        DATA_ENCRYPTION_KEY=$(generate_base64_key)
        DATA_ENCRYPTION_KEYS_JSON="{\"data-v1\":\"${DATA_ENCRYPTION_KEY}\"}"
        GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=digest-v1
        GATEWAY_KEY_DIGEST_KEY=$(generate_base64_key)
        GATEWAY_KEY_DIGEST_KEYS_JSON="{\"digest-v1\":\"${GATEWAY_KEY_DIGEST_KEY}\"}"
        BACKUP_ENCRYPTION_ACTIVE_KEY_ID=backup-v1
        BACKUP_ENCRYPTION_KEY=$(generate_base64_key)
        BACKUP_ENCRYPTION_KEYS_JSON="{\"backup-v1\":\"${BACKUP_ENCRYPTION_KEY}\"}"
        validate_independent_keyrings \
            "$DATA_ENCRYPTION_ACTIVE_KEY_ID" "$DATA_ENCRYPTION_KEYS_JSON" data-encryption \
            "$GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID" "$GATEWAY_KEY_DIGEST_KEYS_JSON" gateway-digest \
            "$BACKUP_ENCRYPTION_ACTIVE_KEY_ID" "$BACKUP_ENCRYPTION_KEYS_JSON" backup-encryption
    fi

    # Create .env atomically under a restrictive umask. Secret values are
    # substituted by shell builtins so they never appear in child-process argv.
    local old_umask env_tmp
    old_umask=$(umask)
    umask 077
    env_tmp=".env.tmp.$$"
    : > "$env_tmp"
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            JWT_SECRET=*) printf 'JWT_SECRET=%s\n' "$JWT_SECRET" ;;
            TOTP_ENCRYPTION_KEY=*) printf 'TOTP_ENCRYPTION_KEY=%s\n' "$TOTP_ENCRYPTION_KEY" ;;
            POSTGRES_PASSWORD=*) printf 'POSTGRES_PASSWORD=%s\n' "$POSTGRES_PASSWORD" ;;
            REDIS_PASSWORD=*) printf 'REDIS_PASSWORD=%s\n' "$REDIS_PASSWORD" ;;
            EXAPI_IMAGE=*) printf 'EXAPI_IMAGE=%s\n' "$EXAPI_IMAGE" ;;
            POSTGRES_IMAGE=*) printf 'POSTGRES_IMAGE=%s\n' "$POSTGRES_IMAGE" ;;
            REDIS_IMAGE=*) printf 'REDIS_IMAGE=%s\n' "$REDIS_IMAGE" ;;
            APPLE_CONTAINER_SUB2API_IMAGE=*) printf 'APPLE_CONTAINER_SUB2API_IMAGE=%s\n' "$EXAPI_IMAGE" ;;
            APPLE_CONTAINER_POSTGRES_IMAGE=*) printf 'APPLE_CONTAINER_POSTGRES_IMAGE=%s\n' "$POSTGRES_IMAGE" ;;
            APPLE_CONTAINER_REDIS_IMAGE=*) printf 'APPLE_CONTAINER_REDIS_IMAGE=%s\n' "$REDIS_IMAGE" ;;
            SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=*) printf 'SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID=%s\n' "$DATA_ENCRYPTION_ACTIVE_KEY_ID" ;;
            SUB2API_DATA_ENCRYPTION_KEYS_JSON=*) printf 'SUB2API_DATA_ENCRYPTION_KEYS_JSON=%s\n' "$DATA_ENCRYPTION_KEYS_JSON" ;;
            SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=*) printf 'SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=%s\n' "$GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID" ;;
            SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON=*) printf 'SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON=%s\n' "$GATEWAY_KEY_DIGEST_KEYS_JSON" ;;
            SUB2API_BACKUP_ENCRYPTION_ACTIVE_KEY_ID=*) printf 'SUB2API_BACKUP_ENCRYPTION_ACTIVE_KEY_ID=%s\n' "$BACKUP_ENCRYPTION_ACTIVE_KEY_ID" ;;
            SUB2API_BACKUP_ENCRYPTION_KEYS_JSON=*) printf 'SUB2API_BACKUP_ENCRYPTION_KEYS_JSON=%s\n' "$BACKUP_ENCRYPTION_KEYS_JSON" ;;
            *) printf '%s\n' "$line" ;;
        esac
    done < .env.example > "$env_tmp"
    mv -f "$env_tmp" .env
    chmod 600 .env
    umask "$old_umask"
    unset DATA_ENCRYPTION_ACTIVE_KEY_ID DATA_ENCRYPTION_KEY DATA_ENCRYPTION_KEYS_JSON GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID GATEWAY_KEY_DIGEST_KEY GATEWAY_KEY_DIGEST_KEYS_JSON BACKUP_ENCRYPTION_ACTIVE_KEY_ID BACKUP_ENCRYPTION_KEY BACKUP_ENCRYPTION_KEYS_JSON

    # Create data directories
    print_info "Creating data directories..."
    mkdir -p data postgres_data redis_data
    print_success "Created data directories"

    # Set secure permissions for .env file (readable/writable only by owner)
    chmod 600 .env
    echo ""

    # Display completion message
    echo "=========================================="
    echo "  Preparation Complete!"
    echo "=========================================="
    echo ""
    echo "Generated credentials were saved to .env (not displayed)."
    echo ""
    print_warning "These credentials have been saved to .env file."
    print_warning "Please keep them secure and do not share publicly!"
    echo ""
    echo "Directory structure:"
    echo "  ${compose_file} - Docker Compose configuration"
    echo "  .env                      - Environment variables (generated secrets)"
    echo "  .env.example              - Example template (for reference)"
    echo "  data/                     - Application data (will be created on first run)"
    echo "  postgres_data/            - PostgreSQL data"
    echo "  redis_data/               - Redis data"
    echo ""
    echo "Next steps:"
    echo "  1. (Optional) Edit .env to customize configuration"
    echo "  2. Start services:"
    echo "     docker compose -f ${compose_file} up -d"
    echo ""
    echo "  3. View logs:"
    echo "     docker compose -f ${compose_file} logs -f sub2api"
    echo ""
    echo "  4. Access Web UI:"
    echo "     http://localhost:8080"
    echo ""
    print_info "If admin password is not set in .env, it will be auto-generated."
    print_info "Check logs for the generated admin password on first startup."
    echo ""
}

# Run main function when executed, while allowing contract tests to source the
# validation helpers without performing a deployment.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
