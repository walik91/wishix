const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5175'
const browser = await chromium.launch({channel:'chrome',headless:true})
try {
 for (const lang of ['en','ru','ro']) {
  const context = await browser.newContext({locale:lang, viewport:{width:390,height:900}})
  const page=await context.newPage()
  const texts=JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))
  for (const key of ['register.username', 'register.usernameHint', 'register.profileUrl', 'register.suggestions']) assert.ok(!texts[key].includes('?'), `${lang}: ${key}`)
  await page.route('**/api/auth/google/status', r=>r.fulfill({json:{enabled:false}}))
  await page.route('**/api/auth/facebook/status', r=>r.fulfill({json:{enabled:false}}))
  await page.route('**/api/auth/username-suggestions', async r=>{const {username}=r.request().postDataJSON(); await r.fulfill({json:{suggestions:[username+'1',username+'2',username+'3',username+'4']}})})
  await page.route('**/api/auth/register',r=>r.fulfill({status:409,json:{error:'registration_unavailable'}}))
  await page.goto(origin + '/register')
  assert.equal(await page.locator('label[for="username"]').innerText(), texts['register.username'])
  await page.locator('#email').fill('anya@example.com')
  await page.getByRole('button',{name:'anya2',exact:true}).waitFor()
  assert.equal(await page.locator('#username').inputValue(),'anya1')
  await page.getByRole('button',{name:'anya2',exact:true}).click()
  assert.equal(await page.locator('#username').inputValue(),'anya2')
  assert.ok((await page.locator('.profile-url').innerText()).includes('/u/anya2'))
  await page.locator('#username').fill('myname')
  await page.locator('#email').fill('another@example.com')
  await page.getByRole('button',{name:'another1',exact:true}).waitFor()
  assert.equal(await page.locator('#username').inputValue(),'myname')
  for (const width of [320,390,768,1024,1440]) {
   await page.setViewportSize({width,height:900})
   assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,`${lang} ${width}`)
   for (const button of await page.locator('.suggestion-list button').all()) { const rect=await button.boundingBox(); assert.ok(rect.width>=44&&rect.height>=44) }
  }
  await page.locator('#username').fill('аб')
  await page.locator('#password').fill('abcd')
  await page.locator('.register-form button[type=submit]').click()
  assert.equal(await page.locator('#username').getAttribute('aria-invalid'),'true')
  await page.locator('#username').fill('validname')
  assert.equal(await page.locator('#username').getAttribute('aria-invalid'),null)
  await page.locator('.register-form button[type=submit]').click()
  await page.locator('.auth-error').waitFor()
  await page.locator('#username').fill('validname2')
  assert.equal(await page.locator('.auth-error').count(),0)
  await page.goto(origin + '/google/finish')
  await page.route('**/api/auth/google/username-suggestions', r => r.fulfill({json:{suggestions:['google_user','google_user1','google_user2','google_user3']}}))
  await page.reload()
  await page.getByRole('button',{name:'google_user1',exact:true}).waitFor()
  assert.equal(await page.locator('#username').inputValue(),'google_user')
  await page.getByRole('button',{name:'google_user1',exact:true}).click()
  assert.equal(await page.locator('#username').inputValue(),'google_user1')
  assert.ok((await page.locator('.profile-url').innerText()).includes('/u/google_user1'))
  for (const width of [320,390,768,1024,1440]) {
   await page.setViewportSize({width,height:900})
   assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,`Google ${lang} ${width}`)
   for (const button of await page.locator('.suggestion-list button').all()) { const rect=await button.boundingBox(); assert.ok(rect.width>=44&&rect.height>=44) }
  }
  await page.locator('#username').fill('')
  await page.locator('.finish-form button[type=submit]').click()
  assert.equal(await page.locator('#username').getAttribute('aria-invalid'),'true')
  await page.locator('#username').fill('validname')
  assert.equal(await page.locator('#username').getAttribute('aria-invalid'),null)
  await page.unroute('**/api/auth/google/username-suggestions')
  let release
  const responseReady = new Promise(resolve => { release = resolve })
  await page.route('**/api/auth/google/username-suggestions', async r => { await responseReady; await r.fulfill({json:{suggestions:['late_user']}}) })
  await page.reload()
  await page.locator('#username').fill('my_manual_name')
  release()
  await page.getByRole('button',{name:'late_user',exact:true}).waitFor()
  assert.equal(await page.locator('#username').inputValue(),'my_manual_name')
  await context.close()
 }
 console.log('Registration and Google finish: suggestions, error recovery, 3 languages and 5 widths passed')
} finally {await browser.close()}

