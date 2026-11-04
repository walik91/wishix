import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5173'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const failures = []
const wish = { id:'wish-1', title:'A very long wish '.repeat(6), priority:'medium', price:null, currency:'', shop_links:[], image_url:null, description:'' }
const list = { id:'list-1', title:'A very long list '.repeat(6), event_date:null, visibility:'link', wish_count:1 }
try {
 for (const lang of ['en','ru','ro']) for (const width of [320,390,768,1024,1440]) for (const hasLists of [false,true]) {
  const context = await browser.newContext({ locale:lang, viewport:{width,height:900} })
  const page = await context.newPage(); page.on('pageerror', e=>failures.push(e.message)); page.on('console', m=>{if(m.text().includes('[Vue warn]')) failures.push(m.text())})
  await page.route('**/api/auth/me', route=>route.fulfill({json:{user:{username:'a_very_long_username_123456789',email:'test@example.com'}}}))
  await page.route('**/api/lists', route=>route.fulfill({json:{lists:hasLists?[list,{...list,id:'list-2'}]:[]}}))
  await page.route('**/api/wishes', route=>route.fulfill({json:{wishes:[wish]}}))
  await page.goto(`${origin}/my`); await page.locator('.wish-preview').waitFor(); await page.evaluate(()=>document.fonts.ready)
  await page.locator(hasLists ? '.list-card' : '.hero').first().waitFor()
  assert.equal(await page.locator('.list-card').count(),hasLists?2:0)
  assert.equal(await page.locator('.hero').count(),hasLists?0:1)
  assert.equal(await page.locator('.wish-preview').count(),1)
  const state=await page.evaluate(()=>{const visible=e=>e.getClientRects().length&&getComputedStyle(e).visibility!=='hidden'; const width=document.documentElement.clientWidth;return {overflow:document.documentElement.scrollWidth>width, outside:[...document.querySelectorAll('main *, .app-header *, .bottom-navigation *')].filter(visible).filter(e=>{let r=e.getBoundingClientRect();return r.left< -1||r.right>width+1}).map(e=>e.className), small:[...document.querySelectorAll('a,button,select')].filter(visible).filter(e=>!e.classList.contains('skip-link')).filter(e=>{let r=e.getBoundingClientRect();return r.width<43.9||r.height<43.9}).map(e=>e.textContent)}})
  assert.equal(state.overflow,false,`${lang}/${width} overflow`); assert.deepEqual(state.outside,[],`${lang}/${width} outside`); assert.deepEqual(state.small,[],`${lang}/${width} small`)
  await page.locator('.app-navigation:visible a[href="/my/wishes"]').click(); await page.locator('[data-wish-id="wish-1"]').waitFor(); assert.equal(new URL(page.url()).pathname,'/my/wishes'); assert.equal(await page.locator('.app-navigation:visible a[aria-current="page"]').getAttribute('href'),'/my/wishes')
  await context.close()
 }
 const context=await browser.newContext();const page=await context.newPage()
 await page.route('**/api/auth/me',route=>route.fulfill({json:{user:{username:'tester',email:'test@example.com'}}}))
 await page.route('**/api/lists',route=>route.fulfill({json:{lists:[]}})); await page.route('**/api/wishes',route=>route.fulfill({json:{wishes:[]}}))
 await page.goto(`${origin}/my`); await page.locator('.hero').waitFor(); await page.locator('.wishes-empty').waitFor(); assert.equal(await page.locator('.create-list').count(),1)
 await page.unroute('**/api/wishes'); await page.route('**/api/wishes',route=>route.fulfill({status:500,json:{error:'unavailable'}})); await page.reload(); await page.locator('.dashboard-error').waitFor(); assert.equal(await page.locator('.create-list').count(),1)
 assert.equal(await page.locator('.hero').count(),1)
 await page.unroute('**/api/lists'); await page.route('**/api/lists',route=>route.fulfill({status:500,json:{error:'unavailable'}})); await page.reload(); await page.locator('.dashboard-error').first().waitFor(); assert.equal(await page.locator('.hero').count(),0)
 await context.close(); assert.deepEqual(failures,[]); console.log('Passed: EN/RU/RO x five widths, navigation, populated/empty/error states.')
} finally { await browser.close() }
