import {mkdtempSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';import {join} from 'node:path';import {spawnSync} from 'node:child_process';
const dir=mkdtempSync(join(tmpdir(),'renalsafe-types-'));
try{const output=join(dir,'api.ts');const run=spawnSync(process.execPath,['node_modules/openapi-typescript/bin/cli.js','../api/openapi.json','-o',output],{stdio:'inherit'});if(run.status!==0)process.exitCode=1;else if(readFileSync(output,'utf8')!==readFileSync('src/api.generated.ts','utf8')){console.error('API types are stale. Run npm run types.');process.exitCode=1;}}finally{rmSync(dir,{recursive:true,force:true});}
