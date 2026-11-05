#!/usr/bin/env python3
"""Create a password-based staff account with a private activation file."""
import argparse,json,os
from pathlib import Path
from urllib.request import Request,build_opener,HTTPRedirectHandler
class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self,*args,**kwargs):raise ValueError('Redirects forbidden')
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('username');p.add_argument('--role',choices=['physician','pharmacist','admin'],default='admin');p.add_argument('--output',type=Path,required=True)
a=p.parse_args()
# Reserve output before creating anything. Never overwrite an earlier secret.
fd=os.open(a.output,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
root=Path(__file__).resolve().parents[1]
token=json.loads((root/'.local/tokens.json').read_text())['admin']
opener=build_opener(NoRedirect)
def call(path,body):
    req=Request('http://127.0.0.1:8080/api/v1'+path,data=json.dumps(body).encode(),headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'})
    with opener.open(req,timeout=30) as r:return json.load(r)
with os.fdopen(fd,'w') as out:
    user=call('/admin/users',dict(username=a.username,role=a.role,active=True))
    activation=call('/admin/users/'+user['id']+'/reset',{})
    json.dump(dict(username=a.username,**activation),out,indent=2)
print('Account created. Activation details saved privately to '+str(a.output))
