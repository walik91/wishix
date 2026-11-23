// End-to-end list management against compose.auth-test.yaml only (Mailpit, isolated DB).
import assert from 'node:assert/strict'
import { readFile, mkdir } from 'node:fs/promises'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5174'
if (!/^http:\/\/(localhost|127\.0\.0\.1):5174$/.test(origin)) throw new Error('Run only against the isolated test environment on port 5174')
const texts = Object.fromEntries(await Promise.all(['en', 'ru', 'ro'].map(async lang => [lang, JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))])))
const browser = await chromium.launch({ headless: true, channel: 'chrome' })
const headers = { Origin: origin, 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }
const context = await browser.newContext({ locale: 'en', viewport: { width: 390, height: 844 } })
const issues = [], traffic = [], results = []
const suffix = Date.now(), username = `lists_${suffix}`, email = `${username}@example.test`
let registered = false, png
async function call(method, path, data, wanted = 200, client = context) {
  const response = await client.request.fetch(`${origin}/api${path}`, { method, headers, ...(data === undefined ? {} : { data }) })
  assert.equal(response.status(), wanted, `${method} ${path}: ${await response.text()}`)
  return response.json()
}
function observe(page) {
  page.on('pageerror', error => issues.push(error.message))
  page.on('console', message => { if (message.text().includes('[Vue warn]')) issues.push(message.text()) })
  page.on('request', request => traffic.push(request.url()))
}
const tx = key => texts.en[key]
async function button(page, key) { await page.getByRole('button', { name: tx(key), exact: true }).click() }
async function manage(page, key) { await button(page, 'lists.manage'); await page.getByRole('dialog', { name: tx('lists.manage'), exact: true }).getByRole('button', { name: tx(key), exact: true }).click() }
async function confirm(page, key) { await page.getByRole('dialog', { name: tx(key), exact: true }).getByRole('button', { name: tx(key), exact: true }).click() }
async function waitList(page) { await page.locator('.list-toolbar').waitFor() }
async function geometry(page, label) {
  await page.evaluate(() => document.fonts.ready)
  const state = await page.evaluate(() => {
    const visible = el => el.getClientRects().length && !el.classList.contains('sr-only') && getComputedStyle(el).visibility !== 'hidden'
    const width = document.documentElement.clientWidth
    const active = document.querySelector('dialog[open]') || document.querySelector('main')
    const elements = [...active.querySelectorAll('*')].filter(visible)
    return { overflow: document.documentElement.scrollWidth > width,
      outside: elements.filter(el => { const r = el.getBoundingClientRect(); return r.left < -1 || r.right > width + 1 }).map(el => el.id || el.className),
      small: [...active.querySelectorAll('a,button,input,select,textarea')].filter(visible).map(el => el.matches('input[type=checkbox]') ? el.closest('label') : el).filter(el => { const r = el.getBoundingClientRect(); return r.width < 43.9 || r.height < 43.9 }).map(el => el.id || el.textContent),
      untranslated: /\b(?:lists|wishes)\.[a-zA-Z]/.test(active.innerText) }
  })
  assert.equal(state.overflow, false, label); assert.deepEqual(state.outside, [], label); assert.deepEqual(state.small, [], label); assert.equal(state.untranslated, false, label)
}
try {
  const registeredResponse = await call('POST', '/auth/register', { email, username, password: 'test password', locale: 'en' }, 202)
  const { messages } = await (await fetch('http://localhost:8026/api/v1/messages?limit=100')).json()
  const mail = messages.find(item => item.To.some(to => to.Address === email))
  const detail = await (await fetch(`http://localhost:8026/api/v1/message/${mail.ID}`)).json()
  await call('POST', '/auth/verify-email', { challenge: registeredResponse.challenge, code: detail.Text.match(/\b[0-9]{6}\b/)[0] })
  registered = true
  const page = await context.newPage(); observe(page)
  await page.addInitScript(() => { Object.defineProperty(navigator, 'clipboard', { value: { writeText: async text => { window.testClipboard = text } }, configurable: true }) })
  await page.goto(`${origin}/my/lists`)
  await page.getByRole('heading', { name: tx('lists.emptyTitle') }).waitFor()
  await page.locator('.list-page-heading').getByRole('link', { name: tx('lists.create'), exact: true }).click()
  await page.locator('#list-title').waitFor()
  assert.equal(await page.locator('#list-visibility').inputValue(), 'link')
  await button(page, 'lists.create')
  await page.getByText(tx('lists.error.list_title_invalid'), { exact: true }).waitFor()
  await page.locator('#list-title').fill('Birthday')
  await page.locator('#list-description').fill('Gifts for the next occasion')
  await page.locator('#list-date').fill('2027-06-14')
  await button(page, 'lists.create'); await waitList(page)
  const firstID = page.url().match(/\/my\/lists\/([a-f0-9-]{36})/)[1]
  const { list: second } = await call('POST', '/lists', { title: 'For later', visibility: 'private' }, 201)
  await page.locator('.list-toolbar').getByRole('link', { name: tx('my.addWish'), exact: true }).click()
  await button(page, 'lists.wishLists')
  await page.locator('.list-check-row').first().waitFor()
  assert.equal(await page.locator(`input[type=checkbox][value="${firstID}"]`).isChecked(), true)
  await page.locator(`input[type=checkbox][value="${second.id}"]`).check()
  await page.locator('#wish-title').fill('Gift A')
  png = Buffer.from(await page.evaluate(() => { const canvas = document.createElement('canvas'); canvas.width = 80; canvas.height = 40; canvas.getContext('2d').fillRect(0, 0, 80, 40); return canvas.toDataURL('image/png').split(',')[1] }), 'base64')
  await button(page, 'wishes.addDetails')
  await page.locator('#wish-image').setInputFiles({ name: 'gift.png', mimeType: 'image/png', buffer: png })
  await page.locator('button[type=submit]').click(); await waitList(page)
  assert.ok(page.url().includes(firstID))
  const { wishes: created } = await call('GET', '/wishes')
  assert.equal(created.length, 1); const gift = created[0]
  const { wish: another } = await call('POST', '/wishes', { title: 'Gift B', price: '12.50', currency: 'EUR' }, 201)
  const originalLink = `${origin}/u/${username}/wishlist${(await call('GET', `/lists/${firstID}`)).list.share_path}`
  const originalToken = originalLink.split('/').at(-1)
  await button(page, 'lists.fromCollection')
  const picker = page.getByRole('dialog', { name: tx('lists.fromCollection'), exact: true })
  await picker.getByRole('checkbox', { name: 'Gift B', exact: true }).check()
  assert.equal(await picker.getByRole('checkbox', { name: 'Gift A', exact: true }).count(), 0)
  await picker.getByRole('button', { name: /Add selected/ }).click()
  await page.locator(`[data-wish-id="${another.id}"]`).waitFor()
  await manage(page, 'lists.edit')
  const edit = page.getByRole('dialog', { name: tx('lists.edit'), exact: true })
  await edit.locator('#list-title').fill('Birthday edited')
  await edit.locator('#list-date').fill('2027-07-15')
  await edit.locator('#list-visibility').selectOption('public')
  await edit.getByRole('button', { name: tx('lists.save'), exact: true }).click()
  await page.getByRole('heading', { name: 'Birthday edited', exact: true }).waitFor()
  await button(page, 'lists.share')
  const share = page.getByRole('dialog', { name: tx('lists.share'), exact: true })
  const oldURL = await share.locator('#list-share-url').inputValue()
  assert.equal(new URL(oldURL).pathname, `/u/${username}/wishlist/${firstID}`)
  assert.equal(await share.getByRole('button', { name: tx('lists.rotate'), exact: true }).count(), 0)
  const staleURL = oldURL.replace(`/u/${username}/`, '/u/old_username/')
  const redirect = await context.request.get(staleURL, { maxRedirects: 0 })
  assert.equal(redirect.status(), 301)
  assert.equal(new URL(redirect.headers().location, origin).href, oldURL)
  assert.equal(redirect.headers()['x-robots-tag'], undefined)
  assert.equal(redirect.headers()['referrer-policy'], 'no-referrer')
  await share.getByRole('button', { name: tx('lists.copyLink'), exact: true }).click()
  assert.equal(await page.evaluate(() => window.testClipboard), oldURL)
  const guest = await browser.newContext({ locale: 'en' }), guestPage = await guest.newPage(); observe(guestPage)
  const publicPage = await guestPage.goto(oldURL)
  assert.equal(publicPage.headers()['x-robots-tag'], undefined)
  assert.equal(publicPage.headers()['referrer-policy'], 'no-referrer')
  await guestPage.getByRole('heading', { name: 'Birthday edited', exact: true }).waitFor()
  await guestPage.locator(`[data-wish-id="${gift.id}"] img`).waitFor()
  await guestPage.locator(`[data-wish-id="${gift.id}"] img`).evaluate(img => img.decode())
  assert.equal(await guestPage.locator('.app-shell').count(), 0)
  assert.equal(await guestPage.getByRole('button', { name: /Delete|Remove from list|Manage list/ }).count(), 0)
  assert.equal((await guestPage.locator('body').innerText()).includes(email), false)
  await guestPage.goto(`${origin}/u/${username}`)
  await guestPage.getByRole('heading', { name: 'Birthday edited', exact: true }).waitFor()
  assert.equal(await guestPage.getByRole('heading', { name: 'For later', exact: true }).count(), 0)
  const publicRedirect = await context.request.get(originalLink, { maxRedirects: 0 })
  assert.equal(publicRedirect.status(), 302)
  assert.equal(new URL(publicRedirect.headers().location, origin).href, oldURL)
  assert.equal(publicRedirect.headers()['cache-control'], 'no-store')
  await share.getByRole('button', { name: tx('my.close'), exact: true }).click()
  await manage(page, 'lists.edit'); await edit.locator('#list-visibility').selectOption('link'); await edit.getByRole('button', { name: tx('lists.save'), exact: true }).click(); await waitList(page)
  await guestPage.goto(oldURL); await guestPage.getByRole('heading', { name: tx('lists.sharedUnavailable') }).waitFor()
  await guestPage.goto(originalLink); await guestPage.getByRole('heading', { name: 'Birthday edited', exact: true }).waitFor()
  await button(page, 'lists.share'); assert.equal(await page.locator('#list-share-url').inputValue(), originalLink)
  const iconSize = await share.getByRole('button', { name: tx('lists.rotate'), exact: true }).boundingBox()
  assert.ok(iconSize.width >= 44 && iconSize.height >= 44)
  await share.getByRole('button', { name: tx('lists.rotate'), exact: true }).click()
  const rotateDialog = page.getByRole('dialog', { name: tx('lists.rotate'), exact: true })
  await rotateDialog.getByText(tx('lists.confirm.rotate')).waitFor()
  await rotateDialog.getByRole('button', { name: tx('wishes.cancel'), exact: true }).click()
  await guestPage.reload(); await guestPage.getByRole('heading', { name: 'Birthday edited', exact: true }).waitFor()
  await button(page, 'lists.share')
  await share.getByRole('button', { name: tx('lists.rotate'), exact: true }).click(); await confirm(page, 'lists.rotate'); await waitList(page)
  await guestPage.goto(originalLink); await guestPage.getByRole('heading', { name: tx('lists.sharedUnavailable') }).waitFor()
  const newURL = await page.locator('#list-share-url').inputValue(); assert.notEqual(newURL, oldURL)
  assert.notEqual(newURL, originalLink)
  await guestPage.goto(newURL); await guestPage.getByRole('heading', { name: 'Birthday edited', exact: true }).waitFor()
  assert.equal(traffic.filter(url => url.includes(originalToken) || url.includes(newURL.split('/').at(-1))).every(url => new URL(url).pathname.includes('/wishlist/')), true, 'capability appeared in an unexpected network URL')
  await share.getByRole('button', { name: tx('my.close'), exact: true }).click()
  await manage(page, 'lists.copy'); await page.locator('#list-copy-title').fill('Next year'); await confirm(page, 'lists.copy'); await page.waitForURL(url => /\/my\/lists\/[a-f0-9-]{36}/.test(url.pathname) && !url.pathname.includes(firstID)); await waitList(page)
  const copyID = page.url().match(/\/my\/lists\/([a-f0-9-]{36})/)[1]; assert.notEqual(copyID, firstID)
  const copy = await call('GET', `/lists/${copyID}`)
  assert.equal(copy.list.event_date, null); assert.equal(copy.list.visibility, 'link'); assert.deepEqual(copy.wishes.map(w => w.id).sort(), [gift.id, another.id].sort())
  await manage(page, 'lists.clear'); await confirm(page, 'lists.clear'); await page.getByRole('heading', { name: tx('lists.emptyListTitle') }).waitFor()
  assert.equal((await call('GET', '/wishes')).wishes.length, 2)
  await page.goto(`${origin}/my/lists/${firstID}`); await waitList(page)
  await page.getByRole('button', { name: `${tx('lists.remove')}: Gift A`, exact: true }).click(); await confirm(page, 'lists.remove')
  await page.locator(`[data-wish-id="${gift.id}"]`).waitFor({ state: 'detached' })
  assert.equal((await call('GET', `/lists/${second.id}`)).wishes[0].id, gift.id)
  await page.goto(`${origin}/my/wishes`)
  await page.locator(`[data-wish-id="${gift.id}"]`).getByRole('button', { name: tx('lists.wishLists'), exact: true }).click()
  const membership = page.getByRole('dialog', { name: tx('lists.wishLists'), exact: true })
  await membership.locator(`input[value="${firstID}"]`).check(); await membership.locator(`input[value="${second.id}"]`).uncheck()
  await membership.getByRole('button', { name: tx('lists.save'), exact: true }).click()
  assert.equal((await call('GET', `/lists/${firstID}`)).wishes.length, 2)
  assert.equal((await call('GET', `/lists/${second.id}`)).wishes.length, 0)
  // A stale edit must preserve the form and offer an explicit reload.
  await page.goto(`${origin}/my/lists/${firstID}`); await waitList(page); await manage(page, 'lists.edit')
  await edit.locator('#list-title').fill('My stale edit')
  const current = await call('GET', `/lists/${firstID}`)
  await call('PUT', `/lists/${firstID}`, { title: 'Another tab', visibility: 'public', revision: current.list.revision })
  await edit.getByRole('button', { name: tx('lists.save'), exact: true }).click()
  await edit.getByRole('alert').waitFor(); assert.equal(await edit.locator('#list-title').inputValue(), 'My stale edit')
  await edit.getByRole('button', { name: tx('lists.reload'), exact: true }).click()
  await page.getByRole('heading', { name: 'Another tab', exact: true }).waitFor()
  await manage(page, 'lists.delete'); await confirm(page, 'lists.delete'); await page.waitForURL(/\/my\/lists(?:\?|$)/)
  await page.getByText(tx('lists.notice.deleted'), { exact: true }).waitFor()
  assert.equal((await call('GET', '/wishes')).wishes.length, 2)
  await guestPage.reload(); await guestPage.getByRole('heading', { name: tx('lists.sharedUnavailable') }).waitFor()
  await guest.close()
  results.push('CRUD, default access, multi-list wish creation, picker, sharing, images, profile privacy, link rotation, linked copy, clear, detach, membership editing, stale edit and deletion')

  const fixtures = []
  for (const visibility of ['link', 'public', 'private']) fixtures.push((await call('POST', '/lists', { title: `${visibility}: ${'Long list name '.repeat(6)}`, description: 'A'.repeat(200), event_date: '2027-08-31', visibility }, 201)).list)
  if (process.env.WISHIX_PRODUCTION_TEST_ORIGIN) {
    const production = process.env.WISHIX_PRODUCTION_TEST_ORIGIN
    if (!/^http:\/\/(localhost|127\.0\.0\.1):5175$/.test(production)) throw new Error('Production proxy checks require the isolated port 5175')
    for (const [index, visibility] of ['link', 'public'].entries()) {
      const canonical = `${production}/u/${username}/wishlist${fixtures[index].share_path}`
      const stale = canonical.replace(`/u/${username}/`, '/u/old_username/')
      const moved = await context.request.get(stale, { maxRedirects: 0 })
      assert.equal(moved.status(), 301)
      assert.equal(new URL(moved.headers().location, production).href, canonical)
      assert.equal(moved.headers()['referrer-policy'], 'no-referrer')
      const loaded = await context.request.get(canonical)
      assert.equal(loaded.status(), 200)
      assert.match(loaded.headers()['content-type'], /text\/html/)
      assert.equal(loaded.headers()['referrer-policy'], 'no-referrer')
      if (visibility === 'link') {
        assert.match(moved.headers()['x-robots-tag'], /noindex/)
        assert.match(loaded.headers()['x-robots-tag'], /noindex/)
      } else {
        assert.equal(moved.headers()['x-robots-tag'], undefined)
        assert.equal(loaded.headers()['x-robots-tag'], undefined)
      }
    }
    results.push('production Nginx: 301 and HTML response headers for public and link-only lists')
  }
  await call('POST', `/lists/${fixtures[0].id}/wishes`, { wish_ids: [gift.id, another.id] })
  const state = await context.storageState(); state.cookies = state.cookies.filter(cookie => cookie.name !== 'wishix_locale')
  await mkdir(new URL('../../.cache/list-screens/', import.meta.url), { recursive: true })
  let layouts = 0
  for (const lang of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const responsive = await browser.newContext({ storageState: state, locale: lang, viewport: { width, height: width < 768 ? 844 : 900 } })
    const view = await responsive.newPage(); observe(view)
    await view.goto(`${origin}/my/lists`); await view.locator('.list-card').first().waitFor(); await geometry(view, `${lang}/${width}/overview`)
    await view.goto(`${origin}/my/lists/new`); await view.locator('#list-title').waitFor(); await geometry(view, `${lang}/${width}/form`)
    await view.goto(`${origin}/my/lists/${fixtures[0].id}`); await waitList(view); await geometry(view, `${lang}/${width}/detail`)
    await view.getByRole('button', { name: texts[lang]['lists.share'], exact: true }).click()
    await geometry(view, `${lang}/${width}/share`)
    if (lang === 'ru' && [390, 1440].includes(width)) await view.screenshot({ path: new URL(`../../.cache/list-screens/share-${width}.png`, import.meta.url).pathname.replace(/^\/(\w:)/, '$1'), fullPage: true })
    await view.getByRole('dialog', { name: texts[lang]['lists.share'], exact: true }).getByRole('button', { name: texts[lang]['my.close'], exact: true }).click()
    await view.getByRole('button', { name: texts[lang]['lists.manage'], exact: true }).click(); await geometry(view, `${lang}/${width}/manage`)
    await view.getByRole('dialog').filter({ has: view.getByRole('button', { name: texts[lang]['lists.edit'], exact: true }) }).getByRole('button', { name: texts[lang]['lists.edit'], exact: true }).click()
    await view.locator('dialog[open] #list-title').waitFor(); await geometry(view, `${lang}/${width}/edit`)
    await view.locator('dialog[open]').getByRole('button', { name: texts[lang]['wishes.cancel'], exact: true }).click()
    await view.waitForFunction(() => document.activeElement?.closest('.list-toolbar') !== null)
    if (lang === 'ru' && [390, 1440].includes(width)) await view.screenshot({ path: new URL(`../../.cache/list-screens/detail-${width}.png`, import.meta.url).pathname.replace(/^\/(\w:)/, '$1'), fullPage: true })
    await view.goto(`${origin}/u/${username}/wishlist${fixtures[0].share_path}`); await view.getByRole('heading', { name: fixtures[0].title, exact: true }).waitFor(); await geometry(view, `${lang}/${width}/guest`)
    layouts += 7; await responsive.close()
  }
  results.push(`${layouts} layout states: EN/RU/RO × 320/390/768/1024/1440, without overflow or undersized touch targets`)
  assert.deepEqual(issues, [])
  console.log(JSON.stringify({ passed: results, vue_errors: 0 }, null, 2))
} finally {
  if (registered) {
    for (const list of (await call('GET', '/lists')).lists) await call('DELETE', `/lists/${list.id}`, { revision: list.revision })
    for (const wish of (await call('GET', '/wishes')).wishes) await call('DELETE', `/wishes/${wish.id}`, {})
  }
  await browser.close()
}
