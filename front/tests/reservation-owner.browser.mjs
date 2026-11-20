// Run against a local Vite server. All API calls are mocked.
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ headless: true, channel: 'chrome' })
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5175'
const id = '00000000-0000-4000-8000-000000000001'
const token = 'a'.repeat(43)
const html = await readFile(new URL('../index.html', import.meta.url), 'utf8')
const errors = []
let checks = 0
try {
  for (const lang of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const texts = JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))
    for (const viewer of ['owner', 'visitor', 'anonymous']) {
      const context = await browser.newContext({ locale: lang, viewport: { width, height: 900 } })
      const page = await context.newPage()
      page.on('pageerror', error => errors.push(error.message))
      page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
      await page.route('**/api/auth/me', route => viewer === 'anonymous' ? route.fulfill({ status: 401, json: { error: 'unauthorized' } }) : route.fulfill({ json: { user: { username: viewer, email: `${viewer}@example.test` } } }))
      await page.route('**/api/public/lists/guest', route => route.fulfill({ json: { is_owner: viewer === 'owner', username: 'owner', list: { id, title: 'Birthday', visibility: 'link', expired: false, event_date: null }, wishes: [
        { id: 'free', title: 'Free gift', description: '', priority: 'medium', price: null, currency: '', shop_links: [], has_image: false, reserved: false },
        { id: 'reserved', title: 'Reserved gift', description: '', priority: 'medium', price: null, currency: '', shop_links: [], has_image: false, reserved: true, guest_recoverable: true },
      ] } }))
      await page.route(`**/u/owner/wishlist/${id}/${token}`, route => route.fulfill({ contentType: 'text/html', body: html }))
      await page.goto(`${origin}/u/owner/wishlist/${id}/${token}`)
      await page.locator('.wish-card').nth(1).waitFor()
      assert.equal(await page.getByRole('button', { name: texts['reservation.reserve'], exact: true }).count(), viewer === 'owner' ? 0 : 1)
      assert.equal(await page.getByText(texts['reservation.reserved'], { exact: true }).count(), viewer === 'owner' ? 0 : 1)
      assert.equal(await page.getByRole('button', { name: texts['reservation.needLink'], exact: true }).count(), viewer === 'owner' ? 0 : 1)
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false)
      checks++
      await context.close()
    }
  }
  assert.deepEqual(errors, [])
  console.log(`Reservation visibility: ${checks} owner/visitor/anonymous scenarios passed`)
} finally { await browser.close() }
