#!/usr/bin/env python3
"""Run all Go tests against a disposable isolated PostgreSQL+pgvector database."""
from pathlib import Path
import os,subprocess,secrets,time,json,sys
root=Path(__file__).resolve().parents[1]
if not (root/'core/internal/httpapi/web/dist/index.html').is_file():
    raise SystemExit('Build the frontend first: npm ci --prefix web && npm run build --prefix web')
name='renalsafe-tests-'+secrets.token_hex(5)
password=secrets.token_urlsafe(32)
image='pgvector/pgvector:pg18@sha256:2358fcba361ed2233a5ed81b5fe4ca779ccb304120ce531a3bf51c0ed7e2bc11'
def docker(*args,check=True):
    p=subprocess.run(['docker',*args],capture_output=True,text=True)
    if check and p.returncode:raise RuntimeError('Docker operation failed: '+p.stderr[:400])
    return p
try:
    docker('run','-d','--rm','--name',name,'-e','POSTGRES_USER=renalsafe','-e','POSTGRES_PASSWORD='+password,'-e','POSTGRES_DB=renalsafe_test','-p','127.0.0.1::5432',image)
    for _ in range(60):
        if docker('exec',name,'pg_isready','-U','renalsafe','-d','renalsafe_test',check=False).returncode==0:break
        time.sleep(.5)
    else:raise RuntimeError('Dedicated test database did not become ready')
    binding=json.loads(docker('inspect',name).stdout)[0]['NetworkSettings']['Ports']['5432/tcp'][0]
    port=binding['HostPort'];env=os.environ.copy()
    env['RENALSAFE_TEST_DATABASE_URL']='postgres://renalsafe:'+password+'@127.0.0.1:'+port+'/renalsafe_test?sslmode=disable'
    env.setdefault('GOMODCACHE','/tmp/renalsafe-gomod');env.setdefault('GOCACHE','/tmp/renalsafe-build-cache')
    result=subprocess.run(['go','test','-count=1','-race','-cover','./...'],cwd=root/'core',env=env)
    sys.exit(result.returncode)
finally:
    docker('rm','-f',name,check=False)
