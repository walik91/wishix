import assert from 'node:assert/strict'
import { chromium } from 'playwright'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
try {
  for (const language of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const context = await browser.newContext({ locale: language, viewport: { width, height: 900 } })
    const page = await context.newPage()
    let signedIn = false, deleted = false
    const person = { id: '00000000-0000-4000-8000-000000000001', username: 'long_user_name_123', display_name: 'Very long display name '.repeat(4), email: 'a_very_long_email_address_for_the_example@example.test', created_at: '2026-11-01T12:00:00Z' }
    await page.route('**/api/admin/**', async route => {
      const path = new URL(route.request().url()).pathname
      if (path.endsWith('/login')) { signedIn = true; return route.fulfill({ status: 204 }) }
      if (path.endsWith('/logout')) { signedIn = false; return route.fulfill({ status: 204 }) }
      if (!signedIn) return route.fulfill({ status: 401, json: { error: 'unauthorized' } })
      if (path.endsWith('/delete')) { deleted = true; return route.fulfill({ status: 204 }) }
      if (path.endsWith('/users')) return route.fulfill({ json: { users: [{ ...person, deleted_at: deleted ? '2026-11-02T12:00:00Z' : null }], has_more: false } })
      return route.fulfill({ json: { users: deleted ? 0 : 1, wishes: 34, lists: 5, reservations: 2 } })
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
    await page.locator('form button').click()
    await page.locator('.user-row').waitFor()
    assert.equal(await page.locator('.stat').count(), 4)
    await check('overview')
    if (language === 'en' && (width === 390 || width === 1440)) await page.screenshot({ path: `../.cache/admin-users-${width}.png`, fullPage: true })
    await page.locator('.user-row button').click()
    await page.locator('dialog[open]').waitFor()
    assert.equal(deleted, false, 'confirmation must precede deletion')
    await page.locator('dialog .dialog-actions .destructive').click()
    await page.locator('.deleted-label').waitFor()
    assert.equal(deleted, true)
    assert.equal(await page.locator('.user-row button').count(), 0)
    await check('deleted')
    await page.locator('.header-actions button').click()
    await page.locator('#admin-password').waitFor()
    await context.close()
  }
  console.log('Admin users, confirmation and deletion: EN/RU/RO at five viewport widths passed.')
} finally { await browser.close() }
