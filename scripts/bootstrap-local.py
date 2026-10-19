#!/usr/bin/env python3
"""Provision local demo credentials without printing secrets or overwriting keys."""
from pathlib import Path
import hashlib,json,secrets,os
root=Path(__file__).resolve().parents[1]
local=root/'.local';local.mkdir(mode=0o700,exist_ok=True)
users=local/'users.json';tokens=local/'tokens.json';env=root/'.env'
if users.exists() or tokens.exists() or env.exists():
    raise SystemExit('Local configuration already exists; refusing to overwrite credentials.')
records=[];plain={}
for role in ['physician','pharmacist','admin']:
    token=secrets.token_urlsafe(40);plain[role]=token
    records.append({'id':'local-'+role,'role':role,'token_sha256':hashlib.sha256(token.encode()).hexdigest()})
password=secrets.token_urlsafe(32)
for path,content in [(users,json.dumps(records,indent=2)+'\n'),(tokens,json.dumps(plain,indent=2)+'\n'),(env,'RENALSAFE_DB_PASSWORD='+password+'\nRENALSAFE_DB_PORT=55432\nRENALSAFE_DATABASE_URL=postgres://renalsafe:'+password+'@127.0.0.1:55432/renalsafe?sslmode=disable\n')]:
    descriptor=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o644 if path==users else 0o600)
    with os.fdopen(descriptor,'w') as stream:stream.write(content)
print('Created .env and .local/users.json (hashes), .local/tokens.json (private role keys).')
