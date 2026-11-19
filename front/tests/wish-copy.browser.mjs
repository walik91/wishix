// Run against an isolated local Vite server; API responses are mocked.
import assert from 'node:assert/strict'
import { readFile, mkdir } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5174'
const sourceID = '00000000-0000-4000-8000-000000000001'
const copyID = '00000000-0000-4000-8000-000000000002'
const listID = '00000000-0000-4000-8000-000000000003'
const wish = { id: sourceID, title: 'A long wish title with a useful description', description: 'A gift for another occasion', price: '12.50', currency: 'EUR', priority: 'high', shop_links: ['https://shop.test/item'], image_url: null }
const list = { id: listID, title: 'Birthday', description: '', event_date: null, visibility: 'public', revision: 1, share_path: `/${listID}`, wish_count: 1 }
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const errors = []
await mkdir(new URL('../../.cache/wish-copy-screens/', import.meta.url), { recursive: true })
try {
  for (const lang of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const texts = JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))
    const context = await browser.newContext({ locale: lang, viewport: { width, height: 900 } })
    const page = await context.newPage()
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    let fail = false, requests = 0
    await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { username: 'example', email: 'example@example.test' } } }))
    await page.route('**/api/wishes', route => route.fulfill({ json: { wishes: [wish] } }))
    await page.route(`**/api/lists/${listID}`, route => route.fulfill({ json: { list, wishes: [wish] } }))
    await page.route(`**/api/wishes/${sourceID}/copy`, route => {
      requests++
      assert.equal(route.request().method(), 'POST')
      assert.deepEqual(route.request().postDataJSON(), {})
      return route.fulfill(fail ? { status: 503, json: { error: 'unavailable' } } : { status: 201, json: { wish: { ...wish, id: copyID } } })
    })
    await page.goto(`${origin}/my/wishes`)
    await page.getByRole('button', { name: texts['wishes.copy'], exact: true }).click()
    await page.getByText(texts['wishes.copied'], { exact: true }).waitFor()
    assert.equal(await page.locator('.wish-card').count(), 2)
    assert.equal(await page.locator('.wish-card').first().getAttribute('data-wish-id'), copyID)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/collection`)
    if (lang === 'ru' && [390, 1440].includes(width)) await page.screenshot({ path: fileURLToPath(new URL(`../../.cache/wish-copy-screens/collection-${width}.png`, import.meta.url)), fullPage: true })
    await page.goto(`${origin}/my/lists/${listID}`)
    await page.getByRole('button', { name: texts['wishes.copy'], exact: true }).click()
    await page.getByText(texts['lists.notice.wishCopied'], { exact: true }).waitFor()
    assert.equal(await page.locator('.wish-card').count(), 1)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/list`)
    fail = true
    await page.getByRole('button', { name: texts['wishes.copy'], exact: true }).click()
    await page.getByRole('alert').filter({ hasText: texts['wishes.copyError'] }).waitFor()
    assert.equal(requests, 3)
    await context.close()
  }
  assert.deepEqual(errors, [])
  console.log('Wish copy: EN/RU/RO × 320/390/768/1024/1440 passed.')
} finally { await browser.close() }
