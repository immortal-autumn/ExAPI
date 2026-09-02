#!/usr/bin/env python3
"""Fail closed when required ExAPI release gates or artifact provenance regress."""
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]


def require(path: str, needle: str) -> None:
    text = (ROOT / path).read_text(encoding="utf-8")
    if needle not in text:
        raise SystemExit(f"{path}: missing required contract: {needle}")


def forbid(path: str, needle: str) -> None:
    text = (ROOT / path).read_text(encoding="utf-8")
    if needle in text:
        raise SystemExit(f"{path}: forbidden release contract remains: {needle}")


for required in (
    "pnpm run lint:check",
    "pnpm run typecheck",
    "pnpm run check:coverage-config",
    "pnpm run test:coverage",
    "pnpm run build",
    "pnpm run check:bundle",
    "pnpm run check:private-bundle",
):
    require("Makefile", required)

for required in (
    "GOTOOLCHAIN=auto go test -race ./internal/...",
    "GOTOOLCHAIN=auto go mod tidy -diff",
    "python3 tools/check_release_contract.py",
):
    require(".github/workflows/backend-ci.yml", required)

for workflow in (
    ".github/workflows/backend-ci.yml",
    ".github/workflows/security-scan.yml",
    ".github/workflows/release.yml",
):
    require(workflow, "node-version: '24'")
    require(workflow, "version: 9.15.9")

require("frontend/package.json", '"packageManager": "pnpm@9.15.9"')
require("frontend/package.json", '"node": ">=24 <25"')
require("frontend/package.json", '"dompurify": "^3.4.13"')
require("frontend/package.json", '"write-excel-file": "^4.1.1"')
forbid("frontend/package.json", '"xlsx":')
forbid("frontend/pnpm-lock.yaml", "xlsx@")
forbid(".github/audit-exceptions.yml", "package: xlsx")
require("Dockerfile", "corepack prepare pnpm@9.15.9 --activate")
require("deploy/Dockerfile", "corepack prepare pnpm@9.15.9 --activate")

for path in ("Dockerfile", "deploy/Dockerfile", "Dockerfile.goreleaser"):
    require(path, "org.opencontainers.image.title=\"ExAPI\"")
    require(path, "org.opencontainers.image.source=\"https://github.com/immortal-autumn/ExAPI\"")
    require(path, "http://localhost:${SERVER_PORT:-8080}/ready")
    require(path, "/app/migrate-private-only")
    require(path, "/app/verify-private-cutover-report")
    require(path, "/app/with-migration-report-key.sh")
    forbid(path, "http://localhost:${SERVER_PORT:-8080}/health")
    forbid(path, "github.com/Wei-Shaw/sub2api")

docker_build_image_args = {
    "Dockerfile": {"NODE_IMAGE", "GOLANG_IMAGE", "ALPINE_IMAGE", "POSTGRES_IMAGE"},
    "deploy/Dockerfile": {"NODE_IMAGE", "GOLANG_IMAGE", "ALPINE_IMAGE"},
    "Dockerfile.goreleaser": {"ALPINE_IMAGE", "POSTGRES_IMAGE"},
}
for path, expected_args in docker_build_image_args.items():
    seen_args = set()
    for line_number, line in enumerate((ROOT / path).read_text(encoding="utf-8").splitlines(), 1):
        match = re.fullmatch(r"ARG ([A-Z_]+)=(.+)", line)
        if not match or match.group(1) not in expected_args:
            continue
        seen_args.add(match.group(1))
        if not re.search(r"@sha256:[0-9a-f]{64}$", match.group(2)):
            raise SystemExit(f"{path}:{line_number}: {match.group(1)} is not digest-pinned")
    if seen_args != expected_args:
        missing = ", ".join(sorted(expected_args - seen_args))
        raise SystemExit(f"{path}: missing required build image ARG(s): {missing}")

require(".github/workflows/release.yml", "quality-gate")
require(".github/workflows/release.yml", "resolve-ref:")
require(".github/workflows/release.yml", "needs: [resolve-ref, quality-gate]")
require(".github/workflows/release.yml", "ref: ${{ needs.resolve-ref.outputs.sha }}")
require(".github/workflows/release.yml", "python3 tools/check_release_contract.py")
forbid(".github/workflows/release.yml", "sync-version-file:")
forbid(".github/workflows/release.yml", "name: version-file")
require(".github/workflows/release.yml", "test -x /app/migrate-private-only")
require(".github/workflows/release.yml", "test -x /app/verify-private-cutover-report")
require(".github/workflows/release.yml", "test -x /app/with-migration-report-key.sh")
require(".goreleaser.yaml", "id: migrate-private-only")
require("Dockerfile.goreleaser", "COPY migrate-private-only /app/migrate-private-only")
require("Dockerfile.goreleaser", "COPY verify-private-cutover-report /app/verify-private-cutover-report")
require("Dockerfile.goreleaser", "deploy/ops/with-migration-report-key.sh /app/with-migration-report-key.sh")
goreleaser_lines = (ROOT / ".goreleaser.yaml").read_text(encoding="utf-8").splitlines()
archives_start = goreleaser_lines.index("archives:")
archives_end = next(
    index
    for index in range(archives_start + 1, len(goreleaser_lines))
    if goreleaser_lines[index] and not goreleaser_lines[index][0].isspace()
)
archives_contract = "\n".join(goreleaser_lines[archives_start:archives_end])
if "    ids:\n      - sub2api" not in archives_contract:
    raise SystemExit(".goreleaser.yaml: public archives must contain only the cross-platform server build")
