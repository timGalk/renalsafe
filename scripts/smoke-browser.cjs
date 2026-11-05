// Optional browser smoke: reads local role keys without printing them.
const fs=require('node:fs');const path=require('node:path');const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const root=path.resolve(__dirname,'..');const tokens=JSON.parse(fs.readFileSync(path.join(root,'.local/tokens.json'),'utf8'));const output=path.join(root,'outputs');fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,...(process.env.RENALSAFE_CHROMIUM_PATH?{executablePath:process.env.RENALSAFE_CHROMIUM_PATH}:{})});
 try{
  const page=await browser.newPage({viewport:{width:1440,height:1000}});const errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(process.env.RENALSAFE_BASE_URL||'http://127.0.0.1:8080');
  async function login(role){await page.locator('#login-form').evaluate(form=>{form.closest('details').open=true;});await page.locator('#token').fill(tokens[role]);await page.locator('#login-form button').click();await page.locator('#workspace').waitFor({state:'visible'});await page.waitForFunction(()=>document.querySelector('#medications select')?.options.length>0);}
  assert.equal(await page.locator('html').getAttribute('lang'),'en');await page.getByRole('heading',{name:'Open workspace',exact:true}).waitFor();
  await login('physician');await page.locator('#example').click();await page.locator('.dose').fill('10');await page.locator('.interval').fill('24');await page.locator('#assessment-form button[type="submit"],#assessment-form .submit-row button').click();
  await page.getByRole('heading',{name:'Assessment saved',exact:true}).waitFor();assert.match(await page.locator('#assessment-result').innerText(),/No action or dose has been recommended/);assert.match(await page.locator('#assessment-result').innerText(),/25 mL\/min/);
  await page.getByRole('button',{name:'Verify reproducibility'}).click();await page.waitForFunction(()=>document.getElementById('message').textContent.includes('reproduced'));
  await page.screenshot({path:path.join(output,'assessment.png'),fullPage:true});
  await page.getByRole('button',{name:'Find source passages'}).click();await page.locator('#search-results article').first().waitFor();assert.match(await page.locator('#search-results').innerText(),/No patient-specific dose has been generated/);assert.match(await page.locator('#search-results').innerText(),/full-text/);
  await page.screenshot({path:path.join(output,'evidence.png'),fullPage:true});assert.equal(await page.locator('#review-nav').isVisible(),false);
  await page.locator('#logout').click();await login('pharmacist');await page.locator('#review-nav').click();await page.locator('#rules-list article').first().waitFor();assert.match(await page.locator('#rules-list').innerText(),/PENDING/);assert.equal(await page.locator('#rules-list input[type="checkbox"]').first().isChecked(),false);
  await page.screenshot({path:path.join(output,'review.png'),fullPage:true});
  await page.locator('#documents-nav').click();await page.getByRole('heading',{name:'SmPC / ChPL documents',exact:true}).waitFor();await page.getByRole('button',{name:'Upload and extract sections',exact:true}).waitFor();await page.screenshot({path:path.join(output,'documents.png'),fullPage:true});
  await page.setViewportSize({width:390,height:844});await page.locator('[data-view="assessment"]').click();assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth),false);await page.screenshot({path:path.join(output,'mobile.png'),fullPage:true});
  assert.deepEqual(errors,[]);assert.equal(await page.evaluate(()=>localStorage.length+sessionStorage.length),0);
  const summary={status:'passed',checks:['English interface and document workflow','physician login','synthetic assessment and manual-review result','historical replay','evidence retrieval and explicit lexical fallback','review role visibility','pending rule not approved','mobile no horizontal overflow','no browser storage credentials','no JavaScript runtime errors']};
  fs.writeFileSync(path.join(output,'browser-smoke.json'),JSON.stringify(summary,null,2)+'\n');console.log(JSON.stringify(summary));
 }finally{await browser.close();}
})().catch(e=>{console.error("Browser workflow check failed; credentials omitted from diagnostic output.");process.exitCode=1;});
