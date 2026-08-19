#!/usr/bin/env python3
import json
import hashlib
import hmac
import os
import threading
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

EXPECTED_KEY = os.environ.get("SYNTHETIC_UPSTREAM_KEY", "")
ROLLOUT_ID = os.environ.get("SYNTHETIC_ROLLOUT_ID", "")
if not EXPECTED_KEY:
    raise SystemExit("SYNTHETIC_UPSTREAM_KEY is required")
if not ROLLOUT_ID:
    raise SystemExit("SYNTHETIC_ROLLOUT_ID is required")

MAX_BODY_BYTES = 1024 * 1024
TOKEN_FINGERPRINT = hashlib.sha256(EXPECTED_KEY.encode()).hexdigest()
COUNTS = {
    "chat_completions": 0,
    "responses": 0,
    "models": 0,
    "auth_failures": 0,
    "oversized_requests": 0,
    "last_request": None,
}
COUNTS_LOCK = threading.Lock()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        return

    def send_json(self, status, payload):
        raw = json.dumps(payload, separators=(",", ":")).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def authorized(self):
        supplied = self.headers.get("Authorization", "")
        valid = hmac.compare_digest(supplied, f"Bearer {EXPECTED_KEY}")
        if not valid:
            with COUNTS_LOCK:
                COUNTS["auth_failures"] += 1
            self.send_json(401, {"error": {"message": "invalid synthetic credential"}})
        return valid

    def do_GET(self):
        if self.path == "/health":
            self.send_json(200, {"status": "ok"})
            return
        if self.path == "/stats":
            with COUNTS_LOCK:
                counts = dict(COUNTS)
            counts["rollout_id"] = ROLLOUT_ID
            counts["expected_token_sha256"] = TOKEN_FINGERPRINT
            self.send_json(200, counts)
            return
        if self.path == "/v1/models":
            if not self.authorized():
                return
            with COUNTS_LOCK:
                COUNTS["models"] += 1
            self.send_json(200, {"object": "list", "data": [{"id": "synthetic-model", "object": "model"}]})
            return
        self.send_json(404, {"error": {"message": "not found"}})

    def do_POST(self):
        if self.path not in ("/v1/chat/completions", "/v1/responses"):
            self.send_json(404, {"error": {"message": "not found"}})
            return
        if not self.authorized():
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            self.send_json(400, {"error": {"message": "invalid content length"}})
            return
        if length < 0 or length > MAX_BODY_BYTES:
            with COUNTS_LOCK:
                COUNTS["oversized_requests"] += 1
            self.send_json(413, {"error": {"message": "synthetic request too large"}})
            return
        self.connection.settimeout(5)
        body = self.rfile.read(length)
        request_metadata = {
            "path": self.path,
            "request_id": self.headers.get("X-Request-Id", ""),
            "observed_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
            "body_sha256": hashlib.sha256(body).hexdigest(),
            "authorization_sha256": hashlib.sha256(
                self.headers.get("Authorization", "").removeprefix("Bearer ").encode()
            ).hexdigest(),
        }
        with COUNTS_LOCK:
            COUNTS["last_request"] = request_metadata
        if self.path == "/v1/responses":
            with COUNTS_LOCK:
                COUNTS["responses"] += 1
            self.send_json(200, {
                "id": "synthetic-response",
                "object": "response",
                "status": "completed",
                "model": "synthetic-model",
                "output": [{
                    "type": "message",
                    "id": "synthetic-message",
                    "role": "assistant",
                    "status": "completed",
                    "content": [{"type": "output_text", "text": "synthetic-ok"}],
                }],
                "usage": {"input_tokens": 1, "output_tokens": 1, "total_tokens": 2},
            })
            return
        with COUNTS_LOCK:
            COUNTS["chat_completions"] += 1
        self.send_json(200, {
            "id": "synthetic-response",
            "object": "chat.completion",
            "choices": [{"index": 0, "message": {"role": "assistant", "content": "synthetic-ok"}, "finish_reason": "stop"}],
            "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
        })


if __name__ == "__main__":
    bind = os.environ.get("MOCK_PROVIDER_BIND", "127.0.0.1")
    server = ThreadingHTTPServer((bind, 19091), Handler)
    server.daemon_threads = True
    server.request_queue_size = 16
    server.serve_forever()
