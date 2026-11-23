import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5173'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const failures = []
const wish = { id:'wish-1', title:'A very long wish '.repeat(6), priority:'medium', price:null, currency:'', shop_links:[], image_url:null, description:'' }
const list = { id:'list-1', title:'A very long list '.repeat(6), event_date:null, visibility:'link', wish_count:1 }
const labels = {
 en: ['Add a wish', 'Create a list'],
 ru: ['Добавить желание', 'Создать список'],
 ro: ['Adaugă o dorință', 'Creează o listă'],
}
try {
 for (const lang of ['en','ru','ro']) for (const width of [320,390,768,1024,1440]) for (const hasLists of [false,true]) for (const hasWishes of [false,true]) {
  const context = await browser.newContext({ locale:lang, viewport:{width,height:900} })
  const page = await context.newPage(); page.on('pageerror', e=>failures.push(e.message)); page.on('console', m=>{if(m.text().includes('[Vue warn]')) failures.push(m.text())})
  await page.route('**/api/auth/me', route=>route.fulfill({json:{user:{username:'a_very_long_username_123456789',email:'test@example.com'}}}))
  await page.route('**/api/lists', route=>route.fulfill({json:{lists:hasLists?[list,{...list,id:'list-2'}]:[]}}))
  await page.route('**/api/wishes', route=>route.fulfill({json:{wishes:hasWishes?[wish]:[]}}))
  await page.goto(`${origin}/my`); await page.locator(hasWishes ? '.wish-preview' : '.wishes-empty').waitFor(); await page.evaluate(()=>document.fonts.ready)
  await page.locator(hasLists ? '.list-card' : '.list-empty').first().waitFor()
  assert.deepEqual(await page.locator('.dashboard-section').evaluateAll(sections=>sections.map(section=>section.getAttribute('aria-labelledby'))),['dashboard-wishes-title','dashboard-lists-title'])
  assert.equal(await page.locator('.list-card').count(),hasLists?2:0)
  assert.equal(await page.locator('.list-empty').count(),hasLists?0:1)
  assert.equal(await page.locator('.create-list').count(),hasLists?1:0)
  const createList = page.locator('.dashboard a[href="/my/lists/new"]')
  assert.equal(await createList.count(), 1)
  assert.equal((await createList.innerText()).trim(), labels[lang][1])
  assert.equal(await createList.evaluate(link => link.closest('section').getAttribute('aria-labelledby')), 'dashboard-lists-title')
  assert.equal(await page.locator('.hero').count(),hasLists && hasWishes?0:1)
  const addWish = page.locator('.dashboard a[href="/my/wishes/new"]')
  assert.equal(await addWish.count(), 1)
  assert.equal((await addWish.innerText()).trim(), labels[lang][0])
  assert.equal(await addWish.evaluate(link => link.parentElement.matches('header.greeting')), true)
  if (width < 768) {
   const greeting = await page.locator('.greeting > div').boundingBox()
   const action = await addWish.boundingBox()
   assert.ok(action.y >= greeting.y + greeting.height, `${lang}/${width} add action below greeting`)
   assert.equal(action.width, greeting.width, `${lang}/${width} full-width add action`)
  }
  assert.equal(await page.locator('.wish-preview').count(),hasWishes?1:0)
  const state=await page.evaluate(()=>{const visible=e=>e.getClientRects().length&&getComputedStyle(e).visibility!=='hidden'; const width=document.documentElement.clientWidth;return {overflow:document.documentElement.scrollWidth>width, outside:[...document.querySelectorAll('main *, .app-header *, .bottom-navigation *')].filter(visible).filter(e=>{let r=e.getBoundingClientRect();return r.left< -1||r.right>width+1}).map(e=>e.className), small:[...document.querySelectorAll('a,button,select')].filter(visible).filter(e=>!e.classList.contains('skip-link')).filter(e=>{let r=e.getBoundingClientRect();return r.width<43.9||r.height<43.9}).map(e=>e.textContent)}})
  assert.equal(state.overflow,false,`${lang}/${width} overflow`); assert.deepEqual(state.outside,[],`${lang}/${width} outside`); assert.deepEqual(state.small,[],`${lang}/${width} small`)
  await addWish.click()
  await page.waitForURL('**/my/wishes/new')
  await page.locator('#wish-title').waitFor()
  await page.locator('.app-navigation:visible a[href="/my/wishes"]').click(); await page.locator(hasWishes ? '[data-wish-id="wish-1"]' : '.wishes-empty').waitFor(); assert.equal(new URL(page.url()).pathname,'/my/wishes'); assert.equal(await page.locator('.app-navigation:visible a[aria-current="page"]').getAttribute('href'),'/my/wishes')
  await context.close()
 }
 const context=await browser.newContext();const page=await context.newPage()
 await page.route('**/api/auth/me',route=>route.fulfill({json:{user:{username:'tester',email:'test@example.com'}}}))
 await page.route('**/api/lists',route=>route.fulfill({json:{lists:[]}})); await page.route('**/api/wishes',route=>route.fulfill({json:{wishes:[]}}))
 await page.goto(`${origin}/my`); await page.locator('.hero').waitFor(); await page.locator('.wishes-empty').waitFor(); await page.locator('.list-empty').waitFor()
 const dashboardEmptyText=await page.locator('.list-empty').innerText(); await page.goto(`${origin}/my/lists`); await page.locator('.list-empty').waitFor(); assert.equal(await page.locator('.list-empty').innerText(),dashboardEmptyText)
 await page.unroute('**/api/wishes'); await page.route('**/api/wishes',route=>route.fulfill({status:500,json:{error:'unavailable'}})); await page.goto(`${origin}/my`); await page.locator('.dashboard-error').waitFor(); await page.locator('.list-empty').waitFor()
 assert.equal(await page.locator('.hero').count(),0)
 await page.locator('.greeting a[href="/my/wishes/new"]').waitFor()
 await page.unroute('**/api/lists'); await page.route('**/api/lists',route=>route.fulfill({status:500,json:{error:'unavailable'}})); await page.reload(); await page.locator('.dashboard-error').nth(1).waitFor(); assert.equal(await page.locator('.hero').count(),0)
 await page.locator('.greeting a[href="/my/wishes/new"]').waitFor()
 await page.unroute('**/api/wishes'); await page.route('**/api/wishes',route=>route.fulfill({json:{wishes:[]}})); await page.reload(); await page.locator('.hero').waitFor()
 await page.locator('.greeting a[href="/my/wishes/new"]').waitFor()
 await context.close(); assert.deepEqual(failures,[]); console.log('Passed: one-click wish creation entry, EN/RU/RO x five widths, navigation, populated/empty/error states.')
} finally { await browser.close() }
