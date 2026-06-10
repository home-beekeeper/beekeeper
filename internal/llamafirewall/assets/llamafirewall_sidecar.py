#!/usr/bin/env python3
"""LlamaFirewall sidecar — supervised by beekeeper.

Launched and monitored by the Go Supervisor in
internal/llamafirewall/supervisor.go. It listens on a LOOPBACK TCP socket
(127.0.0.1:$BEEKEEPER_LLMF_PORT) and responds to length-prefixed JSON scan
requests using the same 4-byte big-endian length framing as the Go IPC layer.

Phase 20 (LLMF) fixes the prior SILENT FAIL-OPEN no-op:
  * the LlamaFirewall scanners are constructed ONCE at startup (not per request,
    which reloaded the model every scan),
  * scan_prompt calls lf.scan(UserMessage(content=...)) — the old code passed a
    bogus role kwarg to the constructor, which raised a swallowed TypeError that
    returned "clean" for every prompt,
  * scan_code uses the real CODE_SHIELD scanner on an AssistantMessage,
  * any exception now returns an explicit "error" field AND a fail-closed
    sentinel result the Go layer BLOCKS on (never "clean" on error),
  * the cloud AlignmentCheck (Together AI) path is removed entirely.

Access control: the Go supervisor passes a per-launch bearer token via
$BEEKEEPER_LLMF_TOKEN; every request must carry a matching "token" or it is
rejected. This restores the access control the old 0600 unix socket provided.
"""
import json
import os
import socket
import struct
import sys

MAX_MSG = 1024 * 1024  # 1 MB — matches Go maxMessageSize

# Fail-closed sentinel: when the scanner errors, return this result + an "error"
# field. The Go layer maps a populated "error" to a blocking (fail-closed)
# decision, so a sidecar fault can never silently allow.
RESULT_ERROR = "error"


def recv_msg(conn):
    """Read a length-prefixed JSON message; return the dict, or None on clean EOF."""
    hdr = _recv_all(conn, 4)
    if hdr is None or len(hdr) < 4:
        return None
    size = struct.unpack(">I", hdr)[0]
    if size > MAX_MSG:
        raise ValueError(f"message too large: {size}")
    data = _recv_all(conn, size)
    if data is None:
        return None
    return json.loads(data)


def _recv_all(conn, n):
    """Read exactly n bytes (MSG_WAITALL is unreliable on some platforms)."""
    buf = bytearray()
    while len(buf) < n:
        chunk = conn.recv(n - len(buf))
        if not chunk:
            return None if not buf else bytes(buf)
        buf.extend(chunk)
    return bytes(buf)


def send_msg(conn, obj):
    """Write obj as a length-prefixed JSON message to conn."""
    data = json.dumps(obj).encode()
    conn.sendall(struct.pack(">I", len(data)) + data)


class Scanner:
    """Holds the LlamaFirewall instance, constructed ONCE at startup with the
    PROMPT_GUARD (USER role) and CODE_SHIELD (ASSISTANT role) scanners."""

    def __init__(self):
        from llamafirewall import LlamaFirewall, Role, ScannerType

        self._lf = LlamaFirewall(
            scanners={
                Role.USER: [ScannerType.PROMPT_GUARD],
                Role.ASSISTANT: [ScannerType.CODE_SHIELD],
            }
        )

    def scan_prompt(self, content):
        from llamafirewall import UserMessage, ScanDecision

        result = self._lf.scan(UserMessage(content=content))
        if result.decision != ScanDecision.ALLOW:
            return "injection", float(getattr(result, "score", 0.0) or 0.9), str(getattr(result, "reason", "") or "")
        return "clean", 0.0, ""

    def scan_code(self, content):
        from llamafirewall import AssistantMessage, ScanDecision

        result = self._lf.scan(AssistantMessage(content=content))
        if result.decision != ScanDecision.ALLOW:
            return "unsafe", float(getattr(result, "score", 0.0) or 0.9), str(getattr(result, "reason", "") or "")
        return "clean", 0.0, ""


def handle_scan(scanner: Scanner, req: dict) -> dict:
    """Evaluate a scan request and return a scan response dict.

    On ANY exception the response sets result="error" and populates "error" so
    the Go layer treats it fail-closed (block) — never "clean".
    """
    import time

    kind = req.get("kind", "")
    content = req.get("content", "")
    request_id = req.get("request_id", "")
    start = time.time()

    try:
        if kind == "scan_prompt":
            result, confidence, reason = scanner.scan_prompt(content)
        elif kind == "scan_code":
            result, confidence, reason = scanner.scan_code(content)
        else:
            raise ValueError(f"unknown scan kind: {kind}")
    except Exception as e:  # noqa: BLE001 — fail closed on ANY error
        return {
            "request_id": request_id,
            "result": RESULT_ERROR,
            "confidence": 0.0,
            "latency_ms": int((time.time() - start) * 1000),
            "error": str(e),
        }

    return {
        "request_id": request_id,
        "result": result,
        "confidence": confidence,
        "reason": reason,
        "latency_ms": int((time.time() - start) * 1000),
    }


def main():
    """Bind a loopback TCP socket and serve token-authenticated scan requests."""
    port = int(os.environ.get("BEEKEEPER_LLMF_PORT", "0"))
    token = os.environ.get("BEEKEEPER_LLMF_TOKEN", "")
    if port == 0 or token == "":
        print("llamafirewall_sidecar: BEEKEEPER_LLMF_PORT/TOKEN required", file=sys.stderr, flush=True)
        sys.exit(2)

    # Build the firewall ONCE — fail closed loudly if the model/import is missing
    # (e.g. gated model not accepted, venv not bootstrapped). The supervisor then
    # treats the dead sidecar as fail-closed rather than silently allowing.
    scanner = Scanner()

    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind(("127.0.0.1", port))
    srv.listen(8)
    print(f"llamafirewall_sidecar: listening on 127.0.0.1:{port}", flush=True)

    while True:
        conn, _ = srv.accept()
        try:
            req = recv_msg(conn)
            if req is None:
                continue
            if req.get("token", "") != token:
                # Reject: an unauthenticated local process. Do not scan.
                send_msg(conn, {
                    "request_id": req.get("request_id", ""),
                    "result": RESULT_ERROR,
                    "confidence": 0.0,
                    "latency_ms": 0,
                    "error": "unauthorized: token mismatch",
                })
                continue
            send_msg(conn, handle_scan(scanner, req))
        except Exception as e:  # noqa: BLE001
            print(f"llamafirewall_sidecar: error: {e}", file=sys.stderr, flush=True)
        finally:
            conn.close()


if __name__ == "__main__":
    main()
