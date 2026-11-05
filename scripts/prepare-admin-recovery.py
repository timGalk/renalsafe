#!/usr/bin/env python3
"""Prepare a new audited bootstrap identity; does not change running accounts or files."""
import argparse, hashlib, json, os, secrets
from pathlib import Path
import recovery as r

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--output', type=Path, required=True, help='New private directory under .local/')
a = p.parse_args()
path = a.output.resolve()
if not path.is_relative_to((r.ROOT / '.local').resolve()):
    p.error('Output must be under .local/')
os.umask(0o077)
path.mkdir(mode=0o700, parents=True, exist_ok=False)
records = json.loads((r.ROOT / '.local/users.json').read_text())
identity = 'operator-recovery-' + secrets.token_hex(8)
token = secrets.token_urlsafe(40)
records.append({'id': identity, 'role': 'admin', 'token_sha256': hashlib.sha256(token.encode()).hexdigest()})
r.private_json(path / 'users.json', records)
r.private_json(path / 'credential.json', {'actor_id': identity, 'admin': token})
print('Recovery identity prepared privately. Install the hashes file and restart only during operator recovery; credential file: ' + str(path / 'credential.json'))
