#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""A local stand-in for the Anthropic Messages API.

It exists so the e2e lane can drive a REAL harness TUI through Gangline with no
network and no provider account. It speaks only as much of the dialect as the
harness demands, and it FAILS LOUDLY on anything else: an unrecognised path is
logged and answered with an error rather than a plausible empty success, so a
harness that starts asking for something new breaks the lane visibly.

What it answers is chosen by markers the test puts in the prompt it sends,
never by a scenario language. Three markers exist and each is hardcoded below.

The request log is the lane's primary instrument. Every request body reaching
this server is written to it, so a test can assert on what actually entered the
model's context rather than on what a pane appeared to show.

A log line is one of three phases. A `request` line carries the path, whether
the body was the agent's own turn or one of the harness's side errands, and
whether this server understood it at all; a `held` line names the request being
frozen; a `complete` line says a request was answered to the last byte. Arrival
and delivery are therefore distinguishable, and so are the agent's turns from
everything else the harness asks for — without which "the model saw it" is a
claim about the wrong request.
"""

import argparse
import json
import os
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

# A prompt carrying this marker holds its response open until the lane opens the
# gate FIFO for writing. Opening a FIFO for reading blocks until a writer
# arrives, so the hold is an event barrier rather than a sleep.
HOLD_MARKER = "GANGLINE-E2E-HOLD"
# A prompt carrying this marker is answered with a provider error instead of a
# completion.
ERROR_MARKER = "GANGLINE-E2E-ERROR"
# Every ordinary completion contains this, so a test can tell a stub answer from
# anything else on the screen.
REPLY_PREFIX = "E2E-STUB-REPLY"


class StubServer(ThreadingHTTPServer):
    """Carries the lane's instrument and barriers to each handler."""

    daemon_threads = True

    def __init__(self, address, handler, recorder, gate, held, turn_sentinel):
        super().__init__(address, handler)
        self.recorder = recorder
        self.gate = gate
        self.held = held
        self.turn_sentinel = turn_sentinel
        self.hold_lock = threading.Lock()
        self.held_once = False

    def is_agent_turn(self, text):
        """Is this the agent's own turn, or one of the harness's side errands?

        NOT EVERY REQUEST CARRYING THE MARKER IS THE TURN. Observed on
        claude-code 2.1.233: each submitted prompt also triggers a small
        auxiliary completion — a session-title call — whose body quotes the
        user's message and therefore carries the marker too, and which arrives
        BEFORE the real turn. Holding that one instead would release the lane
        while the turn it meant to freeze ran to completion unheld.

        The caller supplies a sentinel that only the agent's own turn carries.
        With none supplied every marked request qualifies.
        """
        return not self.turn_sentinel or self.turn_sentinel in text

    def hold(self, seq):
        """Announce the live turn by number, then block until it is released.

        Opening a FIFO for writing blocks until a reader arrives and opening
        one for reading blocks until a writer does, so this pair is a two-way
        barrier: the lane learns the turn is live from the turn itself, and the
        turn ends when the lane says so. Neither side polls.

        THE NUMBER IS THE POINT. The lane cannot otherwise tell which request
        it froze, and every completion this stub writes looks alike apart from
        it. Sending the sequence number here is what lets a test name the held
        turn instead of accepting any turn's answer as evidence.

        AT MOST ONE REQUEST EVER HOLDS. Every later turn replays the whole
        conversation, marker included, so a marker-matching hold would fire
        again on a turn the lane never meant to stop — and the lane, having
        already spent its one release, would leave the harness blocked while
        its assertions passed. The marker means the FIRST request carrying it.
        """
        with self.hold_lock:
            if self.held_once:
                return
            self.held_once = True
        self.recorder.record({"phase": "held", "for": seq})
        if self.held:
            with open(self.held, "wb") as announce:
                announce.write(f"{seq}\n".encode("utf-8"))
        if self.gate:
            with open(self.gate, "rb"):
                pass


class Recorder:
    """Append-only request log. One JSON object per line."""

    def __init__(self, path):
        self.path = path
        self.lock = threading.Lock()
        self.count = 0

    def record(self, entry):
        with self.lock:
            self.count += 1
            entry["seq"] = self.count
            with open(self.path, "a", encoding="utf-8") as log:
                log.write(json.dumps(entry, sort_keys=True) + "\n")
            return self.count