for private_binary in ("migrate-private-only", "verify-private-cutover-report"):
    if private_binary in archives_contract:
        raise SystemExit(f".goreleaser.yaml: {private_binary} must remain image-internal")
if (ROOT / ".goreleaser.yaml").read_text(encoding="utf-8").count("      - migrate-private-only") != 2:
    raise SystemExit(".goreleaser.yaml: every production Docker target must include the offline cutover build")
if (ROOT / ".goreleaser.yaml").read_text(encoding="utf-8").count("      - verify-private-cutover-report") != 2:
    raise SystemExit(".goreleaser.yaml: every production Docker target must include the cutover verifier build")
if (ROOT / ".goreleaser.simple.yaml").exists():
    raise SystemExit(".goreleaser.simple.yaml: x86-only production release path must remain removed")
release_lines = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8").splitlines()
backend_gate_index = release_lines.index("      - name: Backend unit and integration tests")
if release_lines[backend_gate_index + 1].strip() != "working-directory: backend":
    raise SystemExit(".github/workflows/release.yml: backend quality gate must run in backend/")
release_workflow = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
if release_workflow.count("ref: ${{ github.event.inputs.tag || github.ref }}") != 1:
    raise SystemExit(".github/workflows/release.yml: the mutable event ref may only be used by resolve-ref")
if release_workflow.count("ref: ${{ needs.resolve-ref.outputs.sha }}") != 2:
    raise SystemExit(".github/workflows/release.yml: every consuming job must checkout the one resolved release SHA")
tag_pattern_match = re.search(r"^\s*RELEASE_TAG_PATTERN: '([^']+)'\s*$", release_workflow, re.MULTILINE)
if not tag_pattern_match:
    raise SystemExit(".github/workflows/release.yml: missing explicit release tag pattern")
release_tag_pattern = re.compile(tag_pattern_match.group(1))
for accepted_tag in ("v0.2.0", "v0.2.0-rc.1", "v12.34.56-beta.2"):
    if release_tag_pattern.fullmatch(accepted_tag) is None:
        raise SystemExit(f".github/workflows/release.yml: release tag pattern rejects {accepted_tag}")
for rejected_tag in ("v0.2.0+build.1", "v0.2.0-rc.1+build.1", "0.2.0", "v0.2"):
    if release_tag_pattern.fullmatch(rejected_tag) is not None:
        raise SystemExit(f".github/workflows/release.yml: release tag pattern accepts unsafe {rejected_tag}")
require(".github/workflows/release.yml", 'while grep -Fxq "$delimiter" <<<"$TAG_MESSAGE"')
require(".github/workflows/release.yml", "secrets.token_hex(16)")
if release_workflow.count("TAG_MESSAGE: ${{ steps.tag_message.outputs.message }}") != 2:
    raise SystemExit(".github/workflows/release.yml: tag messages must reach consumers only through step environment values")
if release_workflow.count("${{ steps.tag_message.outputs.message }}") != 2:
    raise SystemExit(".github/workflows/release.yml: tag-message expressions must not be interpolated into shell source")
