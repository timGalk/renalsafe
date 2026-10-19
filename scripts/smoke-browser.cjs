// Optional browser smoke: reads local role keys without printing them.
const fs=require('node:fs');const path=require('node:path');const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const root=path.resolve(__dirname,'..');const tokens=JSON.parse(fs.readFileSync(path.join(root,'.local/tokens.json'),'utf8'));const output=path.join(root,'outputs');fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,...(process.env.RENALSAFE_CHROMIUM_PATH?{executablePath:process.env.RENALSAFE_CHROMIUM_PATH}:{})});
 try{
  const page=await browser.newPage({viewport:{width:1440,height:1000}});const errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(process.env.RENALSAFE_BASE_URL||'http://127.0.0.1:8080');
  async function login(role){await page.locator('#token').fill(tokens[role]);await page.locator('#login-form button').click();await page.locator('#workspace').waitFor({state:'visible'});await page.waitForFunction(()=>document.querySelector('#medications select')?.options.length>0);}
  await login('physician');await page.locator('#example').click();await page.locator('.dose').fill('10');await page.locator('.interval').fill('24');await page.locator('#assessment-form button[type="submit"],#assessment-form .submit-row button').click();
  await page.getByRole('heading',{name:'Оценка сохранена',exact:true}).waitFor();assert.match(await page.locator('#assessment-result').innerText(),/Действие и доза не назначены/);assert.match(await page.locator('#assessment-result').innerText(),/25 mL\/min/);
  await page.getByRole('button',{name:'Проверить воспроизводимость'}).click();await page.waitForFunction(()=>document.getElementById('message').textContent.includes('воспроизведён'));
  await page.screenshot({path:path.join(output,'assessment.png'),fullPage:true});
  await page.getByRole('button',{name:'Найти исходные фрагменты'}).click();await page.locator('#search-results article').first().waitFor();assert.match(await page.locator('#search-results').innerText(),/Персональная доза не сформирована/);assert.match(await page.locator('#search-results').innerText(),/полнотекстовый/);
  await page.screenshot({path:path.join(output,'evidence.png'),fullPage:true});assert.equal(await page.locator('#review-nav').isVisible(),false);
  await page.locator('#logout').click();await login('pharmacist');await page.locator('#review-nav').click();await page.locator('#rules-list article').first().waitFor();assert.match(await page.locator('#rules-list').innerText(),/PENDING/);assert.equal(await page.locator('#rules-list input[type="checkbox"]').first().isChecked(),false);
  await page.screenshot({path:path.join(output,'review.png'),fullPage:true});
  await page.setViewportSize({width:390,height:844});await page.locator('[data-view="assessment"]').click();assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth),false);await page.screenshot({path:path.join(output,'mobile.png'),fullPage:true});
  assert.deepEqual(errors,[]);assert.equal(await page.evaluate(()=>localStorage.length+sessionStorage.length),0);
  const summary={status:'passed',checks:['physician login','synthetic assessment and manual-review result','historical replay','evidence retrieval and explicit lexical fallback','review role visibility','pending rule not approved','mobile no horizontal overflow','no browser storage credentials','no JavaScript runtime errors']};
  fs.writeFileSync(path.join(output,'browser-smoke.json'),JSON.stringify(summary,null,2)+'\n');console.log(JSON.stringify(summary));
 }finally{await browser.close();}
})().catch(e=>{console.error(e.message);process.exitCode=1;});
