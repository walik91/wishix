// Run against a local Vite server; all API calls are mocked.
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ headless: true, channel: 'chrome' })
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5174'
const id = '00000000-0000-4000-8000-000000000001'
const list = '00000000-0000-4000-8000-000000000002'
const wish = { id, title: 'Original wish', description: 'Notes', price: '12.50', currency: 'EUR', priority: 'high', shop_links: ['https://shop.test/first', 'https://shop.test/second'], edit_version: 'a'.repeat(64), image_url: null }
const errors = []
let checks = 0
try {
  for (const lang of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const texts = JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))
    const context = await browser.newContext({ locale: lang, viewport: { width, height: 900 } })
    const page = await context.newPage()
    let imports = 0, sent = null
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { id: 'owner', username: 'tester', email: 'tester@example.test' } } }))
    await page.route('**/api/wishes/import-preview', route => { imports++; return route.abort() })
    await page.route(`**/api/wishes/${id}`, route => {
      if (route.request().method() === 'PUT') { sent = route.request().postDataJSON(); return route.fulfill({ status: 409, json: { error: 'wish_conflict' } }) }
      return route.fulfill({ json: { wish } })
    })
    await page.goto(`${origin}/my/wishes/${id}/edit?list=${list}`)
    await page.locator('#wish-title').waitFor()
    assert.equal(await page.locator('h1').innerText(), texts['wishes.edit'])
    assert.equal(await page.locator('#wish-title').inputValue(), wish.title)
    assert.equal(await page.locator('#wish-price').inputValue(), wish.price)
    assert.equal(await page.locator('.link-row input').count(), 2)
    assert.equal(await page.locator('.back-link').getAttribute('href'), `/my/lists/${list}`)
    await page.locator('#wish-title').fill('Changed')
    await page.locator('.link-row input').first().fill('https://shop.test/changed')
    await page.locator('button[type=submit]').click()
    await page.getByRole('alert').filter({ hasText: texts['wishes.error.wish_conflict'] }).waitFor()
    assert.equal(sent.title, 'Changed')
    assert.equal(sent.edit_version, wish.edit_version)
    assert.deepEqual(sent.shop_links, ['https://shop.test/changed', 'https://shop.test/second'])
    assert.equal(await page.locator('#wish-title').inputValue(), 'Changed')
    assert.equal(imports, 0)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${lang} ${width}: horizontal overflow`)
    await page.getByRole('button', { name: texts['wishes.reloadEdit'] }).click()
    await page.locator('#wish-title').waitFor()
    assert.equal(await page.locator('#wish-title').inputValue(), 'Original wish')
    assert.equal(errors.length, 0, errors.join('\n'))
    checks++
    await context.close()
  }
  {
    const context = await browser.newContext({ locale: 'en' })
    const page = await context.newPage()
    let sent
    await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { id: 'owner', username: 'tester', email: 'tester@example.test' } } }))
    await page.route('**/api/wishes', route => route.fulfill({ json: { wishes: [{ ...wish, title: 'Saved wish' }] } }))
    await page.route(`**/api/wishes/${id}`, route => {
      if (route.request().method() === 'PUT') { sent = route.request().postDataJSON(); return route.fulfill({ json: { status: 'ok' } }) }
      return route.fulfill({ json: { wish } })
    })
    await page.goto(`${origin}/my/wishes/${id}/edit`)
    await page.locator('#wish-title').fill('Saved wish')
    await page.locator('button[type=submit]').click()
    await page.waitForURL('**/my/wishes*')
    await page.getByText('Wish updated.', { exact: true }).waitFor()
    assert.equal(sent.title, 'Saved wish')
    assert.equal(await page.locator('.wish-card h2').innerText(), 'Saved wish')
    await context.close()
    checks++
  }
  console.log(`Wish edit: ${checks} locale/width checks passed`)
} finally { await browser.close() }