def prompt_text(body):
    """Every text the request carries, flattened, newline-joined.

    System prompt and message content both matter: Gangline passes the standing
    contract through a launch option into the system prompt, and delivers
    envelopes as user turns.
    """
    parts = []

    def walk(value):
        if isinstance(value, str):
            parts.append(value)
        elif isinstance(value, list):
            for item in value:
                walk(item)
        elif isinstance(value, dict):
            if isinstance(value.get("text"), str):
                parts.append(value["text"])
            else:
                for item in value.values():
                    walk(item)

    walk(body.get("system"))
    walk(body.get("messages"))
    return "\n".join(parts)


def sse(event, payload):
    return f"event: {event}\ndata: {json.dumps(payload)}\n\n".encode("utf-8")


def message_events(model, text):
    """The Anthropic streaming event sequence for one plain text completion."""
    yield sse(
        "message_start",
        {
            "type": "message_start",
            "message": {
                "id": "msg_e2e_stub",
                "type": "message",
                "role": "assistant",
                "model": model,
                "content": [],
                "stop_reason": None,
                "stop_sequence": None,
                "usage": {"input_tokens": 1, "output_tokens": 1},
            },
        },
    )
    yield sse(
        "content_block_start",
        {
            "type": "content_block_start",
            "index": 0,
            "content_block": {"type": "text", "text": ""},
        },
    )
    yield sse("ping", {"type": "ping"})
    yield sse(
        "content_block_delta",
        {
            "type": "content_block_delta",
            "index": 0,
            "delta": {"type": "text_delta", "text": text},
        },
    )
    yield sse("content_block_stop", {"type": "content_block_stop", "index": 0})
    yield sse(
        "message_delta",
        {
            "type": "message_delta",
            "delta": {"stop_reason": "end_turn", "stop_sequence": None},
            "usage": {"output_tokens": 1},
        },
    )
    yield sse("message_stop", {"type": "message_stop"})


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    # The harness's own stderr is the lane's window onto the harness. This
    # server's per-request chatter is not, and it would bury it.
    def log_message(self, format, *args):  # noqa: A002 - base class signature
        pass

    @property
    def stub(self) -> StubServer:  # the base class has not heard of its extras
        return self.server  # type: ignore[return-value]

    def read_body(self):
        length = int(self.headers.get("Content-Length") or 0)
        if not length:
            return {}
        raw = self.rfile.read(length)
        try:
            return json.loads(raw)
        except (ValueError, UnicodeDecodeError):
            return {"__unparsed__": raw.decode("utf-8", "replace")}

    def send_json(self, status, payload):
        raw = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def start_stream(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Transfer-Encoding", "chunked")
        self.end_headers()

    def chunk(self, raw):
        self.wfile.write(b"%x\r\n" % len(raw) + raw + b"\r\n")
        self.wfile.flush()

    def end_chunks(self):
        self.wfile.write(b"0\r\n\r\n")
        self.wfile.flush()

    def do_POST(self):  # noqa: N802 - base class name
        body = self.read_body()
        path = self.path.split("?", 1)[0]
        text = prompt_text(body)
        entry = {
            "phase": "request",
            "path": path,
            "method": "POST",
            "model": body.get("model"),
            "stream": bool(body.get("stream")),
            # WHOSE REQUEST THIS IS, decided here rather than by a test reading
            # the text back. The lane's assertions are only worth their words if
            # "the model saw it" means the agent's own turn saw it, and the
            # sentinel that separates a turn from a side errand lives here.
            "turn": self.stub.is_agent_turn(text),
            "text": text,
        }

        if path == "/v1/messages/count_tokens":
            self.stub.recorder.record(entry)
            self.send_json(200, {"input_tokens": max(1, len(text) // 4)})
            return

        if path != "/v1/messages":
            entry["unrecognised"] = True
            self.stub.recorder.record(entry)
            self.send_json(
                404,
                {
                    "type": "error",
                    "error": {
                        "type": "not_found_error",
                        "message": f"e2e stub does not implement {path}",
                    },
                },
            )
            return

        # A PLAUSIBLE ANSWER TO AN UNRECOGNISED REQUEST IS THE FAILURE MODE.
        # Answering 200 to a body this stub did not understand would let the
        # dialect drift under the lane while every scenario stayed green, so a
        # Messages request missing either field the dialect requires is
        # rejected and marked, and the lane treats a marked request as a fault.
        missing = [f for f in ("model", "messages") if not body.get(f)]
        if missing:
            entry["unrecognised"] = True
            self.stub.recorder.record(entry)
            self.send_json(
                400,
                {
                    "type": "error",
                    "error": {
                        "type": "invalid_request_error",
                        "message": f"e2e stub requires {', '.join(missing)}",
                    },
                },
            )
            return

        seq = self.stub.recorder.record(entry)

        if ERROR_MARKER in text:
            # THE ONE FATAL SHAPE THE CLAUDE-CODE COLLAR CLASSIFIES. A provider
            # that does not recognise the selected model answers 404
            # not_found_error, and the harness records that as a synthetic
            # assistant turn carrying error="model_not_found" — which is what
            # collar_bricked reads. Nothing here is retryable, so the lane
            # observes one failure rather than a retry ladder.
            self.send_json(
                404,
                {
                    "type": "error",
                    "error": {
                        "type": "not_found_error",
                        "message": f"model: {body.get('model')}",
                    },
                },
            )
            return

        if HOLD_MARKER in text and entry["turn"]:
            # The turn stays live on screen for exactly as long as the lane
            # needs it, and the lane is told the moment it becomes live.
            self.stub.hold(seq)

        # EVERY COMPLETION SAYS WHICH REQUEST IT ANSWERS. A pane check for a
        # bare prefix would be satisfied by any earlier answer still on the
        # screen, including the startup turn's; with the number in the text a
        # test can demand the answer to the turn it actually drove.
        model = body.get("model") or "claude-e2e-stub"
        if not body.get("stream"):
            self.send_json(
                200,
                {
                    "id": "msg_e2e_stub",
                    "type": "message",
                    "role": "assistant",
                    "model": model,
                    "content": [{"type": "text", "text": f"{REPLY_PREFIX} {seq}"}],
                    "stop_reason": "end_turn",
                    "stop_sequence": None,
                    "usage": {"input_tokens": 1, "output_tokens": 1},
                },
            )
            self.stub.recorder.record({"phase": "complete", "for": seq})
            return

        self.start_stream()
        for event in message_events(model, f"{REPLY_PREFIX} {seq}"):
            self.chunk(event)
        self.end_chunks()
        # RECORDED ONLY ONCE THE LAST BYTE IS AWAY. A request log records
        # arrival, and arrival is not delivery: a turn the harness is still
        # blocked on would otherwise satisfy every assertion made about it.
        # This line is what lets a test say the model answered.
        self.stub.recorder.record({"phase": "complete", "for": seq})

    def do_GET(self):  # noqa: N802 - base class name
        path = self.path.split("?", 1)[0]
        self.stub.recorder.record(
            {"phase": "request", "path": path, "method": "GET", "unrecognised": True}
        )
        self.send_json(
            404,
            {
                "type": "error",
                "error": {
                    "type": "not_found_error",
                    "message": f"e2e stub does not implement GET {path}",
                },
            },
        )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--log", required=True, help="request log, one JSON object per line")
    parser.add_argument("--port-file", required=True, help="written once the port is bound")
    parser.add_argument("--gate", help="FIFO that releases a held response")
    parser.add_argument("--held", help="FIFO signalled when a response starts holding")
    parser.add_argument(
        "--turn-sentinel",
        help="text only the agent's own turn carries; side errands never hold",
    )
    args = parser.parse_args()

    server = StubServer(
        ("127.0.0.1", 0),
        Handler,
        Recorder(args.log),
        args.gate,
        args.held,
        args.turn_sentinel,
    )

    port = server.server_address[1]
    with open(args.port_file + ".tmp", "w", encoding="utf-8") as handle:
        handle.write(str(port))
    os.replace(args.port_file + ".tmp", args.port_file)
    print(f"e2e stub listening on 127.0.0.1:{port}", file=sys.stderr, flush=True)

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
