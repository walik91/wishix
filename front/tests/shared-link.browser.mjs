import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5173'
const texts = JSON.parse(await readFile(new URL('../src/locales/en.json', import.meta.url)))
const token = 'a'.repeat(43), nextToken = 'b'.repeat(43)
const owner = 'walik', id = 'example-list'
const list = { id, title:'Birthday', description:'', event_date:null, visibility:'link', wish_count:0, revision:1, share_path:`?token=${token}` }
const browser = await chromium.launch({ channel:'chrome', headless:true })
const context = await browser.newContext({locale:'en'})
const page = await context.newPage(), errors = []
const html = await (await context.request.get(origin)).text()
page.on('pageerror', error => errors.push(error.message))
page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
await page.route('**/wishlist/shared?*', async route => {
 const url = new URL(route.request().url())
 if (url.pathname.startsWith('/old_user/') && url.searchParams.get('token') === token) return route.fulfill({status:301,headers:{location:`/${owner}/wishlist/shared?token=${token}`}})
 return route.fulfill({status:200,contentType:'text/html',body:html})
})
await page.route('**/api/auth/me', route => route.fulfill({json:{user:{username:owner,email:'test@example.com'}}}))
await page.route(`**/api/lists/${id}`, route => route.fulfill({json:{list,wishes:[]}}))
await page.route(`**/api/public/profiles/${owner}`, route => route.fulfill({json:{username:owner,lists:[{...list,visibility:'public'}]}}))
await page.route('**/api/public/lists/view', async route => {
 const body = route.request().postDataJSON()
 return body.token === token ? route.fulfill({json:{username:owner,list:{...list,visibility:'link'},wishes:[]}}) : route.fulfill({status:404,json:{error:'list_not_found'}})
})
try {
 await page.goto(`${origin}/my/lists/${id}`)
 await page.getByRole('button',{name:texts['lists.share'],exact:true}).click()
 assert.equal(await page.locator('#list-share-url').inputValue(),`${origin}/${owner}/wishlist/shared?token=${token}`)
 await page.goto(`${origin}/u/${owner}`)
 await page.locator('.list-card-main').waitFor()
 assert.equal(await page.locator('.list-card-main').getAttribute('href'),`/${owner}/wishlist/shared?token=${token}`)
 await page.locator('.list-card-main').click()
 await page.getByRole('heading',{name:'Birthday',exact:true}).waitFor()
 assert.equal(new URL(page.url()).pathname,`/${owner}/wishlist/shared`)
 await page.goto(`${origin}/old_user/wishlist/shared?token=${token}`)
 await page.getByRole('heading',{name:'Birthday',exact:true}).waitFor()
 assert.equal(new URL(page.url()).pathname,`/${owner}/wishlist/shared`)
 assert.equal(new URL(page.url()).searchParams.get('token'),token)
 await page.goto(`${origin}/${owner}/wishlist/shared?token=${nextToken}`)
 await page.getByRole('heading',{name:texts['lists.sharedUnavailable'],exact:true}).waitFor()
 await page.goto(`${origin}/${owner}/wishlist/shared?token=invalid`)
 await page.getByRole('heading',{name:texts['lists.sharedUnavailable'],exact:true}).waitFor()
 assert.deepEqual(errors,[])
 console.log('Passed: sharing and public profile URLs, guest access, stale username redirect and invalid/revoked tokens.')
} finally { await context.close(); await browser.close() }
