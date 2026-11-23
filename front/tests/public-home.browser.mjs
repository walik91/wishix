import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5193'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const viewports = [
  { width: 320, height: 640 },
  { width: 390, height: 844 },
  { width: 768, height: 1024 },
  { width: 1024, height: 768 },
  { width: 1440, height: 900 },
]

async function checkOverflow(page, label) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
  assert.equal(overflow, false, label)
}

try {
  for (const locale of ['en', 'ru', 'ro']) {
    const texts = JSON.parse(await readFile(new URL(`../src/locales/${locale}.json`, import.meta.url)))
    for (const viewport of viewports) {
      const context = await browser.newContext({ locale, viewport })
      const page = await context.newPage()
      const errors = [], dataRequests = []
      page.on('pageerror', error => errors.push(error.message))
      page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
      await page.route('**/www.googletagmanager.com/**', route => route.fulfill({ contentType: 'text/javascript', body: '' }))
      await page.route(origin + '/api/**', route => {
        const path = new URL(route.request().url()).pathname
        if (path === '/api/auth/me') return route.fulfill({ status: 401, json: { error: 'unauthorized' } })
        dataRequests.push(path)
        return route.fulfill({ status: 404, json: { error: 'not_found' } })
      })
      await page.goto(origin + '/')
      const hero = page.locator('.hero')
      const signUp = hero.locator('a[href="/register"]')
      const example = hero.getByRole('link', { name: texts['home.viewExample'], exact: true })
      await signUp.waitFor()
      assert.equal(await signUp.innerText(), texts['home.start'])
      assert.equal(await hero.getByText(texts['home.guestHint'], { exact: true }).count(), 1)
      for (const link of [signUp, example]) {
        const { width, height } = await link.boundingBox()
        assert.ok(width >= 44 && height >= 44, 'hero actions meet the minimum touch size')
      }
      await checkOverflow(page, `Home ${locale}/${viewport.width}`)
      await example.click()
      assert.equal(new URL(page.url()).pathname, '/wishlist/example')
      await page.getByRole('heading', { name: texts['exampleWishlist.title'], exact: true }).waitFor()
      const cards = page.locator('.shared-wish-grid .wish-card')
      assert.equal(await cards.count(), 2)
      assert.equal(await cards.getByRole('button', { name: texts['reservation.reserve'], exact: true }).isDisabled(), true)
      assert.equal(await cards.getByText(texts['reservation.reserved'], { exact: true }).count(), 1)
      const images = cards.getByRole('img')
      assert.equal(await images.count(), 2)
      await images.last().scrollIntoViewIfNeeded()
      await page.waitForFunction(() => [...document.querySelectorAll('.shared-wish-grid img')].every(image => image.naturalWidth > 0))
      await checkOverflow(page, `Example ${locale}/${viewport.width}`)
      assert.deepEqual(dataRequests, [], 'example must not call list or reservation APIs')
      assert.deepEqual(errors, [])
      await context.close()
    }
  }
  console.log('Passed: guest home and example wishlist across EN/RU/RO and five sizes.')
} finally { await browser.close() }
