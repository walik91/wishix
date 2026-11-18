import assert from 'node:assert/strict'
import { readFile, mkdir } from 'node:fs/promises'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5173'
const texts = Object.fromEntries(await Promise.all(['en', 'ru', 'ro'].map(async lang => [lang, JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))])))
const indexHTML = await readFile(new URL('../index.html', import.meta.url), 'utf8')
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const errors = []
try {
  await mkdir(new URL('../../.cache/profile-screens/', import.meta.url), { recursive: true })
  for (const lang of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const context = await browser.newContext({ locale: lang, viewport: { width, height: 900 } })
    const page = await context.newPage()
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    let enabled = false
    await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { username: 'example', email: 'private@example.test' } } }))
    await page.route('**/u/example', route => route.fulfill({ contentType: 'text/html', body: indexHTML }))
    await page.route('**/api/auth/google/status', route => route.fulfill({ json: { enabled: false } }))
    await page.route('**/api/auth/facebook/status', route => route.fulfill({ json: { enabled: false } }))
    await page.route('**/api/profile/name', route => route.fulfill({ json: { display_name: '', show_display_name: false } }))
    await page.route('**/api/wishes', route => route.fulfill({ json: { wishes: [] } }))
    await page.route('**/api/lists', route => route.fulfill({ json: { lists: [] } }))
    await page.route('**/api/profile/collection', route => {
      if (route.request().method() === 'PUT') enabled = route.request().postDataJSON().show_all_wishes
      return route.fulfill({ json: { show_all_wishes: enabled } })
    })
    await page.route('**/api/public/profiles/example', route => route.fulfill({ json: { username: 'example', show_all_wishes: enabled, lists: [{ id: '00000000-0000-4000-8000-000000000001', title: 'Public list', visibility: 'public', wish_count: 1, share_path: '/00000000-0000-4000-8000-000000000001' }] } }))
    await page.route('**/api/public/profiles/example/details', route => route.fulfill({ json: { username: 'example', has_avatar: false, show_followers: false, show_following: false } }))
    await page.route('**/api/social/example/state', route => route.fulfill({ json: { self: true, following: false, blocked: false, can_follow: false } }))
    await page.route('**/api/public/profiles/example/wishes', route => enabled ? route.fulfill({ json: { username: 'example', wishes: [{ id: 'wish', title: 'A very long wish '.repeat(8), description: '', price: null, currency: '', priority: 'medium', shop_links: [], has_image: false }] } }) : route.fulfill({ status: 404, json: { error: 'list_not_found' } }))
    await page.goto(`${origin}/u/example`)
    await page.getByRole('heading', { name: 'example' }).waitFor()
    assert.equal(await page.getByRole('link', { name: texts[lang]['profile.allWishes'] }).count(), 0)
    await page.goto(`${origin}/my`)
    await page.getByRole('button', { name: texts[lang]['my.openAccount'] }).first().click()
    const checkbox = page.getByRole('checkbox', { name: texts[lang]['profile.showAll'] })
    let saving = page.waitForResponse(response => response.url().endsWith('/api/profile/collection') && response.request().method() === 'PUT')
    await checkbox.check()
    await saving
    await page.goto(`${origin}/u/example`)
    await page.getByRole('link', { name: texts[lang]['profile.allWishes'] }).waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/profile`)
    if (lang === 'ru' && [390, 1440].includes(width)) await page.screenshot({ path: new URL(`../../.cache/profile-screens/profile-${width}.png`, import.meta.url).pathname.replace(/^\/(\w:)/, '$1'), fullPage: true })
    await page.getByRole('link', { name: texts[lang]['profile.allWishes'] }).click()
    await page.getByRole('heading', { name: texts[lang]['profile.allWishes'], exact: true }).waitFor()
    await page.getByText('A very long wish', { exact: false }).first().waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}`)
    if (lang === 'ru' && [390, 1440].includes(width)) await page.screenshot({ path: new URL(`../../.cache/profile-screens/collection-${width}.png`, import.meta.url).pathname.replace(/^\/(\w:)/, '$1'), fullPage: true })
    await page.goto(`${origin}/my`)
    await page.getByRole('button', { name: texts[lang]['my.openAccount'] }).first().click()
    await page.waitForFunction(() => [...document.querySelectorAll('.collection-setting input')].some(input => input.checked && !input.disabled))
    saving = page.waitForResponse(response => response.url().endsWith('/api/profile/collection') && response.request().method() === 'PUT')
    await page.getByRole('checkbox', { name: texts[lang]['profile.showAll'] }).uncheck()
    await saving
    await page.goto(`${origin}/u/example/wishes`)
    await page.getByRole('heading', { name: texts[lang]['lists.sharedUnavailable'] }).waitFor()
    await context.close()
  }
  assert.deepEqual(errors, [])
  console.log('Public profile and collection: EN/RU/RO × 320/390/768/1024/1440 passed.')
} finally { await browser.close() }
