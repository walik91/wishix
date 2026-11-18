import assert from 'node:assert/strict'
import { readFile, mkdir } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5173'
const texts = Object.fromEntries(await Promise.all(['en', 'ru', 'ro'].map(async lang => [lang, JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))])))
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const errors = []
await mkdir(new URL('../../.cache/search-screens/', import.meta.url), { recursive: true })
try {
  for (const lang of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const context = await browser.newContext({ locale: lang, viewport: { width, height: 900 } })
    const page = await context.newPage()
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    let name = { display_name: '', show_display_name: false }
    await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { username: 'example', email: 'owner@example.test' } } }))
    await page.route('**/api/auth/google/status', route => route.fulfill({ json: { enabled: false } }))
    await page.route('**/api/auth/facebook/status', route => route.fulfill({ json: { enabled: false } }))
    await page.route('**/api/profile/collection', route => route.fulfill({ json: { show_all_wishes: false } }))
    await page.route('**/api/profile/name', route => {
      if (route.request().method() === 'PUT') name = route.request().postDataJSON()
      return route.fulfill({ json: name })
    })
    await page.route('**/api/search/people?*', route => {
      const params = new URL(route.request().url()).searchParams
      const people = params.get('q') === 'nobody' ? [] : params.get('after') ? [{ username: 'another' }] : [{ username: 'alice', display_name: 'Alexandra Long Name' }]
      return route.fulfill({ json: { people, next_cursor: params.get('q') !== 'nobody' && !params.get('after') ? 'alice' : '' } })
    })
    await page.route('**/api/public/profiles/alice', route => route.fulfill({ json: { username: 'alice', display_name: 'Alexandra Long Name', show_all_wishes: false, lists: [] } }))
    await page.goto(`${origin}/my/search`)
    await page.getByRole('heading', { name: texts[lang]['search.title'] }).waitFor()
    const input = page.getByLabel(texts[lang]['search.label'])
    await input.fill('alice')
    await page.locator('.search-form').getByRole('button', { name: texts[lang]['my.search'] }).click()
    const link = page.getByRole('link', { name: /Alexandra Long Name/ })
    await link.waitFor()
    assert.equal(await link.getAttribute('href'), '/u/alice')
    await page.getByRole('button', { name: texts[lang]['search.more'] }).click()
    await page.getByRole('link', { name: 'another' }).waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/search`)
    if (lang === 'ru' && [390, 1440].includes(width)) await page.screenshot({ path: fileURLToPath(new URL(`../../.cache/search-screens/search-${width}.png`, import.meta.url)), fullPage: true })
    await input.fill('nobody')
    await page.locator('.search-form').getByRole('button', { name: texts[lang]['my.search'] }).click()
    await page.getByText(texts[lang]['search.empty'], { exact: true }).waitFor()
    await page.getByRole('button', { name: texts[lang]['my.openAccount'] }).first().click()
    const dialog = page.getByRole('dialog', { name: texts[lang]['my.profile'] })
    const display = dialog.getByLabel(texts[lang]['profile.displayName'], { exact: true })
    await display.fill('Alexandra Long Name')
    await dialog.getByRole('checkbox', { name: texts[lang]['profile.showName'] }).check()
    await dialog.getByRole('button', { name: texts[lang]['profile.saveName'] }).click()
    await dialog.getByText(texts[lang]['profile.nameSaved']).waitFor()
    assert.deepEqual(name, { display_name: 'Alexandra Long Name', show_display_name: true })
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/profile`)
    await page.goto(`${origin}/u/alice`)
    await page.getByRole('heading', { name: 'Alexandra Long Name' }).waitFor()
    await context.close()
  }
  assert.deepEqual(errors, [])
  console.log('People search and name settings: EN/RU/RO × 320/390/768/1024/1440 passed.')
} finally { await browser.close() }
