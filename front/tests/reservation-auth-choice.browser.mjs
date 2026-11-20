// Run against a local Vite server; authentication and reservations are mocked.
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5175'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const htmlContext = await browser.newContext()
const html = await (await htmlContext.request.get(origin)).text()
await htmlContext.close()
const listID = '00000000-0000-4000-8000-000000000001'
const wishID = '00000000-0000-4000-8000-000000000002'
const token = 'a'.repeat(43), listPath = `/u/example/wishlist/${listID}/${token}`
const errors = []
async function setup(lang = 'en', googleEnabled = true, googleFlow = 'existing', afterLogin = 'available') {
  const context = await browser.newContext({ locale: lang, viewport: { width: 390, height: 900 } })
  const page = await context.newPage()
  let signedIn = false, starts = 0, googleStarts = 0
  page.on('pageerror', error => errors.push(error.message))
  page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
  await page.route('**/u/*/wishlist/**', route => route.request().resourceType() === 'document' ? route.fulfill({ status: 200, contentType: 'text/html', body: html }) : route.continue())
  await page.route('**/api/auth/me', route => signedIn ? route.fulfill({ json: { user: { id: 'visitor', username: 'guest', email: 'guest@example.test' } } }) : route.fulfill({ status: 401, json: { error: 'unauthorized' } }))
  await page.route('**/api/auth/google/status', route => route.fulfill({ json: { enabled: googleEnabled } }))
  await page.route('**/api/auth/facebook/status', route => route.fulfill({ json: { enabled: false } }))
  await page.route('**/api/auth/google/start', route => { googleStarts++; return googleFlow === 'failure' ? route.fulfill({ status: 503, json: { error: 'unavailable' } }) : route.fulfill({ json: { url: `${origin}/test-google` } }) })
  await page.route('**/test-google', route => { if (googleFlow === 'existing') signedIn = true; return route.fulfill({ status: 302, headers: { location: googleFlow === 'existing' ? '/login?google=success' : '/google/finish' } }) })
  await page.route('**/api/auth/google/complete', route => { signedIn = true; return route.fulfill({ json: { user: { id: 'visitor', username: 'guest', email: 'guest@example.test' } } }) })
  await page.route('**/api/auth/register', route => route.fulfill({ status: 202, json: { challenge: 'c'.repeat(43), resend_after: 0 } }))
  await page.route('**/api/auth/verify-email', route => { signedIn = true; return route.fulfill({ json: { user: { id: 'visitor', username: 'guest', email: 'guest@example.test' } } }) })
  await page.route('**/api/public/lists/guest', route => route.fulfill({ json: { is_owner: signedIn && afterLogin === 'owner', username: 'example', list: { id: listID, title: 'Birthday', visibility: 'link', expired: false, event_date: null }, wishes: [{ id: wishID, title: 'Gift', description: '', priority: 'medium', price: null, currency: '', shop_links: [], has_image: false, reserved: signedIn && afterLogin === 'reserved' }] } }))
  await page.route('**/api/reservations/start', route => { starts++; return route.fulfill({ status: 202, json: { challenge: 'b'.repeat(43) } }) })
  return { context, page, counts: () => ({ starts, googleStarts }) }
}
try {
  for (const lang of ['en', 'ru', 'ro']) for (const enabled of [true, false]) {
    const texts = JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))
    const { context, page } = await setup(lang, enabled)
    await page.goto(origin + listPath)
    const statusResponse = page.waitForResponse(response => response.url().endsWith('/api/auth/google/status'))
    await page.getByRole('button', { name: texts['reservation.reserve'], exact: true }).click()
    await statusResponse
    const dialog = page.getByRole('dialog')
    if (enabled) await dialog.getByRole('button', { name: texts['login.google'] }).waitFor()
    assert.equal(await dialog.getByRole('button', { name: texts['login.google'] }).count(), enabled ? 1 : 0)
    assert.equal(await dialog.getByRole('link', { name: texts['reservation.signInOrCreate'] }).count(), 1)
    assert.equal(await dialog.getByRole('button', { name: texts['reservation.continueGuest'] }).count(), 1)
    assert.equal(await dialog.getByLabel(texts['reservation.email']).count(), 0)
    for (const width of [320, 390, 768, 1024, 1440]) {
      await page.setViewportSize({ width, height: 900 })
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false)
      for (const control of await dialog.locator('button, a').all()) {
        const box = await control.boundingBox()
        assert.ok(box && box.height >= 44 && box.width >= 44, `${lang} ${width}: touch target`)
      }
    }
    await context.close()
  }
  for (const flow of ['existing', 'new']) {
    const { context, page, counts } = await setup('en', true, flow)
    await page.goto(origin + listPath)
    await page.getByRole('button', { name: 'Reserve gift', exact: true }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Continue with Google' }).click()
    if (flow === 'new') {
      await page.waitForURL(origin + '/google/finish')
      await page.locator('#username').fill('newguest')
      await page.locator('.finish-form button[type=submit]').click()
    }
    await page.waitForURL(origin + listPath)
    await page.getByRole('dialog').getByRole('button', { name: 'Send code' }).waitFor()
    assert.deepEqual(counts(), { starts: 0, googleStarts: 1 }, 'sign-in must not create a reservation')
    assert.equal(await page.locator('dialog[open] #email-' + wishID).inputValue(), 'guest@example.test')
    await context.close()
  }
  for (const afterLogin of ['owner', 'reserved']) {
    const { context, page, counts } = await setup('en', true, 'existing', afterLogin)
    await page.goto(origin + listPath)
    await page.getByRole('button', { name: 'Reserve gift', exact: true }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Continue with Google' }).click()
    await page.waitForURL(origin + listPath)
    await page.locator('.wish-card').waitFor()
    assert.equal(await page.locator('dialog[open]').count(), 0, `must not resume for ${afterLogin}`)
    assert.equal(counts().starts, 0)
    await context.close()
  }
  {
    const { context, page } = await setup('en', true, 'failure')
    await page.goto(origin + listPath)
    await page.getByRole('button', { name: 'Reserve gift', exact: true }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Continue with Google' }).click()
    await page.getByRole('dialog').getByRole('alert').waitFor()
    assert.equal(await page.evaluate(() => sessionStorage.getItem('wishix_reservation_return')), null)
    await page.getByRole('dialog').getByRole('button', { name: 'Continue without an account' }).click()
    await page.getByRole('dialog').getByLabel('Email address').waitFor()
    await context.close()
  }
  {
    const { context, page, counts } = await setup('en', false)
    await page.goto(origin + listPath)
    await page.getByRole('button', { name: 'Reserve gift', exact: true }).click()
    await page.getByRole('dialog').getByRole('link', { name: 'Sign in or create an account' }).click()
    await page.waitForURL(origin + '/login')
    assert.equal(new URL(page.url()).search, '')
    await page.locator('#main-content').getByRole('link', { name: 'Create an account' }).click()
    await page.waitForURL(origin + '/register')
    await page.locator('#username').fill('newguest')
    await page.locator('#email').fill('guest@example.test')
    await page.locator('#password').fill('password')
    await page.locator('.register-form button[type=submit]').click()
    await page.locator('#code').fill('123456')
    await page.locator('.code-form button[type=submit]').click()
    await page.waitForURL(origin + listPath)
    await page.getByRole('dialog').getByRole('button', { name: 'Send code' }).waitFor()
    assert.equal(counts().starts, 0, 'registration must not create a reservation')
    await context.close()
  }
  assert.deepEqual(errors, [])
  console.log('Reservation auth choice: Google, email registration, guest option, and responsive layouts passed')
} finally { await browser.close() }
