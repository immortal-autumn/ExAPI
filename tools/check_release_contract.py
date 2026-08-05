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
require("Dockerfile", "corepack prepare pnpm@9.15.9 --activate")
require("deploy/Dockerfile", "corepack prepare pnpm@9.15.9 --activate")

require(".github/workflows/release.yml", "quality-gate")
require(".github/workflows/release.yml", "resolve-ref:")
require(".github/workflows/release.yml", "needs: [resolve-ref, update-version, quality-gate]")
require(".github/workflows/release.yml", "ref: ${{ needs.resolve-ref.outputs.sha }}")
require(".github/workflows/release.yml", "python3 tools/check_release_contract.py")
release_lines = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8").splitlines()
backend_gate_index = release_lines.index("      - name: Backend unit and integration tests")
if release_lines[backend_gate_index + 1].strip() != "working-directory: backend":
    raise SystemExit(".github/workflows/release.yml: backend quality gate must run in backend/")
release_workflow = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
if release_workflow.count("ref: ${{ github.event.inputs.tag || github.ref }}") != 1:
    raise SystemExit(".github/workflows/release.yml: the mutable event ref may only be used by resolve-ref")
if release_workflow.count("ref: ${{ needs.resolve-ref.outputs.sha }}") != 3:
    raise SystemExit(".github/workflows/release.yml: every consuming job must checkout the one resolved release SHA")
forbid(".github/workflows/release.yml", "--skip=validate")

for path in (
    "deploy/install.sh",
    "deploy/docker-compose.local.yml",
    "deploy/docker-compose.standalone.yml",
    "deploy/apple-container.sh",
    "deploy/README.md",
):
    forbid(path, "Wei-Shaw/sub2api")
    forbid(path, "weishaw/sub2api:latest")

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

for path in (
    "deploy/docker-compose.yml",
    "deploy/docker-compose.local.yml",
    "deploy/docker-compose.standalone.yml",
    "deploy/docker-compose.dev.yml",
):
    require(path, "SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID=${SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID:?")
    require(path, "SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON=${SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON:?")

require("deploy/install.sh", "validate_runtime_keyring_independence")
require("deploy/docker-deploy.sh", "validate_independent_keyrings")

require("deploy/apple-container.sh", "validate_immutable_app_image")
require("deploy/apple-container.sh", "SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON")
require(".goreleaser.yaml", "/sub2api2personal:{{ .Version }}")
require(".goreleaser.simple.yaml", "/sub2api2personal:{{ .Version }}")
for path in (".goreleaser.yaml", ".goreleaser.simple.yaml"):
    forbid(path, "/sub2api:latest")
    forbid(path, "/sub2api2personal:latest")
    require(path, "go -C backend mod tidy -diff")

for workflow in (ROOT / ".github/workflows").glob("*.yml"):
    for line_number, line in enumerate(workflow.read_text(encoding="utf-8").splitlines(), 1):
        match = re.search(r"\buses:\s*([^\s#]+)", line)
        if not match or match.group(1).startswith("./"):
            continue
        ref = match.group(1).rsplit("@", 1)[-1]
        if not re.fullmatch(r"[0-9a-f]{40}", ref):
            raise SystemExit(f"{workflow.relative_to(ROOT)}:{line_number}: action is not pinned to a commit SHA")

print("ExAPI CI, release, and artifact provenance contracts validated.")
