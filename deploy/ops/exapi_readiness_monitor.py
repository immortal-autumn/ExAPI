#!/usr/bin/env python3
"""Low-noise, off-host JSON readiness monitor for ExAPI.

The production application already has a Docker health check.  This monitor is
an independent *notification* probe and deliberately requires several
consecutive observations before changing the confirmed state.  It is kept
dependency-free so it can run from a small workstation user service.

The state and event files are private (0700/0600), writes are atomic, and the
event log is bounded.  A failed probe is not itself a desktop notification:
three consecutive failed contract probes are required by default, followed by
two consecutive healthy probes for recovery.  A short outage immediately
after recovery is suppressed by the notification cooldown and is delivered
only if it remains unhealthy after that cooldown.
"""

from __future__ import annotations

import argparse
import contextlib
import datetime as dt
import fcntl
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
from typing import Any, Callable, Iterator, Mapping, MutableMapping, Sequence
from urllib.parse import urlsplit
import uuid

try:  # pragma: no cover - resource is present on the Linux deployment host
    import resource
except ImportError:  # pragma: no cover - keeps the module importable elsewhere
    resource = None  # type: ignore[assignment]


SCHEMA_VERSION = 2
DEFAULT_TARGET_URL = "https://sub2api.research.for-immortal.cn/ready"
DEFAULT_STATE_DIR = os.path.expanduser("~/.local/state/exapi-readiness-monitor")
DEFAULT_FAILURE_THRESHOLD = 3
DEFAULT_RECOVERY_THRESHOLD = 2
DEFAULT_NOTIFICATION_COOLDOWN_SECONDS = 15 * 60
DEFAULT_DELIVERY_RETRY_SECONDS = 5 * 60
DEFAULT_REMINDER_SECONDS = 0  # Disabled: one notification per incident by default.
DEFAULT_PROBE_INTERVAL_SECONDS = 30
DEFAULT_PROBE_GAP_RESET_SECONDS = 120
DEFAULT_NOTIFICATION_EXPIRE_MS = 120_000
DEFAULT_MAX_EVENT_BYTES = 4 * 1024 * 1024
DEFAULT_MAX_EVENT_LINES = 12_000
DEFAULT_MAX_RESPONSE_BYTES = 64 * 1024
DEFAULT_CONNECT_TIMEOUT_SECONDS = 5
DEFAULT_REQUEST_TIMEOUT_SECONDS = 15


def utc_now() -> dt.datetime:
    return dt.datetime.now(dt.timezone.utc)


def timestamp(value: dt.datetime | None = None) -> str:
    value = value or utc_now()
    return value.astimezone(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def parse_timestamp(value: Any) -> dt.datetime | None:
    if not isinstance(value, str) or not value:
        return None
    try:
        parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=dt.timezone.utc)
    return parsed.astimezone(dt.timezone.utc)


def seconds_since(now: dt.datetime, value: Any) -> float | None:
    parsed = parse_timestamp(value)
    if parsed is None:
        return None
    # A clock adjustment into the future must not bypass a cooldown.
    return max(0.0, (now - parsed).total_seconds())


def safe_target_label(target_url: str) -> str:
    """Return a notification-safe URL without credentials/query material."""

    parsed = urlsplit(target_url)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        return "configured readiness endpoint"
    host = parsed.hostname or parsed.netloc
    if ":" in host and not host.startswith("["):
        host = f"[{host}]"
    try:
        port = parsed.port
    except ValueError:
        port = None
    netloc = host if port is None else f"{host}:{port}"
    return f"{parsed.scheme}://{netloc}{parsed.path or '/'}"


def evidence_target_url(target_url: str) -> str:
    """Return the target representation safe to persist in state/proof/logs."""

    parsed = urlsplit(target_url)
    # Query strings are allowed for local testing/custom gateways, but may
    # contain bearer material.  The actual curl request still uses the full
    # configured URL; only persisted evidence is redacted.
    return safe_target_label(target_url) if parsed.query or parsed.fragment else target_url


def validate_target_url(target_url: str) -> None:
    if not isinstance(target_url, str) or any(ord(char) < 0x20 or ord(char) == 0x7F for char in target_url):
        raise ValueError("target URL must not contain control characters")
    try:
        parsed = urlsplit(target_url)
    except ValueError as exc:
        raise ValueError("target URL is malformed") from exc
    if parsed.scheme not in {"http", "https"} or not parsed.hostname:
        raise ValueError("target URL must be an http(s) URL with a host")
    if parsed.username or parsed.password:
        raise ValueError("target URL must not contain credentials")
    if parsed.query or parsed.fragment:
        raise ValueError("target URL must not contain a query string or fragment")


