import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5177'
const dictionaries = Object.fromEntries(await Promise.all(['en', 'ru', 'ro'].map(async lang => [lang, JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))])))
const texts = dictionaries.en
const listId = '00000000-0000-4000-8000-000000000001'
const wishId = '00000000-0000-4000-8000-000000000002'
const token = 'a'.repeat(43), managementToken = 'b'.repeat(43)
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const context = await browser.newContext({ locale: 'en' })
const page = await context.newPage(), errors = []
let reserved = false, expired = false, purchased = false
const html = await (await context.request.get(origin)).text()
page.on('pageerror', error => errors.push(error.message))
page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
await page.route('**/u/*/wishlist/**', route => route.request().resourceType() === 'document' ? route.fulfill({ status: 200, contentType: 'text/html', body: html }) : route.continue())
await page.route('**/api/auth/me', route => route.fulfill({ status: 401, json: { error: 'unauthorized' } }))
await page.route('**/api/public/lists/guest', route => route.fulfill({ json: { username: 'example', list: { id: listId, title: 'Birthday', visibility: 'link', expired, event_date: null }, wishes: expired ? [] : [{ id: wishId, title: 'Gift', description: '', priority: 'medium', price: null, currency: '', shop_links: [], has_image: false, reserved, guest_recoverable: reserved }] } }))
await page.route('**/api/reservations/start', route => { const body = route.request().postDataJSON(); assert.equal(body.email, 'guest@example.test'); assert.equal(body.list_token, token); return route.fulfill({ status: 202, json: { challenge: 'c'.repeat(43) } }) })
await page.route('**/api/reservations/confirm', route => { assert.equal(route.request().postDataJSON().code, '123456'); reserved = true; return route.fulfill({ json: { status: 'ok' } }) })
await page.route('**/api/reservations/recover', route => route.fulfill({ status: 202, json: { status: 'ok' } }))
await page.route('**/api/reservations/manage', route => { const body = route.request().postDataJSON(); assert.equal(body.token, managementToken); if (body.action === 'purchase') purchased = true; if (body.action === 'undo_purchase') purchased = false; return route.fulfill({ json: { reservation: { wish_title: 'Gift', list_title: 'Birthday', status: purchased ? 'purchased' : 'reserved', expires_at: purchased ? null : '2027-01-01T00:00:00Z', can_extend: !purchased } } }) })
try {
  await page.goto(`${origin}/u/example/wishlist/${listId}/${token}`)
  await page.getByRole('button', { name: texts['reservation.reserve'] }).click()
  await page.getByRole('button', { name: texts['reservation.continueGuest'] }).click()
  await page.getByLabel(texts['reservation.email']).fill('guest@example.test')
  await page.getByRole('button', { name: texts['reservation.sendCode'] }).click()
  await page.getByLabel(texts['reservation.code']).fill('123456')
  await page.getByRole('button', { name: texts['reservation.confirm'] }).click()
  await page.getByText(texts['reservation.confirmed']).waitFor()
  await page.getByRole('dialog').getByRole('button', { name: texts['my.close'] }).click()
  await page.getByText(texts['reservation.reserved'], { exact: true }).waitFor()
  for (const lang of ['en', 'ru', 'ro']) {
    const dictionary = dictionaries[lang]
    await page.locator('.language-picker select').first().selectOption(lang)
    for (const width of [320, 390, 768, 1024, 1440]) {
      await page.setViewportSize({ width, height: 900 })
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true, `horizontal overflow at ${lang}/${width}px`)
      await page.getByRole('button', { name: dictionary['reservation.needLink'] }).click()
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true, `dialog overflow at ${lang}/${width}px`)
      await page.getByRole('dialog').getByRole('button', { name: dictionary['my.close'] }).click()
    }
  }
  await page.locator('.language-picker select').first().selectOption('en')
  await page.goto(`${origin}/reservation#${managementToken}`)
  await page.getByRole('button', { name: texts['reservation.markPurchased'] }).click()
  await page.getByText(texts['reservation.purchased'], { exact: true }).waitFor()
  assert.equal(new URL(page.url()).hash, '')
  await page.getByRole('button', { name: texts['reservation.undoKeep'] }).click()
  await page.getByText(texts['reservation.reserved'], { exact: true }).waitFor()
  expired = true
  await page.goto(`${origin}/u/example/wishlist/${listId}/${token}`)
  await page.getByText(texts['lists.expired'], { exact: true }).waitFor()
  assert.equal(await page.getByText('Gift', { exact: true }).count(), 0)
  assert.deepEqual(errors, [])
  console.log('Passed: booking, guest recovery UI, management, closed list and responsive widths.')
} finally { await context.close(); await browser.close() }
