#!/usr/bin/env python3
"""Verify that the audited upstream base and hotfixes exist in this checkout."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def git(*args: str) -> str:
    return subprocess.check_output(["git", *args], cwd=ROOT, text=True).strip()


def main() -> int:
    lock = json.loads((ROOT / "upstream.lock.json").read_text())
    commits = [lock["base_commit"], *[item["commit"] for item in lock["selected_hotfixes"]]]
    applied = [lock["base_commit"], *[item.get("applied_commit", item["commit"]) for item in lock["selected_hotfixes"]]]
    missing = []
    for commit in commits:
        try:
            git("cat-file", "-e", f"{commit}^{{commit}}")
        except subprocess.CalledProcessError:
            missing.append(commit)
    if missing:
        print("missing locked upstream commits: " + ", ".join(missing), file=sys.stderr)
        return 1

    head = git("rev-parse", "HEAD")
    for commit in applied:
        if subprocess.call(["git", "merge-base", "--is-ancestor", commit, head], cwd=ROOT) != 0:
            print(f"locked commit is not an ancestor of HEAD: {commit}", file=sys.stderr)
            return 1
    print(f"upstream lock verified: {len(commits)} commits, HEAD={head}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