def env_int(name: str, default: int, *, minimum: int = 0, maximum: int = 86_400) -> int:
    raw = os.environ.get(name)
    if raw is None or raw.strip() == "":
        return default
    try:
        value = int(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be an integer") from exc
    if value < minimum or value > maximum:
        raise ValueError(f"{name} must be between {minimum} and {maximum}")
    return value


class MonitorConfig:
    def __init__(
        self,
        *,
        target_url: str = DEFAULT_TARGET_URL,
        state_dir: str | os.PathLike[str] = DEFAULT_STATE_DIR,
        failure_threshold: int = DEFAULT_FAILURE_THRESHOLD,
        recovery_threshold: int = DEFAULT_RECOVERY_THRESHOLD,
        notification_cooldown_seconds: int = DEFAULT_NOTIFICATION_COOLDOWN_SECONDS,
        delivery_retry_seconds: int = DEFAULT_DELIVERY_RETRY_SECONDS,
        reminder_seconds: int = DEFAULT_REMINDER_SECONDS,
        max_event_bytes: int = DEFAULT_MAX_EVENT_BYTES,
        max_event_lines: int = DEFAULT_MAX_EVENT_LINES,
        max_response_bytes: int = DEFAULT_MAX_RESPONSE_BYTES,
        connect_timeout_seconds: int = DEFAULT_CONNECT_TIMEOUT_SECONDS,
        request_timeout_seconds: int = DEFAULT_REQUEST_TIMEOUT_SECONDS,
        probe_interval_seconds: int = DEFAULT_PROBE_INTERVAL_SECONDS,
        probe_gap_reset_seconds: int = DEFAULT_PROBE_GAP_RESET_SECONDS,
        curl_bin: str = "curl",
        notify_send_bin: str = "notify-send",
    ) -> None:
        validate_target_url(target_url)
        if failure_threshold < 1 or recovery_threshold < 1:
            raise ValueError("failure/recovery thresholds must be positive")
        if notification_cooldown_seconds < 0 or delivery_retry_seconds < 0 or reminder_seconds < 0:
            raise ValueError("notification intervals must not be negative")
        if max_event_bytes < 16 * 1024 or max_event_lines < 100:
            raise ValueError("event retention limits are too small")
        if max_response_bytes < 1024:
            raise ValueError("max response size is too small")
        if connect_timeout_seconds < 1 or request_timeout_seconds < connect_timeout_seconds:
            raise ValueError("request timeouts are invalid")
        if probe_interval_seconds < 1 or probe_gap_reset_seconds < probe_interval_seconds:
            raise ValueError("probe interval/gap reset values are invalid")
        self.target_url = target_url
        self.state_dir = Path(state_dir).expanduser()
        self.failure_threshold = failure_threshold
        self.recovery_threshold = recovery_threshold
        self.notification_cooldown_seconds = notification_cooldown_seconds
        self.delivery_retry_seconds = delivery_retry_seconds
        self.reminder_seconds = reminder_seconds
        self.max_event_bytes = max_event_bytes
        self.max_event_lines = max_event_lines
        self.max_response_bytes = max_response_bytes
        self.connect_timeout_seconds = connect_timeout_seconds
        self.request_timeout_seconds = request_timeout_seconds
        self.probe_interval_seconds = probe_interval_seconds
        self.probe_gap_reset_seconds = probe_gap_reset_seconds
        self.curl_bin = curl_bin
        self.notify_send_bin = notify_send_bin

    @classmethod
    def from_environment(cls) -> "MonitorConfig":
        return cls(
            target_url=os.environ.get("EXAPI_MONITOR_TARGET_URL", DEFAULT_TARGET_URL),
            state_dir=os.environ.get("EXAPI_MONITOR_STATE_DIR", DEFAULT_STATE_DIR),
            failure_threshold=env_int("EXAPI_MONITOR_FAILURE_THRESHOLD", DEFAULT_FAILURE_THRESHOLD, minimum=1, maximum=100),
            recovery_threshold=env_int("EXAPI_MONITOR_RECOVERY_THRESHOLD", DEFAULT_RECOVERY_THRESHOLD, minimum=1, maximum=100),
            notification_cooldown_seconds=env_int(
                "EXAPI_MONITOR_NOTIFICATION_COOLDOWN_SECONDS",
                DEFAULT_NOTIFICATION_COOLDOWN_SECONDS,
                maximum=7 * 86_400,
            ),
            delivery_retry_seconds=env_int(
                "EXAPI_MONITOR_DELIVERY_RETRY_SECONDS",
                DEFAULT_DELIVERY_RETRY_SECONDS,
                maximum=86_400,
            ),
            reminder_seconds=env_int(
                "EXAPI_MONITOR_REMINDER_SECONDS",
                DEFAULT_REMINDER_SECONDS,
                maximum=30 * 86_400,
            ),
            max_event_bytes=env_int(
                "EXAPI_MONITOR_MAX_EVENT_BYTES",
                DEFAULT_MAX_EVENT_BYTES,
                minimum=16 * 1024,
                maximum=64 * 1024 * 1024,
            ),
            max_event_lines=env_int(
                "EXAPI_MONITOR_MAX_EVENT_LINES",
                DEFAULT_MAX_EVENT_LINES,
                minimum=100,
                maximum=1_000_000,
            ),
            max_response_bytes=env_int(
                "EXAPI_MONITOR_MAX_RESPONSE_BYTES",
                DEFAULT_MAX_RESPONSE_BYTES,
                minimum=1024,
                maximum=10 * 1024 * 1024,
            ),
            connect_timeout_seconds=env_int("EXAPI_MONITOR_CONNECT_TIMEOUT_SECONDS", DEFAULT_CONNECT_TIMEOUT_SECONDS, minimum=1, maximum=60),
            request_timeout_seconds=env_int("EXAPI_MONITOR_REQUEST_TIMEOUT_SECONDS", DEFAULT_REQUEST_TIMEOUT_SECONDS, minimum=1, maximum=120),
            probe_interval_seconds=env_int(
                "EXAPI_MONITOR_PROBE_INTERVAL_SECONDS", DEFAULT_PROBE_INTERVAL_SECONDS, minimum=1, maximum=3_600
            ),
            probe_gap_reset_seconds=env_int(
                "EXAPI_MONITOR_PROBE_GAP_RESET_SECONDS", DEFAULT_PROBE_GAP_RESET_SECONDS, minimum=1, maximum=86_400
            ),
            curl_bin=os.environ.get("EXAPI_MONITOR_CURL_BIN", "curl"),
            notify_send_bin=os.environ.get("EXAPI_MONITOR_NOTIFY_SEND_BIN", "notify-send"),
        )

    @property
    def state_path(self) -> Path:
        return self.state_dir / "state.json"

    @property
    def events_path(self) -> Path:
        return self.state_dir / "events.ndjson"

    @property
    def proof_path(self) -> Path:
        return self.state_dir / "alert-delivery-evidence.json"

    @property
    def lock_path(self) -> Path:
        return self.state_dir / "monitor.lock"

    @property
    def evidence_target_url(self) -> str:
        return evidence_target_url(self.target_url)


def ensure_state_dir(path: Path) -> None:
    path.mkdir(mode=0o700, parents=True, exist_ok=True)
    try:
        path.chmod(0o700)
    except OSError:
        pass


def atomic_write_bytes(path: Path, data: bytes, mode: int = 0o600) -> None:
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    fd, tmp_name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    tmp_path = Path(tmp_name)
    try:
        os.fchmod(fd, mode)
        with os.fdopen(fd, "wb") as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(tmp_path, path)
        try:
            path.chmod(mode)
        except OSError:
            pass
    finally:
        with contextlib.suppress(FileNotFoundError):
            tmp_path.unlink()


def atomic_write_json(path: Path, value: Mapping[str, Any]) -> None:
    payload = json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True).encode("utf-8") + b"\n"
    atomic_write_bytes(path, payload)


