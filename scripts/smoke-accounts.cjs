// Exercise a disposable staff account; never print passwords, keys or activation secrets.
const fs=require('node:fs');const path=require('node:path');const assert=require('node:assert/strict');const crypto=require('node:crypto');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const root=path.resolve(__dirname,'..');const key=JSON.parse(fs.readFileSync(path.join(root,'.local/tokens.json'),'utf8')).admin;
 const base='http://127.0.0.1:8080';
 async function admin(route,body){const res=await fetch(base+'/api/v1'+route,{method:body?'POST':'GET',headers:{Authorization:'Bearer '+key,'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});assert.ok(res.ok,'administrator operation failed');return res.json();}
 const username='smoke-'+crypto.randomBytes(5).toString('hex');const password=crypto.randomBytes(24).toString('base64url');
 const user=await admin('/admin/users',{username,role:'physician',active:true});const reset=await admin('/admin/users/'+user.id+'/reset',{});
 const browser=await chromium.launch({headless:true,...(process.env.RENALSAFE_CHROMIUM_PATH?{executablePath:process.env.RENALSAFE_CHROMIUM_PATH}:{})});
 try{
 const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(base);
 await page.getByText('Activate or reset an account',{exact:true}).click();await page.locator('#activate-user').fill(username);await page.locator('#activate-secret').fill(reset.activation_secret);await page.locator('#activate-password').fill(password);await page.locator('#activate-form button').click();await page.waitForFunction(()=>document.getElementById('message').textContent.includes('Password set'));
 await page.locator('#username').fill(username);await page.locator('#password').fill(password);await page.locator('#account-login button').click();await page.locator('#workspace').waitFor({state:'visible'});await page.waitForFunction(()=>document.getElementById('identity').textContent.includes('physician'));assert.equal(await page.locator('#admin-nav').isVisible(),false);
 await page.reload();await page.locator('#workspace').waitFor({state:'visible'});assert.equal(await page.evaluate(()=>localStorage.length+sessionStorage.length),0);
 await page.locator('#logout').click();await page.locator('#login').waitFor({state:'visible'});await page.reload();await page.locator('#login').waitFor({state:'visible'});assert.equal(await page.locator('#workspace').isVisible(),false);assert.deepEqual(errors,[]);
 console.log(JSON.stringify({status:'passed',checks:['single-use activation UI','password sign-in','physician role visibility','session restored after reload','server logout survives reload','no browser storage credentials','no JavaScript errors']}));
 }finally{await browser.close();await fetch(base+'/api/v1/admin/users/'+user.id,{method:'PATCH',headers:{Authorization:'Bearer '+key,'Content-Type':'application/json'},body:JSON.stringify({username,role:'physician',active:false})});}
})().catch(e=>{console.error("Account browser check failed; credentials omitted from diagnostic output.");process.exitCode=1;});
