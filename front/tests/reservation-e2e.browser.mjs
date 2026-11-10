// Runs only against compose.auth-test.yaml: isolated CockroachDB and Mailpit.
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5174'
if (!/^http:\/\/(localhost|127\.0\.0\.1):5174$/.test(origin)) throw new Error('Use only the isolated test app on port 5174')
const texts = JSON.parse(await readFile(new URL('../src/locales/en.json', import.meta.url)))
const browser = await chromium.launch({ headless: true, channel: 'chrome' })
const owner = await browser.newContext({ locale: 'en' })
const guest = await browser.newContext({ locale: 'en', viewport: { width: 390, height: 844 } })
const username = `book_${Date.now()}`, email = `${username}@example.test`, guestEmail = `guest_${Date.now()}@example.test`
const headers = { Origin: origin, 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }
const errors = []
let listID = ''
async function call(method, path, body, status = 200, client = owner) {
  const response = await client.request.fetch(`${origin}/api${path}`, { method, headers, ...(body === undefined ? {} : { data: body }) })
  assert.equal(response.status(), status, `${method} ${path}: ${await response.text()}`)
  return response.json()
}
async function message(address, accept) {
  for (let i = 0; i < 40; i++) {
    const { messages } = await (await fetch('http://localhost:8026/api/v1/messages?limit=100')).json()
    for (const item of messages.filter(item => item.To.some(to => to.Address === address))) {
      const detail = await (await fetch(`http://localhost:8026/api/v1/message/${item.ID}`)).json()
      if (accept(detail.Text)) return detail.Text
    }
    await new Promise(resolve => setTimeout(resolve, 2000))
  }
  throw new Error('Timed out waiting for test email')
}
try {
  const reservationPage = await guest.request.get(`${origin}/reservation`)
  assert.equal(reservationPage.headers()['cache-control'], 'no-store')
  assert.equal(reservationPage.headers()['referrer-policy'], 'no-referrer')
  assert.match(reservationPage.headers()['x-robots-tag'], /noindex/)
  const registration = await call('POST', '/auth/register', { email, username, password: 'test password', locale: 'en' }, 202)
  const signup = await message(email, text => /\b[0-9]{6}\b/.test(text))
  await call('POST', '/auth/verify-email', { challenge: registration.challenge, code: signup.match(/\b[0-9]{6}\b/)[0] })
  const { list } = await call('POST', '/lists', { title: 'Test celebration', event_date: '2036-12-20', event_timezone: 'UTC', visibility: 'link' }, 201)
  listID = list.id
  await call('POST', '/wishes', { title: 'Test gift', list_ids: [list.id] }, 201)
  const page = await guest.newPage()
  page.on('pageerror', error => errors.push(error.message))
  page.on('console', entry => { if (entry.text().includes('[Vue warn]')) errors.push(entry.text()) })
  await page.goto(`${origin}/u/${username}/wishlist${list.share_path}`)
  await page.getByRole('button', { name: texts['reservation.reserve'] }).click()
  await page.getByLabel(texts['reservation.email']).fill(guestEmail)
  await page.getByRole('button', { name: texts['reservation.sendCode'] }).click()
  const codeMail = await message(guestEmail, text => /\b[0-9]{6}\b/.test(text))
  await page.getByLabel(texts['reservation.code']).fill(codeMail.match(/\b[0-9]{6}\b/)[0])
  await page.getByRole('button', { name: texts['reservation.confirm'] }).click()
  await page.getByText(texts['reservation.confirmed']).waitFor()
  const linkMail = await message(guestEmail, text => text.includes('/reservation#'))
  const link = linkMail.match(/http:\/\/localhost:5174\/reservation#[A-Za-z0-9_-]{43}/)?.[0]
  assert.ok(link, 'management link missing from email')
  await page.goto(link)
  await page.getByRole('button', { name: texts['reservation.markPurchased'] }).click()
  await page.getByText(texts['reservation.purchased'], { exact: true }).waitFor()
  await page.getByRole('button', { name: texts['reservation.undoKeep'] }).click()
  await page.getByText(texts['reservation.reserved'], { exact: true }).waitFor()
  assert.deepEqual(errors, [])
  console.log('Passed: real code email, booking, management link, purchase and undo via isolated Mailpit.')
} finally {
  if (listID) {
    try { const { list } = await call('GET', `/lists/${listID}`); await call('DELETE', `/lists/${listID}`, { revision: list.revision }) } catch { /* Preserve failure from the test. */ }
  }
  await owner.close(); await guest.close(); await browser.close()
}