def load_json(
    path: Path,
    fallback: Mapping[str, Any],
    *,
    quarantine_invalid: bool = False,
) -> dict[str, Any]:
    invalid = False
    try:
        with path.open("r", encoding="utf-8") as handle:
            value = json.load(handle)
        if isinstance(value, dict):
            return value
        invalid = True
    except (OSError, ValueError, TypeError):
        invalid = path.exists()
    if invalid and quarantine_invalid:
        with contextlib.suppress(OSError):
            os.replace(path, path.with_name(path.name + f".corrupt-{time.time_ns()}"))
    return dict(fallback)


def default_state(target_url: str) -> dict[str, Any]:
    return {
        "schema_version": SCHEMA_VERSION,
        "target_url": target_url,
        "status": "unknown",
        "failure_streak": 0,
        "success_streak": 0,
        "incident_id": None,
        "incident_started_at": None,
        "incident_alerted": False,
        "alert_deferred": False,
        "recovery_pending": False,
        "last_alert_attempt_at": None,
        "last_alert_delivered_at": None,
        "last_recovery_attempt_at": None,
        "last_recovery_delivered_at": None,
        "last_recovery_at": None,
        "cooldown_until": None,
        "last_unhealthy_notification_at": None,
        "last_notification_attempt_at": None,
        "last_notification_kind": None,
        "last_observed_at": None,
    }


