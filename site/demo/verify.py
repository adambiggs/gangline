#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Require native delivery receipts for the recorded handoff and reply."""
import json
from pathlib import Path
import sys

log, output = map(Path, sys.argv[1:])
events = [json.loads(line) for line in log.read_text().splitlines()]
messages = [e['envelope'] for e in events if e['type'] == 'send_queued'
            and e['envelope']['id'].startswith('msg-')]
delivered = {(e['id'], e['hitch_id']) for e in events
             if e['type'] == 'delivery_succeeded'}
expected = [('guide', 'builder'), ('builder', 'guide')]
actual = [(m['from']['name'], m['to']) for m in messages]
if actual != expected:
    raise SystemExit(f'Refusing incomplete or unexpected exchange: {actual}')
for message in messages:
    if (message['from']['kind'] != 'agent' or not message['from'].get('hitch_id')
            or (message['id'], message['recipient']) not in delivered):
        raise SystemExit(f"Unverified message: {message['id']}")
if 'Hello, team!' not in messages[-1]['message']['text']:
    raise SystemExit('Reply does not contain the observed program output.')
text = ['Claude Code (guide) and Codex (builder)\n']
for message in messages:
    sender = message['from']['name']
    marker = f"gang:{sender}#{message['id']}"
    text += [f'[{marker}]', message['message']['text'], f'[/{marker}]', '']
output.write_text('\n'.join(text))
print('PASS: registered guide → builder → guide; both native delivery receipts present.')
