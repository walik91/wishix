// Runs only against compose.auth-test.yaml and its isolated Mailpit/database.
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5174'
if (!/^http:\/\/(localhost|127\.0\.0\.1):5174$/.test(origin)) throw new Error('Run only against the isolated test environment on port 5174')
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const owner = await browser.newContext({ locale: 'en', viewport: { width: 390, height: 844 } })
const guest = await browser.newContext({ locale: 'en', viewport: { width: 390, height: 844 } })
const giver = await browser.newContext({ locale: 'en', viewport: { width: 390, height: 844 } })
const headers = { Origin: origin, 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }
const suffix = Date.now().toString(), oldName = `oa${suffix}`, newName = `on${suffix}`, otherName = `ob${suffix}`
async function call(context, method, path, body, status = 200) {
  const response = await context.request.fetch(`${origin}/api${path}`, { method, headers, ...(body === undefined ? {} : { data: body }) })
  assert.equal(response.status(), status, `${method} ${path}: ${await response.text()}`)
  return response.json()
}
async function account(context, username) {
  const email = `${username}@example.test`
  const challenge = await call(context, 'POST', '/auth/register', { email, username, password: 'test password', locale: 'en' }, 202)
  const { messages } = await (await fetch('http://localhost:8026/api/v1/messages?limit=100')).json()
  const mail = messages.find(item => item.To.some(to => to.Address === email))
  assert.ok(mail, 'Mailpit verification message')
  const detail = await (await fetch(`http://localhost:8026/api/v1/message/${mail.ID}`)).json()
  await call(context, 'POST', '/auth/verify-email', { challenge: challenge.challenge, code: detail.Text.match(/\b[0-9]{6}\b/)[0] })
}
try {
  await account(owner, oldName)
  await account(giver, otherName)
  const page = await owner.newPage()
  await page.goto(`${origin}/my/profile`)
  await page.locator('#profile-birth-date').fill('2000-02-29')
  await page.getByRole('checkbox', { name: 'Show my age publicly' }).check()
  await page.getByRole('checkbox', { name: 'Show my followers publicly' }).check()
  await page.getByRole('button', { name: 'Save profile settings' }).click()
  await page.getByText('Changes saved.', { exact: true }).waitFor()
  await page.locator('#profile-username').fill(newName)
  await page.getByRole('button', { name: 'Change profile address', exact: true }).first().click()
  await page.getByRole('dialog', { name: 'Change profile address' }).getByRole('button', { name: 'Change profile address', exact: true }).click()
  await page.getByText('Username changed.').waitFor()
  const oldResponse = await guest.request.get(`${origin}/u/${oldName}`, { maxRedirects: 0 })
  assert.equal(oldResponse.status(), 302)
  assert.equal(oldResponse.headers().location, `/u/${newName}`)
  assert.equal(oldResponse.headers()['cache-control'], 'no-store')
  const gp = await guest.newPage()
  await gp.goto(`${origin}/u/${oldName}`)
  assert.equal(new URL(gp.url()).pathname, `/u/${newName}`)
  await gp.getByRole('heading', { name: oldName }).waitFor()
  const publicDetails = await call(guest, 'GET', `/public/profiles/${newName}/details`)
  assert.equal(typeof publicDetails.age, 'number'); assert.equal(publicDetails.birth_date, undefined)
  const donorPage = await giver.newPage()
  await donorPage.goto(`${origin}/u/${newName}`)
  await donorPage.getByRole('button', { name: 'Follow', exact: true }).click()
  await donorPage.getByRole('button', { name: 'Unfollow', exact: true }).waitFor()
  const followers = await call(guest, 'GET', `/social/${newName}/people/followers`)
  assert.ok(followers.people.some(person => person.username === otherName))
  await page.goto(`${origin}/u/${otherName}`)
  await page.getByRole('button', { name: 'Block', exact: true }).click()
  await page.getByRole('dialog', { name: 'Block' }).getByRole('button', { name: 'Block', exact: true }).click()
  await page.getByRole('button', { name: 'Unblock', exact: true }).waitFor()
  const after = await call(guest, 'GET', `/social/${newName}/people/followers`)
  assert.equal(after.people.length, 0)
  console.log('Social end-to-end against isolated Mailpit and CockroachDB passed.')
} finally { await owner.close(); await giver.close(); await guest.close(); await browser.close() }