forbid(".github/workflows/release.yml", "message<<EOF")
forbid(".github/workflows/release.yml", "TAG_MESSAGE='${{ steps.tag_message.outputs.message }}'")
forbid(".github/workflows/release.yml", 'echo "$TAG_MESSAGE"')
forbid(".github/workflows/release.yml", "--skip=validate")
forbid(".github/workflows/release.yml", "simple_release")
forbid(".github/workflows/release.yml", "SIMPLE_RELEASE")
forbid(".github/workflows/release.yml", "DOCKERHUB")
forbid(".github/workflows/release.yml", "dockerhub")
forbid(".goreleaser.yaml", "DOCKERHUB")
forbid(".goreleaser.yaml", "docker.io/")
require(".goreleaser.yaml", "candidate-{{ .Version }}-amd64")
require(".goreleaser.yaml", "candidate-{{ .Version }}-arm64")
require(".goreleaser.yaml", 'draft: true')
require(".goreleaser.yaml", 'prerelease: auto')
forbid(".goreleaser.yaml", 'prerelease: false')
require(".github/workflows/release.yml", "Promote verified digest to final GHCR tag")
require(".github/workflows/release.yml", 'gh release edit "$RELEASE_TAG" --draft=false')
require(".github/workflows/release.yml", "GORELEASER_CURRENT_TAG: ${{ env.RELEASE_TAG }}")
require(".github/workflows/release.yml", "upload-release-assets: false")
require(".github/workflows/release.yml", 'gh release upload "$RELEASE_TAG" image.spdx.json --clobber')
forbid(".github/workflows/release.yml", "upload-release-assets: true")
release_steps = [line.strip() for line in release_lines]
for earlier, later in (
    ("- name: Attach SPDX SBOM to draft GitHub release", "- name: Publish verified GitHub release"),
    ("- name: Attest SLSA build provenance", "- name: Promote verified digest to final GHCR tag"),
    ("- name: Attest SPDX SBOM", "- name: Promote verified digest to final GHCR tag"),
    ("- name: Promote verified digest to final GHCR tag", "- name: Publish verified GitHub release"),
):
    if release_steps.index(earlier) >= release_steps.index(later):
        raise SystemExit(f".github/workflows/release.yml: {earlier} must precede {later}")

for path in (
    "deploy/install.sh",
    "deploy/docker-compose.local.yml",
    "deploy/docker-compose.standalone.yml",
    "deploy/apple-container.sh",
    "deploy/README.md",
):
    forbid(path, "Wei-Shaw/sub2api")
    forbid(path, "weishaw/sub2api:latest")

for path in (
    "deploy/docker-compose.yml",
    "deploy/docker-compose.local.yml",
    "deploy/docker-compose.standalone.yml",
    "deploy/docker-compose.dev.yml",
):
    require(path, "RUN_MODE=${RUN_MODE:-simple}")
    forbid(path, "RUN_MODE=${RUN_MODE:-standard}")
require("deploy/.env.example", "RUN_MODE=simple")
require("deploy/config.example.yaml", 'run_mode: "simple"')

require("deploy/install.sh", "Checksum retrieval and archive lookup fail closed.")
require("deploy/install.sh", 'curl -fsSL "$checksum_url"')
forbid("deploy/install.sh", 'print_warning "$(msg \'checksum_not_found\')"')

for path in (
    "deploy/docker-compose.yml",
    "deploy/docker-compose.local.yml",
    "deploy/docker-compose.standalone.yml",
):
    require(path, "${EXAPI_IMAGE:?")
    require(path, "http://localhost:8080/ready")
    forbid(path, "http://localhost:8080/health")

for path in ("deploy/docker-compose.yml", "deploy/docker-compose.local.yml"):
    require(path, "${POSTGRES_IMAGE:?Set POSTGRES_IMAGE to an immutable postgres@sha256:<digest> reference}")
    require(path, "${REDIS_IMAGE:?Set REDIS_IMAGE to an immutable redis@sha256:<digest> reference}")
    if (ROOT / path).read_text(encoding="utf-8").count("no-new-privileges:true") != 3:
        raise SystemExit(f"{path}: every service must enable no-new-privileges")
require("deploy/docker-compose.standalone.yml", "no-new-privileges:true")
for path in ("deploy/docker-compose.yml", "deploy/docker-compose.local.yml", "deploy/docker-compose.standalone.yml"):
    require(path, "${EXAPI_STOP_GRACE_PERIOD:-50s}")

for path in (
    "deploy/docker-compose.yml",
    "deploy/docker-compose.local.yml",
    "deploy/docker-compose.standalone.yml",
    "deploy/docker-compose.dev.yml",
):
    require(path, "SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=${SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID:?")
    require(path, "SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON=${SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON:?")
    require(path, "container_name: ${EXAPI_CONTAINER_NAME:-")

require("deploy/install.sh", "validate_runtime_keyring_independence")
require("deploy/docker-deploy.sh", "validate_independent_keyrings")
require("deploy/PRODUCTION_ROLLOUT.md", "COMPOSE_PROJECT_NAME=exapi-canary")
require("deploy/PRODUCTION_ROLLOUT.md", "docker-compose.canary-restored.yml")
require("deploy/PRODUCTION_ROLLOUT.md", "publish-rollout-manifest.sh")
require("deploy/docker-compose.canary-restored.yml", "internal: true")
require("deploy/docker-compose.canary-restored.yml", "external: true")
require("deploy/docker-compose.canary-restored.yml", "RESTORED_POSTGRES_DATABASE")
if (ROOT / "deploy/PRODUCTION_ROLLOUT.md").read_text(encoding="utf-8").count(
    "docker compose --env-file /protected/exapi-canary-restored.env"
) != 2:
    raise SystemExit("deploy/PRODUCTION_ROLLOUT.md: restored canary config/up must use its protected env file")
