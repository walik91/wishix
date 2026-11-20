// Run against a local Vite server. All API calls are mocked.
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ headless: true, channel: 'chrome' })
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5175'
const listID = '00000000-0000-4000-8000-000000000001'
const longDescription = 'A long description with gift details. '.repeat(14)
const wishes = [
  { id: 'one', title: 'Short title', description: 'Short description', price: null, currency: '', priority: 'medium', shop_links: [], has_image: false, image_url: null },
  { id: 'two', title: 'A much longer title '.repeat(6), description: longDescription, price: '12.00', currency: 'EUR', priority: 'high', shop_links: ['https://example.com/product'], has_image: false, image_url: null },
  { id: 'three', title: 'Another wish', description: '', price: null, currency: '', priority: 'low', shop_links: [], has_image: false, image_url: null },
]
const list = { id: listID, title: 'Birthday', description: '', event_date: null, visibility: 'public', revision: 1, share_path: `/${listID}`, wish_count: 3 }
const indexHTML = await readFile(new URL('../index.html', import.meta.url), 'utf8')
const errors = []
let checks = 0
try {
  for (const lang of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const context = await browser.newContext({ locale: lang, viewport: { width, height: 900 } })
    const page = await context.newPage()
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
    await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { username: 'example', email: 'example@example.test' } } }))
    await page.route('**/api/wishes', route => route.fulfill({ json: { wishes } }))
    await page.route(`**/api/lists/${listID}`, route => route.fulfill({ json: { list, wishes } }))
    await page.route('**/api/public/lists/guest', route => route.fulfill({ json: { list, username: 'example', wishes } }))
    await page.route('**/api/public/profiles/example/wishes', route => route.fulfill({ json: { username: 'example', wishes } }))
    await page.route(`**/u/example/wishlist/${listID}`, route => route.fulfill({ contentType: 'text/html', body: indexHTML }))
    for (const path of ['/my/wishes', `/my/lists/${listID}`, `/u/example/wishlist/${listID}`, '/u/example/wishes']) {
      await page.goto(origin + path)
      await page.locator('.wish-card').nth(2).waitFor()
      const heights = await page.locator('.wish-card').evaluateAll(cards => cards.map(card => card.getBoundingClientRect().height))
      assert.ok(Math.max(...heights) - Math.min(...heights) < 1, `${lang} ${width} ${path}: ${heights}`)
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${lang} ${width} ${path}: overflow`)
      const card = page.locator('.wish-card').nth(1)
      const before = heights[1]
      await card.locator('.description-toggle').click()
      const dialog = page.locator('dialog[open]')
      assert.equal(await dialog.locator('.full-description').innerText(), longDescription)
      assert.ok(Math.abs((await card.boundingBox()).height - before) < 1, 'dialog must not resize the card')
      await page.keyboard.press('Escape')
      await dialog.waitFor({ state: 'hidden' })
      if (path.includes('/wishlist/')) assert.equal(await page.getByRole('button', { name: /reserve|забронировать|rezerv/i }).count(), 3)
      checks++
    }
    await context.close()
  }
  assert.deepEqual(errors, [])
  console.log(`Wish card layout: ${checks} scenarios passed`)
} finally { await browser.close() }