def migrate_state(raw: Mapping[str, Any], target_url: str) -> dict[str, Any]:
    """Migrate state conservatively, never manufacturing an alert."""

    if not isinstance(raw, Mapping) or raw.get("target_url") != target_url:
        return default_state(target_url)
    state = default_state(target_url)
    state.update({key: value for key, value in raw.items() if key in state})
    legacy = raw.get("schema_version") != SCHEMA_VERSION
    persisted_status = state.get("status")
    state["status"] = persisted_status if isinstance(persisted_status, str) and persisted_status in {
        "unknown",
        "healthy",
        "unhealthy",
    } else "unknown"
    # The v1 shell monitor did not persist whether an alert was delivered.  An
    # old unhealthy value is therefore untrusted and must satisfy the normal
    # failure threshold after upgrade.
    if legacy and state["status"] == "unhealthy":
        return default_state(target_url)
    state["schema_version"] = SCHEMA_VERSION
    state["target_url"] = target_url
    state["failure_streak"] = bounded_count(state.get("failure_streak"))
    state["success_streak"] = bounded_count(state.get("success_streak"))
    for key in ("incident_alerted", "alert_deferred", "recovery_pending"):
        state[key] = state.get(key) is True
    state["last_observed_at"] = state.get("last_observed_at") or raw.get("observed_at")
    if legacy:
        # A legacy healthy observation is a safe baseline, but stale v1
        # notification timestamps must not delay a future incident.
        state["incident_alerted"] = False
        state["alert_deferred"] = False
        state["recovery_pending"] = False
        state["incident_id"] = None
        state["incident_started_at"] = None
        state["cooldown_until"] = None
        state["last_alert_attempt_at"] = None
        state["last_recovery_attempt_at"] = None
    return state


def bounded_count(value: Any, *, maximum: int = 1_000_000) -> int:
    """Convert an untrusted persisted counter to a safe bounded integer."""

    if isinstance(value, bool):
        return 0
    try:
        number = int(value)
    except (TypeError, ValueError, OverflowError):
        return 0
    return min(max(number, 0), maximum)


def hash_file(path: Path, *, max_bytes: int | None = None) -> str:
    digest = hashlib.sha256()
    try:
        with path.open("rb") as handle:
            remaining = max_bytes
            while True:
                read_size = 64 * 1024 if remaining is None else min(64 * 1024, remaining)
                if read_size <= 0:
                    break
                block = handle.read(read_size)
                if not block:
                    break
                digest.update(block)
                if remaining is not None:
                    remaining -= len(block)
    except OSError:
        return hashlib.sha256(b"").hexdigest()
    return digest.hexdigest()


def classify_failure(
    curl_exit: int,
    http_code: int,
    body_size: int,
    body_valid: bool,
    max_response_bytes: int = DEFAULT_MAX_RESPONSE_BYTES,
) -> str:
    if body_size > max_response_bytes:
        return "response_too_large_or_invalid"
    if curl_exit == 6:
        return "dns_resolution"
    if curl_exit == 7:
        return "connection_failed"
    if curl_exit == 28:
        return "timeout"
    if curl_exit != 0:
        return "curl_error"
    if http_code in {502, 503, 504}:
        return "upstream_unavailable"
    if http_code != 200:
        return "http_status"
    if body_size >= max_response_bytes and not body_valid:
        return "response_too_large_or_invalid"
    return "readiness_contract"


