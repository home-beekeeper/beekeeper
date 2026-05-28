#!/usr/bin/env python3
"""LlamaFirewall sidecar — supervised by beekeeper.

This process is launched and monitored by the Go Supervisor in
internal/llamafirewall/supervisor.go. It listens on a Unix domain socket
(~/.beekeeper/llamafirewall.sock) and responds to length-prefixed JSON scan
requests from the supervisor using the same 4-byte big-endian length framing as
the Go IPC layer.

The sidecar is intentionally minimal on startup — it only imports LlamaFirewall
when a scan_prompt request is actually received, so that the process starts and
opens its socket quickly (the supervisor waits up to 2 seconds for the socket).
"""
import json
import os
import socket
import struct
import sys

SOCK_PATH = os.path.expanduser("~/.beekeeper/llamafirewall.sock")
MAX_MSG = 1024 * 1024  # 1 MB — matches Go maxMessageSize


def recv_msg(conn):
    """Read a length-prefixed JSON message from conn.

    Returns the parsed dict, or None if the connection was closed cleanly.
    Raises ValueError if the declared size exceeds MAX_MSG.
    """
    hdr = conn.recv(4, socket.MSG_WAITALL)
    if len(hdr) < 4:
        return None
    size = struct.unpack(">I", hdr)[0]
    if size > MAX_MSG:
        raise ValueError(f"message too large: {size}")
    data = conn.recv(size, socket.MSG_WAITALL)
    return json.loads(data)


def send_msg(conn, obj):
    """Write obj as a length-prefixed JSON message to conn."""
    data = json.dumps(obj).encode()
    conn.sendall(struct.pack(">I", len(data)) + data)


def handle_scan(req: dict) -> dict:
    """Evaluate a scan request and return a scan response dict.

    Supported kinds:
      scan_prompt    — prompt-injection detection via LlamaFirewall.
      scan_code      — code-safety scan (placeholder; real CodeShield call TBD).
      scan_alignment — goal-hijacking scan (placeholder).

    On any exception the response sets result="clean" and populates "error" so
    the supervisor can surface the failure without blocking (fail-closed is
    enforced at the Go layer, not here).
    """
    import time

    kind = req.get("kind", "")
    content = req.get("content", "")
    request_id = req.get("request_id", "")
    start = time.time()

    result = "clean"
    confidence = 0.0
    reason = ""

    try:
        if kind == "scan_prompt":
            from llamafirewall import LlamaFirewall, UserMessage, ScanDecision

            lf = LlamaFirewall()
            decision = lf.scan(UserMessage(role="user", content=content))
            if decision.decision != ScanDecision.ALLOW:
                result = "injection"
                confidence = float(decision.score or 0.9)
                reason = str(decision.reason or "")
        elif kind == "scan_code":
            # CodeShield placeholder — replace with real CodeShield call when
            # the CodeShield Python package is integrated.
            result = "clean"
        elif kind == "scan_alignment":
            # Alignment check placeholder — future integration point.
            result = "clean"
    except Exception as e:
        latency_ms = int((time.time() - start) * 1000)
        return {
            "request_id": request_id,
            "result": "clean",
            "confidence": 0.0,
            "latency_ms": latency_ms,
            "error": str(e),
        }

    latency_ms = int((time.time() - start) * 1000)
    return {
        "request_id": request_id,
        "result": result,
        "confidence": confidence,
        "reason": reason,
        "latency_ms": latency_ms,
    }


def main():
    """Start the Unix socket server and serve scan requests."""
    # Remove any stale socket from a previous run.
    if os.path.exists(SOCK_PATH):
        os.unlink(SOCK_PATH)

    srv = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    srv.bind(SOCK_PATH)
    # Restrict socket to owner only (0o600) — the Go supervisor runs as the
    # same user, so this is sufficient. World-readable sockets would allow any
    # local process to submit scan requests.
    os.chmod(SOCK_PATH, 0o600)
    srv.listen(1)

    print(f"llamafirewall_sidecar: listening on {SOCK_PATH}", flush=True)

    while True:
        conn, _ = srv.accept()
        try:
            req = recv_msg(conn)
            if req is None:
                continue
            resp = handle_scan(req)
            send_msg(conn, resp)
        except Exception as e:
            print(
                f"llamafirewall_sidecar: error: {e}",
                file=sys.stderr,
                flush=True,
            )
        finally:
            conn.close()


if __name__ == "__main__":
    main()