require("deploy/ops/create-recovery-set.sh", "EXAPI_WRITER_QUIESCED=true")
require("deploy/ops/create-recovery-set.sh", "s3api get-bucket-versioning")
require("deploy/ops/create-recovery-set.sh", "put-object-retention")
require("deploy/ops/create-recovery-set.sh", "validate-immutable-compose.sh")
require("deploy/ops/create-recovery-set.sh", "database and secret recovery objects must have disjoint age recipients")
require("deploy/ops/validate-snapshot-evidence.py", "snapshot retention_until is shorter than RECOVERY_RETENTION_UNTIL")
require("deploy/ops/validate-immutable-compose.sh", "@sha256:[0-9a-f]{64}")
require("deploy/ops/verify-logical-restore.sh", "--version-id")
require("deploy/ops/verify-logical-restore.sh", "umask 077")
require("deploy/ops/adapters/prove-rollout-network", "O_NOFOLLOW")
require("deploy/ops/adapters/prove-rollout-network", "metadata.st_uid != 0")
require("deploy/ops/adapters/prove-rollout-network", "stat.S_IMODE(metadata.st_mode) != 0o600")
require("deploy/ops/adapters/prove-rollout-network", "restored_counts_evidence_sha256")
require("deploy/ops/adapters/prove-rollout-network", "restored_counts_evidence_rollout_id")
forbid("deploy/ops/adapters/prove-rollout-network", "1\\|23\\|8\\|0\\|*")
require("deploy/ops/verify-snapshot-restore.sh", '"egress_denied"')
require("deploy/ops/observe-rollout.sh", "minimum_seconds=1800")
require("deploy/ops/observe-rollout.sh", "minimum_seconds=3600")
require("deploy/ops/publish-rollout-manifest.sh", "cosign")
require("deploy/ops/publish-rollout-manifest.sh", "cosign verify-attestation")
require("deploy/ops/publish-rollout-manifest.sh", "oci-provenance-verification.json")
require("deploy/ops/publish-rollout-manifest.sh", "immortal-autumn/ExAPI/\\.github/workflows/release\\.yml@")
require("deploy/ops/rollout-manifest.example.json", '"independent_restore_paths": false')
require("deploy/ops/rollout-manifest.example.json", '"postgres_volume": "REPLACE_VERIFIED_EXTERNAL_POSTGRES_VOLUME"')

for required in (
    "id-token: write",
    "attestations: write",
    "format: spdx-json",
    "subject-digest: ${{ steps.candidate-image.outputs.digest }}",
    "push-to-registry: true",
    "Verify OCI labels and embedded version",
    "docker.io/tonistiigi/binfmt@sha256:400a4873b838d1b89194d982c45e5fb3cda4593fbfd7e08a02e76b03b21166f0",
    "image=moby/buildkit@sha256:2f5adac4ecd194d9f8c10b7b5d7bceb5186853db1b26e5abd3a657af0b7e26ec",
    "version: v0.36.1",
    "version: v2.17.1",
    "syft-version: v1.42.3",
):
    require(".github/workflows/release.yml", required)

require("deploy/apple-container.sh", "validate_immutable_app_image")
require("deploy/apple-container.sh", "SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON")
require(".goreleaser.yaml", "/sub2api2personal:candidate-{{ .Version }}")
for path in (".goreleaser.yaml",):
    forbid(path, "/sub2api:latest")
    forbid(path, "/sub2api2personal:latest")
    require(path, "go -C backend mod tidy -diff")
    require(path, "-X main.Version={{.Version}}")
    require(path, "--label=org.opencontainers.image.created={{ .Date }}")
    require(path, "--label=org.opencontainers.image.source=https://github.com/{{ .Env.GITHUB_REPO_OWNER }}/{{ .Env.GITHUB_REPO_NAME }}")

for stale in (
    "${{ secrets.DOCKERHUB_USERNAME }}/sub2api\n",
    "${DOCKERHUB_USERNAME}/sub2api\"",
    "/pkgs/container/sub2api)",
):
    forbid(".github/workflows/release.yml", stale)

for workflow in (ROOT / ".github/workflows").glob("*.yml"):
    for line_number, line in enumerate(workflow.read_text(encoding="utf-8").splitlines(), 1):
        match = re.search(r"\buses:\s*([^\s#]+)", line)
        if not match or match.group(1).startswith("./"):
            continue
        ref = match.group(1).rsplit("@", 1)[-1]
        if not re.fullmatch(r"[0-9a-f]{40}", ref):
            raise SystemExit(f"{workflow.relative_to(ROOT)}:{line_number}: action is not pinned to a commit SHA")

print("ExAPI CI, release, and artifact provenance contracts validated.")