def probe_readiness(config: MonitorConfig) -> dict[str, Any]:
    """Run one bounded curl probe and return a redacted result."""

    ensure_state_dir(config.state_dir)
    with tempfile.TemporaryDirectory(prefix=".probe-", dir=config.state_dir) as work_dir:
        work = Path(work_dir)
        headers_path = work / "headers"
        body_path = work / "body"
        command = [
            config.curl_bin,
            "--silent",
            "--show-error",
            "--connect-timeout",
            str(config.connect_timeout_seconds),
            "--max-time",
            str(config.request_timeout_seconds),
            "--max-filesize",
            str(config.max_response_bytes),
            "--dump-header",
            str(headers_path),
            "--output",
            str(body_path),
            "--write-out",
            "%{http_code}",
            "--url",
            config.target_url,
        ]
        run_kwargs: dict[str, Any] = {
            "check": False,
            "capture_output": True,
            "text": True,
            "timeout": config.request_timeout_seconds + 5,
        }
        if resource is not None:
            # --max-filesize relies on Content-Length and can be bypassed by a
            # chunked response.  A child file-size limit provides a second,
            # kernel-enforced bound on temporary-disk growth.
            limit = config.max_response_bytes + 4 * 1024

            def limit_probe_file() -> None:
                assert resource is not None
                hard = resource.getrlimit(resource.RLIMIT_FSIZE)[1]
                if hard != resource.RLIM_INFINITY:
                    limit_value = min(limit, hard)
                else:
                    limit_value = limit
                resource.setrlimit(resource.RLIMIT_FSIZE, (limit_value, limit_value))

            run_kwargs["preexec_fn"] = limit_probe_file
        try:
            completed = subprocess.run(command, **run_kwargs)
            curl_exit = int(completed.returncode)
            stdout = completed.stdout.strip().splitlines()
            try:
                http_code = int(stdout[-1]) if stdout else 0
            except ValueError:
                http_code = 0
        except (OSError, subprocess.TimeoutExpired):
            curl_exit = 124
            http_code = 0

        headers = ""
        with contextlib.suppress(OSError):
            headers = headers_path.read_text(encoding="utf-8", errors="replace")
        body = b""
        with contextlib.suppress(OSError):
            # Read one byte beyond the configured bound so an unexpectedly
            # large response is classified without loading it into memory.
            with body_path.open("rb") as body_handle:
                body = body_handle.read(config.max_response_bytes + 1)
        try:
            body_size = body_path.stat().st_size
        except OSError:
            body_size = len(body)
        body_hash = hash_file(body_path, max_bytes=config.max_response_bytes)
        content_type_ok = any(
            line.lower().startswith("content-type:") and "application/json" in line.lower()
            for line in headers.splitlines()
        )
        body_valid = False
        if body_size <= config.max_response_bytes and content_type_ok:
            try:
                decoded = json.loads(body.decode("utf-8"))
                body_valid = isinstance(decoded, dict) and decoded.get("status") in {"ready", "ok"}
            except (UnicodeDecodeError, ValueError, TypeError):
                body_valid = False
        healthy = curl_exit == 0 and http_code == 200 and body_valid
        return {
            "status": "healthy" if healthy else "unhealthy",
            "http_status": http_code,
            "curl_exit_code": curl_exit,
            "body_sha256": body_hash,
            "body_size": body_size,
            "failure_reason": None
            if healthy
            else classify_failure(curl_exit, http_code, body_size, body_valid, config.max_response_bytes),
        }


def trim_event_log(path: Path, max_bytes: int, max_lines: int) -> None:
    try:
        size = path.stat().st_size
    except OSError:
        return
    # Always inspect the line count; a small file can still contain an
    # unbounded number of records.  For a large historical file, only a
    # bounded tail is needed.
    read_size = min(size, max(max_bytes * 2, 1 * 1024 * 1024))
    try:
        with path.open("rb") as handle:
            handle.seek(size - read_size)
            data = handle.read(read_size)
    except OSError:
        return
    lines = data.splitlines(keepends=True)
    if size > read_size and lines:
        lines = lines[1:]
    # Select newest complete lines while enforcing both limits.  A malformed
    # partial line is discarded rather than copied into the next NDJSON file.
    retained_reversed: list[bytes] = []
    used = 0
    for line in reversed(lines):
        if not line.endswith(b"\n"):
            continue
        if len(line) > max_bytes or used + len(line) > max_bytes:
            if retained_reversed:
                break
            continue
        retained_reversed.append(line)
        used += len(line)
        if len(retained_reversed) >= max_lines:
            break
    retained = list(reversed(retained_reversed))
    # Avoid an unnecessary rewrite when already within both bounds.
    if len(retained) == len(lines) and used == size:
        return
    atomic_write_bytes(path, b"".join(retained))


