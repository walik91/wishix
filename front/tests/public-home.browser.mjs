import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { join } from 'node:path'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5193'
const screenshotDir = process.env.WISHIX_PUBLIC_HOME_SCREENSHOTS
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

async function checkExample(page, texts, label) {
  await page.goto(origin + '/wishlist/example')
  await page.getByRole('heading', { name: texts['exampleWishlist.title'], exact: true }).waitFor()
  const cards = page.locator('.shared-wish-grid .wish-card')
  assert.equal(await cards.count(), 2)
  assert.equal(await cards.getByRole('heading', { name: texts['exampleWishlist.headphones'], exact: true }).count(), 1)
  assert.equal(await cards.getByRole('button', { name: texts['reservation.reserve'], exact: true }).isDisabled(), true)
  assert.equal(await cards.getByText(texts['reservation.reserved'], { exact: true }).count(), 1)
  const images = cards.getByRole('img')
  assert.equal(await images.count(), 2)
  await images.last().scrollIntoViewIfNeeded()
  await page.waitForFunction(() => [...document.querySelectorAll('.shared-wish-grid img')].every(image => image.naturalWidth > 0))
  await checkOverflow(page, 'Example ' + label)
}

try {
  for (const locale of ['en', 'ru', 'ro']) {
    const texts = JSON.parse(await readFile(new URL(`../src/locales/${locale}.json`, import.meta.url)))
    for (const viewport of viewports) {
      const context = await browser.newContext({ locale, viewport })
      await context.addInitScript(() => localStorage.setItem('wishix_analytics_consent_v1', 'denied'))
      const page = await context.newPage()
      const errors = [], unexpectedRequests = []
      let signedIn = false
      page.on('pageerror', error => errors.push(error.message))
      page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
      await page.route(origin + '/api/**', route => {
        const path = new URL(route.request().url()).pathname
        if (path === '/api/auth/me') {
          return signedIn
            ? route.fulfill({ json: { user: { username: 'anna', email: 'test@example.com' } } })
            : route.fulfill({ status: 401, json: { error: 'unauthorized' } })
        }
        if (path === '/api/wishes' && signedIn) return route.fulfill({ json: { wishes: [] } })
        if (path === '/api/auth/google/status' || path === '/api/auth/facebook/status') return route.fulfill({ json: { enabled: false } })
        unexpectedRequests.push(path)
        return route.fulfill({ status: 404, json: { error: 'not_found' } })
      })
      const label = `${locale}/${viewport.width}`
      for (const state of ['guest', 'signed-in']) {
        signedIn = state === 'signed-in'
        const href = signedIn ? '/my/wishes' : '/register'
        const actionLabel = texts[signedIn ? 'home.openMyWishes' : 'home.addWishes']
        await page.goto(origin + '/')
        const main = page.getByRole('main')
        const action = main.locator('.hero').getByRole('link', { name: actionLabel, exact: true })
        await action.waitFor()
        assert.equal(await action.getAttribute('href'), href)
        assert.equal(await main.locator('a, button').count(), 1)
        assert.equal(await main.locator('.wish-card').count(), 0)
        assert.equal(await page.locator('a[href="/wishlist/example"]').count(), 0)
        await page.waitForFunction(() => document.querySelector('.hero .gift')?.naturalWidth > 0)
        const { width, height } = await action.boundingBox()
        assert.ok(width >= 44 && height >= 44, 'hero action meets the minimum touch size')
        await checkOverflow(page, `Home ${label}/${state}`)
        if (screenshotDir && locale === 'ru' && [390, 1440].includes(viewport.width)) {
          await page.screenshot({ path: join(screenshotDir, `home-${state}-${viewport.width}.png`) })
        }
        if (viewport.width === 390) {
          await action.focus()
          assert.equal(await action.evaluate(link => link.matches(':focus-visible')), true)
          await action.press('Enter')
        } else {
          await action.click()
        }
        await page.waitForURL(origin + href)
        if (signedIn) await page.locator('.collection-heading h1').waitFor()
        else await page.getByRole('heading', { name: texts['register.title'], exact: true }).waitFor()
      }
      signedIn = false
      await checkExample(page, texts, label)
      assert.deepEqual(unexpectedRequests, [])
      assert.deepEqual(errors, [])
      await context.close()
    }
  }
  console.log('Passed: guest and signed-in home actions and direct example wishlist across EN/RU/RO and five sizes.')
} finally { await browser.close() }
