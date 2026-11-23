import assert from 'node:assert/strict'
import { readFile, mkdir } from 'node:fs/promises'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5173'
const texts = Object.fromEntries(await Promise.all(['en', 'ru', 'ro'].map(async lang => [lang, JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))])))
const indexHTML = await readFile(new URL('../index.html', import.meta.url), 'utf8')
const publicList = (number, title, extra = {}) => {
  const id = `00000000-0000-4000-8000-00000000000${number}`
  return { id, title, visibility: 'public', wish_count: 1, share_path: `/${id}`, ...extra }
}
const publicLists = [
  publicList(1, 'Public list'),
  publicList(2, 'ДлинноеНазваниеСписка'.repeat(4), { event_date: '2025-12-31', expired: true, wish_count: 0 }),
]
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const errors = []
try {
  await mkdir(new URL('../../.cache/profile-screens/', import.meta.url), { recursive: true })
  for (const lang of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const context = await browser.newContext({ locale: lang, viewport: { width, height: 900 } })
    await context.addInitScript(() => localStorage.setItem('wishix_analytics_consent_v1', 'denied'))
    const page = await context.newPage()
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    let enabled = false
    const collectionWrites = []
    await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { username: 'example', email: 'private@example.test' } } }))
    await page.route('**/u/example', route => route.fulfill({ contentType: 'text/html', body: indexHTML }))
    await page.route('**/api/auth/google/status', route => route.fulfill({ json: { enabled: false } }))
    await page.route('**/api/auth/facebook/status', route => route.fulfill({ json: { enabled: false } }))
    await page.route('**/api/profile/name', route => route.fulfill({ json: { display_name: 'Example Name' } }))
    await page.route('**/api/profile/details', route => route.fulfill({ json: {
      username: 'example', biography: '', birth_date: null, has_avatar: false, show_age: false,
      show_followers: false, show_following: false,
    } }))
    await page.route('**/api/wishes', route => route.fulfill({ json: { wishes: [] } }))
    await page.route('**/api/lists', route => route.fulfill({ json: { lists: [] } }))
    await page.route('**/api/profile/collection', route => {
      if (route.request().method() === 'PUT') {
        enabled = route.request().postDataJSON().show_all_wishes
        collectionWrites.push(enabled)
      }
      return route.fulfill({ json: { show_all_wishes: enabled } })
    })
    await page.route('**/api/public/profiles/example', route => route.fulfill({ json: { username: 'example', display_name: 'Example Name', show_all_wishes: enabled, lists: publicLists } }))
    await page.route('**/api/public/profiles/example/details', route => route.fulfill({ json: { username: 'example', has_avatar: false, show_followers: false, show_following: false } }))
    await page.route('**/api/social/example/state', route => route.fulfill({ json: { self: true, following: false, blocked: false, can_follow: false } }))
    await page.route('**/api/public/profiles/example/wishes', route => enabled ? route.fulfill({ json: { username: 'example', wishes: [{ id: 'wish', title: 'A very long wish '.repeat(8), description: '', price: null, currency: '', priority: 'medium', shop_links: [], has_image: false }] } }) : route.fulfill({ status: 404, json: { error: 'list_not_found' } }))
    await page.goto(`${origin}/u/example`)
    const heading = page.getByRole('heading', { name: 'Example Name @example', exact: true })
    await heading.getByText('@example', { exact: true }).waitFor()
    for (const list of publicLists) {
      const card = page.locator('.list-grid').getByRole('link').filter({ hasText: list.title })
      await card.getByRole('heading', { name: list.title, exact: true, level: 3 }).waitFor()
      assert.equal(await card.getAttribute('href'), `/u/example/wishlist/${list.id}`)
      assert.equal(await card.getByText(`${texts[lang]['lists.wishesCount']} ${list.wish_count}`, { exact: true }).isVisible(), true)
      assert.equal(await card.locator('time').count(), list.event_date ? 1 : 0)
      if (list.event_date) {
        const date = new Intl.DateTimeFormat(lang, { dateStyle: 'medium', timeZone: 'UTC' }).format(new Date(list.event_date))
        assert.equal(await card.locator('time').textContent(), date)
        assert.equal(await card.locator('time').getAttribute('datetime'), list.event_date)
      }
      assert.ok((await card.boundingBox()).height >= 44)
    }
    assert.equal(await page.locator('.list-grid .list-card-footer').count(), 0)
    assert.equal(await page.locator('.list-grid').getByText(texts[lang]['lists.expired'], { exact: true }).count(), 0)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/public cards`)
    assert.equal(await page.locator('.profile-header').getByText(texts[lang]['profile.age'], { exact: false }).count(), 0)
    assert.equal(await page.getByRole('link', { name: texts[lang]['profile.allWishes'] }).count(), 0)
    await page.goto(`${origin}/my`)
    await page.locator('.app-navigation:visible').getByRole('button', { name: texts[lang]['my.profile'], exact: true }).click()
    const visibility = page.getByRole('region', { name: texts[lang]['profile.visibility'] })
    for (const key of ['profile.showAge', 'profile.showFollowers', 'profile.showFollowing', 'profile.showAll']) {
      await visibility.getByRole('checkbox', { name: texts[lang][key] }).waitFor()
    }
    const age = visibility.getByRole('checkbox', { name: texts[lang]['profile.showAge'] })
    assert.equal(await age.isDisabled(), true)
    await visibility.getByText(texts[lang]['profile.ageHint'], { exact: true }).waitFor()
    await page.getByText(texts[lang]['profile.birthDateHint'], { exact: true }).waitFor()
    const checkbox = visibility.getByRole('checkbox', { name: texts[lang]['profile.showAll'] })
    assert.equal(await checkbox.isChecked(), false)
    await checkbox.click()
    const publish = page.getByRole('dialog', { name: texts[lang]['profile.publishCollection'] })
    await publish.getByText(texts[lang]['profile.showAllWarning'], { exact: true }).waitFor()
    assert.deepEqual(collectionWrites, [])
    const cancel = publish.getByRole('button', { name: texts[lang]['wishes.cancel'], exact: true })
    assert.equal(await cancel.evaluate(button => button === document.activeElement), true)
    await cancel.click()
    await publish.waitFor({ state: 'hidden' })
    assert.equal(await checkbox.isChecked(), false)
    assert.equal(await checkbox.evaluate(input => input === document.activeElement), true)
    assert.deepEqual(collectionWrites, [])
    const settings = page.locator('dialog.profile-dialog')
    assert.equal(await settings.locator('.profile-scroll').evaluate(scroll => scroll.scrollWidth > scroll.clientWidth), false, `${lang}/${width}/settings`)
    if (lang === 'ru' && [390, 1440].includes(width)) {
      await visibility.evaluate(section => section.scrollIntoView({ block: 'center' }))
      await page.screenshot({ path: new URL(`../../.cache/profile-screens/visibility-${width}.png`, import.meta.url).pathname.replace(/^\/(\w:)/, '$1') })
    }
    await checkbox.focus()
    await page.keyboard.press('Space')
    await publish.waitFor()
    await page.keyboard.press('Escape')
    await publish.waitFor({ state: 'hidden' })
    assert.equal(await checkbox.isChecked(), false)
    assert.deepEqual(collectionWrites, [])
    await checkbox.click()
    if (lang === 'ru' && [390, 1440].includes(width)) {
      await publish.waitFor()
      await page.screenshot({ path: new URL(`../../.cache/profile-screens/publish-${width}.png`, import.meta.url).pathname.replace(/^\/(\w:)/, '$1') })
    }
    let saving = page.waitForResponse(response => response.url().endsWith('/api/profile/collection') && response.request().method() === 'PUT')
    await publish.getByRole('button', { name: texts[lang]['profile.publishCollection'], exact: true }).click()
    await saving
    assert.deepEqual(collectionWrites, [true])
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
    await page.locator('.app-navigation:visible').getByRole('button', { name: texts[lang]['my.profile'], exact: true }).click()
    await page.waitForFunction(() => [...document.querySelectorAll('.collection-setting input')].some(input => input.checked && !input.disabled))
    saving = page.waitForResponse(response => response.url().endsWith('/api/profile/collection') && response.request().method() === 'PUT')
    await checkbox.uncheck()
    await saving
    assert.deepEqual(collectionWrites, [true, false])
    assert.equal(await publish.isVisible(), false)
    await page.goto(`${origin}/u/example/wishes`)
    await page.getByRole('heading', { name: texts[lang]['lists.sharedUnavailable'] }).waitFor()
    await context.close()
  }
  assert.deepEqual(errors, [])
  console.log('Public profile and collection: EN/RU/RO × 320/390/768/1024/1440 passed.')
} finally { await browser.close() }
