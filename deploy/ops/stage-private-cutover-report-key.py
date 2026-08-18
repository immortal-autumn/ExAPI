#!/usr/bin/env python3
"""Safely stage the private-cutover report key without exposing its bytes."""

from __future__ import annotations

import hashlib
import json
import os
import stat
import sys
from typing import NoReturn


def fail(message: str) -> NoReturn:
    raise SystemExit(f"private cutover report-key staging failed: {message}")


def open_absolute_without_symlinks(path: str) -> int:
    if not os.path.isabs(path):
        fail("source key path must be absolute")
    components = [component for component in path.split(os.sep) if component]
    if not components:
        fail("source key path must name a file")

    nofollow = getattr(os, "O_NOFOLLOW", 0)
    if nofollow == 0:
        fail("this platform cannot reject key-path symlinks")
    directory_flags = os.O_RDONLY | os.O_DIRECTORY | nofollow
    directory_fd = os.open(os.sep, directory_flags)
    try:
        for component in components[:-1]:
            next_fd = os.open(component, directory_flags, dir_fd=directory_fd)
            os.close(directory_fd)
            directory_fd = next_fd
        return os.open(components[-1], os.O_RDONLY | nofollow, dir_fd=directory_fd)
    except OSError as exc:
        fail(f"cannot open source key without following symlinks: {exc}")
    finally:
        os.close(directory_fd)


def read_valid_key(source_fd: int) -> bytes:
    info_before = os.fstat(source_fd)
    if not stat.S_ISREG(info_before.st_mode):
        fail("source key must be a regular file")
    if stat.S_IMODE(info_before.st_mode) != 0o600:
        fail("source key mode must be 0600")
    if info_before.st_nlink != 1:
        fail("source key must have exactly one hard link")

    data = b""
    while len(data) <= 65:
        chunk = os.read(source_fd, 66 - len(data))
        if not chunk:
            break
        data += chunk
    if len(data) != 65 or data[-1:] != b"\n":
        fail("source key must contain exactly 64 lowercase hexadecimal characters and one newline")
    encoded_key = data[:-1]
    if any(value not in b"0123456789abcdef" for value in encoded_key):
        fail("source key must use lowercase hexadecimal characters only")

    info_after = os.fstat(source_fd)
    identity_before = (
        info_before.st_dev,
        info_before.st_ino,
        info_before.st_mode,
        info_before.st_nlink,
        info_before.st_size,
        info_before.st_mtime_ns,
        info_before.st_ctime_ns,
    )
    identity_after = (
        info_after.st_dev,
        info_after.st_ino,
        info_after.st_mode,
        info_after.st_nlink,
        info_after.st_size,
        info_after.st_mtime_ns,
        info_after.st_ctime_ns,
    )
    if identity_after != identity_before:
        fail("source key changed while it was being staged")
    return data


def write_staged_key(destination: str, data: bytes) -> None:
    if not os.path.isabs(destination):
        fail("staged key path must be absolute")
    nofollow = getattr(os, "O_NOFOLLOW", 0)
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | nofollow
    destination_fd = -1
    try:
        destination_fd = os.open(destination, flags, 0o600)
        view = memoryview(data)
        while view:
            written = os.write(destination_fd, view)
            if written <= 0:
                fail("short write while staging key")
            view = view[written:]
        os.fchmod(destination_fd, 0o600)
        os.fsync(destination_fd)
        info = os.fstat(destination_fd)
        if not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o600 or info.st_nlink != 1:
            fail("staged key is not a single-link regular 0600 file")
    except BaseException:
        if destination_fd >= 0:
            try:
                os.unlink(destination)
            except FileNotFoundError:
                pass
        raise
    finally:
        if destination_fd >= 0:
            os.close(destination_fd)


def main() -> None:
    if len(sys.argv) != 3:
        fail("usage: stage-private-cutover-report-key.py SOURCE DESTINATION")
    source, destination = sys.argv[1:]
    source_fd = open_absolute_without_symlinks(source)
    try:
        data = read_valid_key(source_fd)
    finally:
        os.close(source_fd)
    write_staged_key(destination, data)

    decoded_key = bytes.fromhex(data[:-1].decode("ascii"))
    print(
        json.dumps(
            {
                "key_file_sha256": hashlib.sha256(data).hexdigest(),
                "report_key_sha256": hashlib.sha256(decoded_key).hexdigest(),
            },
            separators=(",", ":"),
            sort_keys=True,
        )
    )


if __name__ == "__main__":
    main()
