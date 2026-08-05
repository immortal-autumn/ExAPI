#!/usr/bin/env python3
"""List the largest source files in the ExAPI repository.

Generated/vendor/build outputs are skipped so the report highlights files that
are realistic refactor targets.
"""
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SKIP_PARTS = {
    ".git",
    "node_modules",
    "dist",
    "build",
    ".pnpm-store",
    "coverage",
    "tmp",
}
EXTS = {
    ".vue",
    ".ts",
    ".tsx",
    ".js",
    ".go",
    ".md",
    ".yaml",
    ".yml",
    ".json",
    ".css",
    ".scss",
}


def should_skip(path: Path) -> bool:
    return any(part in SKIP_PARTS for part in path.parts)


def count_lines(path: Path) -> int:
    with path.open("r", encoding="utf-8", errors="ignore") as handle:
        return sum(1 for _ in handle)


def main() -> None:
    rows: list[tuple[int, Path]] = []
    for path in ROOT.rglob("*"):
        if not path.is_file() or should_skip(path) or path.suffix not in EXTS:
            continue
        try:
            rows.append((count_lines(path), path.relative_to(ROOT)))
        except OSError:
            continue

    for lines, path in sorted(rows, reverse=True)[:50]:
        print(f"{lines:6d} {path}")


if __name__ == "__main__":
    main()
