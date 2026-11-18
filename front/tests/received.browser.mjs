import assert from 'node:assert/strict'
import { readFile, mkdir } from 'node:fs/promises'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')

const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5173'
const texts = Object.fromEntries(await Promise.all(['en', 'ru', 'ro'].map(async lang => [lang, JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))])))
const indexHTML = await readFile(new URL('../index.html', import.meta.url), 'utf8')
const listID = '00000000-0000-4000-8000-000000000001'
const wishID = '00000000-0000-4000-8000-000000000002'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const errors = []
await mkdir(new URL('../../.cache/received-screens/', import.meta.url), { recursive: true })
try {
  for (const lang of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const context = await browser.newContext({ locale: lang, viewport: { width, height: 900 } })
    const page = await context.newPage()
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    let received = false
    const wish = () => ({ id: wishID, title: 'Gift certificate', description: '', price: null, currency: '', priority: 'medium', shop_links: [], has_image: false, received })
    const list = { id: listID, title: 'Birthday', description: '', event_date: null, visibility: 'public', revision: 1, share_path: `/${listID}`, wish_count: 1 }
    await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { username: 'example', email: 'example@example.test' } } }))
    await page.route(`**/api/lists/${listID}`, route => route.fulfill({ json: { list, wishes: [wish()] } }))
    await page.route(`**/api/lists/${listID}/wishes/${wishID}/received`, route => { received = route.request().postDataJSON().received; return route.fulfill({ json: { status: 'ok' } }) })
    await page.route('**/api/public/lists/guest', route => route.fulfill({ json: { list, username: 'example', wishes: [wish()] } }))
    await page.route('**/api/public/profiles/example/wishes', route => { const { received: ignored, ...publicWish } = wish(); return route.fulfill({ json: { username: 'example', wishes: [publicWish] } }) })
    await page.route(`**/u/example/wishlist/${listID}`, route => route.fulfill({ contentType: 'text/html', body: indexHTML }))
    await page.goto(`${origin}/my/lists/${listID}`)
    await page.getByRole('button', { name: texts[lang]['lists.markReceived'], exact: true }).click()
    const dialog = page.getByRole('dialog', { name: texts[lang]['lists.markReceived'] })
    await dialog.getByRole('button', { name: texts[lang]['lists.markReceived'], exact: true }).click()
    await page.getByRole('button', { name: texts[lang]['lists.undoReceived'], exact: true }).waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/owner`)
    if (lang === 'ru' && [390, 1440].includes(width)) await page.screenshot({ path: new URL(`../../.cache/received-screens/owner-${width}.png`, import.meta.url).pathname.replace(/^\/(\w:)/, '$1'), fullPage: true })
    await page.goto(`${origin}/u/example/wishlist/${listID}`)
    await page.getByText(texts[lang]['lists.received'], { exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: texts[lang]['reservation.reserve'], exact: true }).count(), 0)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/guest`)
    await page.goto(`${origin}/my/lists/${listID}`)
    await page.getByRole('button', { name: texts[lang]['lists.undoReceived'], exact: true }).click()
    await page.getByRole('button', { name: texts[lang]['lists.markReceived'], exact: true }).waitFor()
    await page.goto(`${origin}/u/example/wishlist/${listID}`)
    await page.getByRole('button', { name: texts[lang]['reservation.reserve'], exact: true }).waitFor()
    await page.goto(`${origin}/u/example/wishes`)
    await page.getByRole('heading', { name: 'Gift certificate', exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: texts[lang]['reservation.reserve'], exact: true }).count(), 0)
    assert.equal(await page.getByText(texts[lang]['lists.received'], { exact: true }).count(), 0)
    await context.close()
  }
  assert.deepEqual(errors, [])
  console.log('Received and collection: EN/RU/RO × 320/390/768/1024/1440 passed.')
} finally { await browser.close() }
