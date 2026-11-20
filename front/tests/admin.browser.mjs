import assert from 'node:assert/strict'
import { chromium } from 'playwright'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
try {
  for (const language of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const context = await browser.newContext({ locale: language, viewport: { width, height: 900 } })
    const page = await context.newPage()
    let signedIn = false
    await page.route('**/api/admin/**', async route => {
      const path = new URL(route.request().url()).pathname
      if (path.endsWith('/login')) { signedIn = true; return route.fulfill({ status: 204 }) }
      if (path.endsWith('/logout')) { signedIn = false; return route.fulfill({ status: 204 }) }
      return signedIn ? route.fulfill({ json: { users: 12, wishes: 34, lists: 5, reservations: 2 } }) : route.fulfill({ status: 401, json: { error: 'unauthorized' } })
    })
    await page.goto('http://localhost:5173/cp')
    await page.locator('#admin-password').fill('long-enough-password')
    await page.locator('#admin-username').fill('admin')
    const check = async state => {
      const dimensions = await page.evaluate(() => {
        const visible = el => el.getClientRects().length && getComputedStyle(el).visibility !== 'hidden'
        const width = document.documentElement.clientWidth
        return { overflow: document.documentElement.scrollWidth > width, outside: [...document.querySelectorAll('main *, header *')].filter(visible).filter(el => { const r = el.getBoundingClientRect(); return r.left < -1 || r.right > width + 1 }).map(el => el.className), small: [...document.querySelectorAll('a,button,select')].filter(visible).filter(el => { const r = el.getBoundingClientRect(); return r.width < 43.9 || r.height < 43.9 }).map(el => el.tagName) }
      })
      assert.equal(dimensions.overflow, false, `${language}/${width}/${state} overflow`)
      assert.deepEqual(dimensions.outside, [], `${language}/${width}/${state} outside`)
      assert.deepEqual(dimensions.small, [], `${language}/${width}/${state} small targets`)
    }
    await check('login')
    if (language === 'en' && (width === 390 || width === 1440)) await page.screenshot({ path: `../.cache/admin-login-${width}.png`, fullPage: true })
    await page.locator('form button').click()
    await page.locator('.stat').first().waitFor()
    assert.equal(await page.locator('.stat').count(), 4)
    await check('overview')
    if (language === 'en' && (width === 390 || width === 1440)) await page.screenshot({ path: `../.cache/admin-overview-${width}.png`, fullPage: true })
    await page.locator('.header-actions button').click()
    await page.locator('#admin-password').waitFor()
    await context.close()
  }
  console.log('Admin login, overview, logout: EN/RU/RO at five viewport widths passed.')
} finally { await browser.close() }
