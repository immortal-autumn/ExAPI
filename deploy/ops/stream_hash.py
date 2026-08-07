#!/usr/bin/env python3
"""Pass stdin to stdout while recording a prefixed SHA-256 digest."""

from __future__ import annotations

import hashlib
import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: stream_hash.py OUTPUT", file=sys.stderr)
        return 2
    output = Path(sys.argv[1])
    digest = hashlib.sha256()
    while chunk := sys.stdin.buffer.read(1024 * 1024):
        digest.update(chunk)
        sys.stdout.buffer.write(chunk)
    sys.stdout.buffer.flush()
    output.write_text("sha256:" + digest.hexdigest() + "\n", encoding="ascii")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
