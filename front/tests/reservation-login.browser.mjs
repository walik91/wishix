import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5177'
const dictionaries = Object.fromEntries(await Promise.all(['en', 'ru', 'ro'].map(async lang => [lang, JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))])))
const listId = '00000000-0000-4000-8000-000000000001', wishId = '00000000-0000-4000-8000-000000000002', token = 'a'.repeat(43)
const listPath = `/u/example/wishlist/${listId}/${token}`
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const context = await browser.newContext({ locale: 'en' })
const page = await context.newPage(), errors = []
const html = await (await context.request.get(origin)).text()
let loggedIn = false, bookingConfirmations = 0
page.on('pageerror', error => errors.push(error.message))
page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
await page.route('**/u/*/wishlist/**', route => route.request().resourceType() === 'document' ? route.fulfill({ status: 200, contentType: 'text/html', body: html }) : route.continue())
await page.route('**/api/auth/me', route => loggedIn ? route.fulfill({ json: { user: { id: '1', username: 'guest', email: 'guest@example.test' } } }) : route.fulfill({ status: 401, json: { error: 'unauthorized' } }))
await page.route('**/api/auth/login', route => { loggedIn = true; return route.fulfill({ json: { user: { id: '1', username: 'guest', email: 'guest@example.test' } } }) })
await page.route('**/api/public/lists/guest', route => route.fulfill({ json: { username: 'example', list: { id: listId, title: 'Birthday', visibility: 'link', expired: false, event_date: null }, wishes: [{ id: wishId, title: 'Gift', description: '', priority: 'medium', price: null, currency: '', shop_links: [], has_image: false, reserved: false }] } }))
await page.route('**/api/public/profiles/example/wishes', route => route.fulfill({ json: { username: 'example', wishes: [{ id: wishId, title: 'Gift', description: '', priority: 'medium', price: null, currency: '', shop_links: [], has_image: false, reserved: false }] } }))
await page.route('**/api/reservations/start', route => route.fulfill({ status: 202, json: { challenge: 'c'.repeat(43) } }))
await page.route('**/api/reservations/confirm', route => {
  const body = route.request().postDataJSON()
  if (body.code !== '123456') return route.fulfill({ status: 400, json: { error: 'invalid_code' } })
  bookingConfirmations++
  return loggedIn ? route.fulfill({ json: { status: 'ok' } }) : route.fulfill({ status: 409, json: { error: 'reservation_login_required' } })
})
try {
  const en = dictionaries.en
  await page.goto(`${origin}${listPath}`)
  await page.getByRole('button', { name: en['reservation.reserve'] }).click()
  assert.equal(await page.getByText(en['reservation.accountExists']).count(), 0)
  await page.getByLabel(en['reservation.email']).fill('guest@example.test')
  await page.getByRole('button', { name: en['reservation.sendCode'] }).click()
  await page.getByLabel(en['reservation.code']).fill('000000')
  await page.getByRole('button', { name: en['reservation.confirm'] }).click()
  await page.getByText(en['reservation.error.invalid_code']).waitFor()
  assert.equal(await page.getByText(en['reservation.accountExists']).count(), 0)
  await page.getByLabel(en['reservation.code']).fill('123456')
  await page.getByRole('button', { name: en['reservation.confirm'] }).click()
  await page.getByText(en['reservation.accountExists']).waitFor()
  for (const lang of ['en', 'ru', 'ro']) {
    await page.locator('.language-picker select').first().selectOption(lang)
    for (const width of [320, 390, 768, 1024, 1440]) {
      await page.setViewportSize({ width, height: 900 })
      await page.getByText(dictionaries[lang]['reservation.accountExists']).waitFor()
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, `${lang}/${width}: horizontal overflow`)
    }
  }
  await page.locator('.language-picker select').first().selectOption('en')
  await page.getByRole('dialog').getByRole('link', { name: en['reservation.signIn'] }).click()
  await page.waitForURL(`${origin}/login`)
  assert.equal(new URL(page.url()).search, '', 'share token must not be placed in the login URL')
  await page.locator('#email').fill('guest@example.test')
  await page.locator('#password').fill('test password')
  await page.getByRole('button', { name: en['login.submit'], exact: true }).click()
  await page.waitForURL(`${origin}${listPath}`)
  await page.getByRole('button', { name: en['reservation.reserve'] }).click()
  await page.getByRole('button', { name: en['reservation.sendCode'] }).click()
  await page.getByLabel(en['reservation.code']).fill('123456')
  await page.getByRole('button', { name: en['reservation.confirm'] }).click()
  await page.getByText(en['reservation.confirmed']).waitFor()
  assert.equal(bookingConfirmations, 2)

  loggedIn = false
  await page.goto(`${origin}/u/example/wishes`)
  await page.getByRole('button', { name: en['reservation.reserve'] }).click()
  await page.getByLabel(en['reservation.email']).fill('guest@example.test')
  await page.getByRole('button', { name: en['reservation.sendCode'] }).click()
  await page.getByLabel(en['reservation.code']).fill('123456')
  await page.getByRole('button', { name: en['reservation.confirm'] }).click()
  await page.getByText(en['reservation.accountExists']).waitFor()
  await page.getByRole('dialog').getByRole('link', { name: en['reservation.signIn'] }).click()
  await page.waitForURL(`${origin}/login`)
  await page.locator('#email').fill('guest@example.test')
  await page.locator('#password').fill('test password')
  await page.getByRole('button', { name: en['login.submit'], exact: true }).click()
  await page.waitForURL(`${origin}/u/example/wishes`)
  assert.deepEqual(errors, [])
  console.log('Passed: account discovery only after code, sign-in, safe return to list and collection, and reservation.')
} finally { await context.close(); await browser.close() }
