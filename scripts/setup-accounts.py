#!/usr/bin/env python3
"""Create a staff account or explicitly renew activation using local administrator credentials."""
import argparse,json,os
from pathlib import Path
from urllib.request import Request,build_opener,HTTPRedirectHandler
class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self,*args,**kwargs):raise ValueError('Redirects forbidden')
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('username');p.add_argument('--role',choices=['physician','pharmacist','admin'],default='admin');p.add_argument('--reset-existing',action='store_true',help='Explicitly revoke credentials and renew activation for the named existing account')
p.add_argument('--output',type=Path,required=True);a=p.parse_args()
root=Path(__file__).resolve().parents[1]
key=json.loads((root/'.local/tokens.json').read_text())['admin'];opener=build_opener(NoRedirect)
def call(path,body=None):
    req=Request('http://127.0.0.1:8080/api/v1'+path,data=json.dumps(body).encode() if body is not None else None,headers={'Authorization':'Bearer '+key,'Content-Type':'application/json'})
    with opener.open(req,timeout=30) as r:return json.load(r)
# Reserve a private output file before modifying accounts; never overwrite secrets.
fd=os.open(a.output,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
try:
    with os.fdopen(fd,'w') as out:
        if a.reset_existing:
            matches=[u for u in call('/admin/users') if u['username']==a.username]
            if len(matches)!=1:raise ValueError('The named account was not found')
            user=matches[0]
        else:user=call('/admin/users',dict(username=a.username,role=a.role,active=True))
        activation=call('/admin/users/'+user['id']+'/reset',{})
        json.dump(dict(username=a.username,**activation),out,indent=2)
except Exception:
    a.output.unlink(missing_ok=True)
    raise
print('Activation details saved privately to '+str(a.output))