def append_event(path: Path, event: Mapping[str, Any], config: MonitorConfig) -> None:
    trim_event_log(path, config.max_event_bytes, config.max_event_lines)
    line = json.dumps(event, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8") + b"\n"
    try:
        with path.open("ab") as handle:
            try:
                os.fchmod(handle.fileno(), 0o600)
            except OSError:
                pass
            handle.write(line)
            handle.flush()
            os.fsync(handle.fileno())
    except OSError:
        # A notification must not be retried solely because diagnostics could
        # not be written.  The next run will retry the state/proof atomically.
        pass
    trim_event_log(path, config.max_event_bytes, config.max_event_lines)


def archive_file(path: Path, label: str) -> None:
    """Move an incompatible evidence file aside without deleting it."""

    if not path.exists():
        return
    with contextlib.suppress(OSError):
        os.replace(path, path.with_name(f"{path.name}.{label}-{time.time_ns()}"))


def prepare_proof(path: Path, event: Mapping[str, Any]) -> dict[str, Any]:
    """Load v1 proof while failing closed on host/target contamination."""

    defaults = {
        "schema_version": 1,
        "monitor_host": event["monitor_host"],
        "target_url": event["target_url"],
    }
    proof = load_json(path, defaults, quarantine_invalid=True)
    incompatible = (
        proof.get("schema_version") != 1
        or proof.get("monitor_host") != event["monitor_host"]
        or proof.get("target_url") != event["target_url"]
    )
    if incompatible:
        archive_file(path, "previous")
        return defaults
    for key in ("last_alert", "last_recovery", "last_alert_observation", "last_recovery_observation"):
        nested = proof.get(key)
        if nested is not None and (
            not isinstance(nested, dict)
            or nested.get("monitor_host") != event["monitor_host"]
            or nested.get("target_url") != event["target_url"]
        ):
            proof.pop(key, None)
    if not isinstance(proof.get("last_alert"), dict) and not isinstance(proof.get("last_recovery"), dict):
        proof.pop("delivery_accepted", None)
    return proof


def should_retry(
    now: dt.datetime,
    state: Mapping[str, Any],
    config: MonitorConfig,
    kind: str = "unhealthy",
) -> bool:
    key = "last_alert_attempt_at" if kind == "unhealthy" else "last_recovery_attempt_at"
    age = seconds_since(now, state.get(key))
    # v1/v2 transitional state may have only the shared field.  It is used for
    # unhealthy retries, but never allowed to delay the first recovery.
    if kind == "unhealthy" and age is None:
        age = seconds_since(now, state.get("last_notification_attempt_at"))
    return age is None or age >= config.delivery_retry_seconds


def cooldown_elapsed(now: dt.datetime, state: Mapping[str, Any], config: MonitorConfig) -> bool:
    until = parse_timestamp(state.get("cooldown_until"))
    if until is not None:
        return now >= until
    # There is deliberately no fallback to last_unhealthy_notification_at:
    # cooldown starts after confirmed recovery, not after the old alert.
    return True


def update_state(
    state: MutableMapping[str, Any],
    raw_status: str,
    now: dt.datetime,
    config: MonitorConfig,
) -> tuple[dict[str, Any], str | None, str | None]:
    """Apply one raw observation; return (state, notification kind, reason)."""

    raw_status = raw_status if raw_status in {"healthy", "unhealthy"} else "unhealthy"
    old_status = state.get("status", "unknown")
    if old_status not in {"unknown", "healthy", "unhealthy"}:
        old_status = "unknown"

    # Timer suspension/reboot must not turn non-consecutive observations into
    # a confirmed outage.  A future timestamp is treated as a gap as well.
    previous = parse_timestamp(state.get("last_observed_at"))
    gap_reset = False
    if previous is not None:
        elapsed = (now - previous).total_seconds()
        gap_reset = elapsed < 0 or elapsed > config.probe_gap_reset_seconds
    failure_before = 0 if gap_reset else bounded_count(state.get("failure_streak"))
    success_before = 0 if gap_reset else bounded_count(state.get("success_streak"))
    if raw_status == "healthy":
        failure_streak = 0
        success_streak = success_before + 1
    else:
        failure_streak = failure_before + 1
        success_streak = 0

    confirmed_status = old_status
    transition: str | None = None
    if old_status == "unknown":
        if raw_status == "healthy" and success_streak >= config.recovery_threshold:
            confirmed_status = "healthy"
            transition = "healthy"
        elif raw_status == "unhealthy" and failure_streak >= config.failure_threshold:
            confirmed_status = "unhealthy"
            transition = "unhealthy"
    elif old_status == "healthy" and raw_status == "unhealthy" and failure_streak >= config.failure_threshold:
        confirmed_status = "unhealthy"
        transition = "unhealthy"
    elif old_status == "unhealthy" and raw_status == "healthy" and success_streak >= config.recovery_threshold:
        confirmed_status = "healthy"
        transition = "healthy"

    now_text = timestamp(now)
    state["schema_version"] = SCHEMA_VERSION
    state["status"] = confirmed_status
    state["failure_streak"] = failure_streak
    state["success_streak"] = success_streak
    state["last_observed_at"] = now_text
    state["target_url"] = config.evidence_target_url
    if gap_reset:
        state["last_streak_reset_at"] = now_text

    notification: str | None = None
    reason: str | None = "probe_gap_reset" if gap_reset else None

    if transition == "unhealthy":
        state["incident_id"] = str(uuid.uuid4())
        state["incident_started_at"] = now_text
        state["incident_alerted"] = False
        # A new incident supersedes an undelivered recovery notice from the
        # previous incident.  The recovery timestamp/cooldown remains active.
        state["recovery_pending"] = False
        if cooldown_elapsed(now, state, config):
            state["alert_deferred"] = False
        else:
            state["alert_deferred"] = True

    if confirmed_status == "unhealthy" and raw_status == "unhealthy":
        if not state.get("incident_id"):
            state["incident_id"] = str(uuid.uuid4())
            state["incident_started_at"] = now_text
        if not state.get("incident_alerted", False):
            if not cooldown_elapsed(now, state, config):
                state["alert_deferred"] = True
                reason = "notification_cooldown"
            elif should_retry(now, state, config, "unhealthy"):
                state["alert_deferred"] = False
                notification = "unhealthy"
                reason = None
            else:
                reason = "delivery_retry_backoff"
        elif config.reminder_seconds > 0:
            delivered_age = seconds_since(now, state.get("last_alert_delivered_at"))
            if delivered_age is not None and delivered_age >= config.reminder_seconds:
                if should_retry(now, state, config, "unhealthy"):
                    notification = "unhealthy"
                else:
                    reason = "delivery_retry_backoff"
    elif transition == "healthy":
        if state.get("incident_alerted", False):
            state["recovery_pending"] = True
            state["last_recovery_at"] = now_text
            state["cooldown_until"] = timestamp(
                now + dt.timedelta(seconds=config.notification_cooldown_seconds)
            )
            state["alert_deferred"] = False
            # Recovery has its own retry clock, so an alert attempt moments
            # earlier never delays the first recovery notice.
            if should_retry(now, state, config, "healthy"):
                notification = "healthy"
                reason = None
            else:
                reason = "delivery_retry_backoff"
        else:
            # A muted flap never produces a recovery notification.
            state["incident_id"] = None
            state["incident_started_at"] = None
            state["recovery_pending"] = False
            state["alert_deferred"] = False
            reason = reason or "no_alerted_incident"
    elif confirmed_status == "healthy" and raw_status == "healthy" and state.get("recovery_pending", False):
        if should_retry(now, state, config, "healthy"):
            notification = "healthy"
            reason = None
        else:
            reason = "delivery_retry_backoff"

    return dict(state), notification, reason


def send_notification(kind: str, probe: Mapping[str, Any], config: MonitorConfig) -> tuple[bool, int]:
    target = safe_target_label(config.target_url)
    if kind == "unhealthy":
        title = "ExAPI readiness alert"
        urgency = "critical"
        body = (
            f"{target} failed the JSON readiness contract "
            f"(HTTP {probe.get('http_status', 0)}) after a confirmed failure streak."
        )
    else:
        title = "ExAPI readiness recovered"
        urgency = "normal"
        body = f"{target} passed the JSON readiness contract after a confirmed recovery streak."
    command = [
        config.notify_send_bin,
        "--app-name=ExAPI",
        f"--expire-time={DEFAULT_NOTIFICATION_EXPIRE_MS}",
        f"--urgency={urgency}",
        title,
        body,
    ]
    try:
        completed = subprocess.run(command, check=False, capture_output=True, timeout=5)
        return completed.returncode == 0, int(completed.returncode)
    except (OSError, subprocess.TimeoutExpired):
        return False, 124


def run_once(
    config: MonitorConfig,
    *,
    now: dt.datetime | None = None,
    probe: Mapping[str, Any] | None = None,
    notifier: Callable[[str, Mapping[str, Any], MonitorConfig], tuple[bool, int]] = send_notification,
    send_notifications: bool = True,
) -> dict[str, Any] | None:
    ensure_state_dir(config.state_dir)
    now = now or utc_now()
    raw_state = load_json(
        config.state_path,
        default_state(config.evidence_target_url),
        quarantine_invalid=True,
    )
    state = migrate_state(raw_state, config.evidence_target_url)
    if probe is None:
        probe = probe_readiness(config)
    raw_status = str(probe.get("status", "unhealthy"))
    if raw_status not in {"healthy", "unhealthy"}:
        raw_status = "unhealthy"
    state, notification, suppression_reason = update_state(state, raw_status, now, config)

    delivery_exit = -1
    delivery_kind = "none"
    alert_sent = False
    if notification is not None and send_notifications:
        delivery_kind = "desktop-notification"
        attempt_at = timestamp(now)
        if notification == "unhealthy":
            state["last_alert_attempt_at"] = attempt_at
        else:
            state["last_recovery_attempt_at"] = attempt_at
        # Retain the v1 shared field for older diagnostics, but never use it
        # as the recovery retry clock.
        state["last_notification_attempt_at"] = attempt_at
        state["last_notification_kind"] = notification
        delivered, delivery_exit = notifier(notification, probe, config)
        alert_sent = delivered
        if delivered and notification == "unhealthy":
            state["incident_alerted"] = True
            state["alert_deferred"] = False
            state["last_alert_delivered_at"] = attempt_at
            state["last_unhealthy_notification_at"] = attempt_at
        elif delivered and notification == "healthy":
            state["incident_alerted"] = False
            state["recovery_pending"] = False
            state["last_recovery_delivered_at"] = attempt_at
            state["incident_id"] = None
            state["incident_started_at"] = None
    elif notification is not None:
        # Dry-runs may record that a notification would be sent, but must not
        # consume either retry budget or alter delivery state.
        delivery_kind = "dry-run"
        suppression_reason = suppression_reason or "dry_run"
    event = {
        "schema_version": SCHEMA_VERSION,
        "observed_at": timestamp(now),
        "monitor_host": os.uname().nodename,
        "target_url": config.evidence_target_url,
        "previous_status": raw_state.get("status")
        if isinstance(raw_state.get("status"), str)
        and raw_state.get("status") in {"unknown", "healthy", "unhealthy"}
        else "unknown",
        "raw_status": raw_status,
        "status": state["status"],
        "failure_streak": state["failure_streak"],
        "success_streak": state["success_streak"],
        "incident_id": state.get("incident_id"),
        "http_status": bounded_count(probe.get("http_status"), maximum=999),
        "curl_exit_code": bounded_count(probe.get("curl_exit_code"), maximum=255),
        "failure_reason": probe.get("failure_reason"),
        "body_sha256": str(probe.get("body_sha256", hashlib.sha256(b"").hexdigest())),
        "body_hash_scope": "prefix" if bounded_count(probe.get("body_size")) > config.max_response_bytes else "full",
        "body_size": bounded_count(probe.get("body_size")),
        "alert_sent": alert_sent,
        "delivery_kind": delivery_kind,
        "delivery_exit_code": delivery_exit,
        "suppression_reason": suppression_reason,
    }
    proof = prepare_proof(config.proof_path, event)
    proof["schema_version"] = 1
    proof["monitor_schema_version"] = SCHEMA_VERSION
    proof["monitor_host"] = event["monitor_host"]
    proof["target_url"] = config.evidence_target_url
    proof["last_probe"] = event
    if alert_sent:
        proof["delivery_accepted"] = True
        proof["delivery_kind"] = delivery_kind
        if notification == "unhealthy":
            proof["last_alert_observation"] = event
            # Keep the v1 adapter's canonical 503/curl=0 evidence stable even
            # if a later notification was caused by DNS/timeout noise.
            if (
                event["http_status"] == 503
                and event["curl_exit_code"] == 0
            ) or "last_alert" not in proof:
                proof["last_alert"] = event
        elif notification == "healthy":
            proof["last_recovery_observation"] = event
            if (event["http_status"] == 200 and event["curl_exit_code"] == 0) or "last_recovery" not in proof:
                proof["last_recovery"] = event
    append_event(config.events_path, event, config)
    atomic_write_json(config.state_path, state)
    atomic_write_json(config.proof_path, proof)
    return event


@contextlib.contextmanager
def monitor_lock(path: Path) -> Iterator[bool]:
    ensure_state_dir(path.parent)
    handle = path.open("a+")
    try:
        try:
            fcntl.flock(handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            yield False
            return
        try:
            os.fchmod(handle.fileno(), 0o600)
        except OSError:
            pass
        yield True
    finally:
        with contextlib.suppress(OSError):
            fcntl.flock(handle.fileno(), fcntl.LOCK_UN)
        handle.close()


def parse_args(argv: Sequence[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dry-run", action="store_true", help="probe and record state without sending a desktop notification")
    return parser.parse_args(argv)


def main(argv: Sequence[str] | None = None) -> int:
    os.umask(0o077)
    try:
        config = MonitorConfig.from_environment()
        args = parse_args(argv or sys.argv[1:])
    except ValueError as exc:
        print(f"exapi-readiness-monitor: {exc}", file=sys.stderr)
        return 2

    with monitor_lock(config.lock_path) as acquired:
        if not acquired:
            return 0
        try:
            run_once(config, notifier=send_notification, send_notifications=not args.dry_run)
        except Exception as exc:  # Keep the timer healthy; do not leak probe data.
            print(f"exapi-readiness-monitor: run failed: {type(exc).__name__}", file=sys.stderr)
            return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
