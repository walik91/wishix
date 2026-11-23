import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { join } from 'node:path'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5173'
const screenshotDir = process.env.WISHIX_DASHBOARD_SCREENSHOTS
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const photo = await readFile(new URL('../src/assets/examples/wish-camera.png', import.meta.url), 'base64')
const wish = { id: 'wish-1', title: 'Fujifilm X-T30 II', priority: 'medium', price: null, currency: '', shop_links: [], image_url: `data:image/png;base64,${photo}`, description: '' }
const listTitles = ['Birthday', 'Things for home', 'Travel', 'New Year']
const lists = listTitles.map((title, index) => ({ id: `list-${index + 1}`, title, event_date: null, visibility: 'link', wish_count: index + 1 }))
const wishes = [wish, { ...wish, id: 'wish-2', title: 'Headphones', image_url: null }, { ...wish, id: 'wish-3', title: 'Book', image_url: null }, { ...wish, id: 'wish-4' }]
const states = [
  { name: 'empty', lists: [], wishes: [] },
  { name: 'first-wish', lists: [], wishes: [wish] },
  { name: 'lists-and-wishes', lists, wishes },
  { name: 'lists-only', lists: [{ ...lists[0], title: 'LongListTitle'.repeat(8), event_date: '2026-11-01', visibility: 'private', wish_count: 0, expired: true }], wishes: [] },
]
const widths = [[320, 640], [390, 844], [768, 1024], [1024, 768], [1440, 900]]
const sectionOrder = (page) => page.locator('.dashboard-section').evaluateAll(sections =>
  sections.map(section => section.getAttribute('aria-labelledby')),
)

async function mockDashboard(page, state) {
  await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { username: 'anna', email: 'test@example.com' } } }))
  await page.route('**/api/lists', route => route.fulfill({ json: { lists: state.lists } }))
  await page.route('**/api/wishes', route => route.fulfill({ json: { wishes: state.wishes } }))
}

async function checkCards(page, grid, card, width, label) {
  const columns = width < 768 ? 1 : width < 1200 ? 2 : 3
  assert.equal(await page.locator(grid).evaluate(element => getComputedStyle(element).gridTemplateColumns.split(' ').length), columns, label)
  if (width < 768) {
    const bounds = await page.locator(card).evaluateAll(elements => elements.map(element => {
      const { x, y, width } = element.getBoundingClientRect()
      const thumbnail = element.querySelector('.list-cover, .wish-image').getBoundingClientRect()
      const title = element.querySelector('.list-card-title, .wish-title').getBoundingClientRect()
      return { x, y, width, thumbnailRight: thumbnail.right, titleLeft: title.left }
    }))
    for (const [index, row] of bounds.entries()) {
      assert.equal(row.x, bounds[0].x, label)
      assert.equal(row.width, bounds[0].width, label)
      assert.ok(row.thumbnailRight <= row.titleLeft, label)
      if (index) assert.ok(row.y > bounds[index - 1].y, label)
    }
  }
}

try {
  for (const locale of ['en', 'ru', 'ro']) {
    const messages = JSON.parse(await readFile(new URL(`../src/locales/${locale}.json`, import.meta.url)))
    for (const [width, height] of widths) {
      const context = await browser.newContext({ locale, viewport: { width, height } })
      await context.addInitScript(() => localStorage.setItem('wishix_analytics_consent_v1', 'denied'))
      const page = await context.newPage()
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })

      for (const state of states) {
        const label = `${locale}/${width}/${state.name}`
        await mockDashboard(page, state)
        await page.goto(`${origin}/my`)
        const dashboard = page.locator('.dashboard')
        const createList = dashboard.getByRole('link', { name: messages['lists.create'], exact: true })
        await createList.waitFor()
        await page.locator(state.wishes.length ? '.wish-preview' : '.wishes-empty').first().waitFor()
        await page.evaluate(() => document.fonts.ready)

        const order = ['dashboard-wishes-title', 'dashboard-lists-title']
        if (state.lists.length) order.reverse()
        assert.deepEqual(await sectionOrder(page), order, label)
        assert.deepEqual(await dashboard.locator('.list-card-title').allTextContents(), state.lists.slice(0, 3).map(list => list.title), label)
        assert.deepEqual(await dashboard.locator('.wish-title').allTextContents(), state.wishes.slice(0, 3).map(wish => wish.title), label)
        assert.equal(await dashboard.getByRole('heading', { name: messages['dashboard.recentWishes'] }).count(), 1, label)
        assert.equal(await dashboard.locator('.hero:visible').count(), width >= 1024 && state.name === 'empty' ? 1 : 0, label)
        assert.equal(await createList.count(), 1, label)
        assert.equal(await createList.getAttribute('href'), '/my/lists/new', label)
        assert.equal(await createList.evaluate(link => link.closest('section').getAttribute('aria-labelledby')), 'dashboard-lists-title', label)
        assert.equal((await createList.boundingBox()).height, 44, label)

        const addWish = dashboard.getByRole('link', { name: messages['my.addWish'], exact: true })
        const greeting = await dashboard.locator('.greeting > div').boundingBox()
        const action = await addWish.boundingBox()
        assert.equal(await addWish.count(), 1, label)
        assert.ok(action.height >= 44, label)
        if (width < 768) {
          assert.ok(action.y >= greeting.y + greeting.height, label)
          assert.equal(action.width, greeting.width, label)
        } else if (width >= 1024) {
          assert.equal(action.y, greeting.y, label)
          assert.ok(action.x >= greeting.x + greeting.width, label)
        }

        if (state.lists.length) await checkCards(page, '.dashboard .list-grid', '.list-card', width, label)
        if (state.wishes.length) await checkCards(page, '.wish-preview-grid', '.wish-preview', width, label)
        const overflow = await dashboard.evaluate(element => {
          const width = document.documentElement.clientWidth
          return document.documentElement.scrollWidth > width || [...element.querySelectorAll('*')].some(child => {
            const { left, right } = child.getBoundingClientRect()
            return left < -1 || right > width + 1
          })
        })
        assert.equal(overflow, false, label)

        if (screenshotDir && locale === 'ru' && [390, 1440].includes(width) && state.name !== 'lists-only') {
          await page.screenshot({ path: join(screenshotDir, `${state.name}-${width}.png`), fullPage: true })
        }
        if (width === 390 && state.name === 'first-wish') {
          await createList.focus()
          await createList.press('Enter')
          await page.waitForURL('**/my/lists/new')
          await page.goBack()
          await addWish.focus()
          await addWish.press('Enter')
          await page.waitForURL('**/my/wishes/new')
          await page.locator('#wish-title').waitFor()
          await page.locator('.app-navigation:visible a[href="/my/wishes"]').click()
          await page.locator('[data-wish-id="wish-1"]').waitFor()
          assert.equal(await page.locator('.app-navigation:visible a[aria-current="page"]').getAttribute('href'), '/my/wishes')
        }
        if (width === 1440 && state.name === 'lists-and-wishes') {
          await dashboard.getByRole('link', { name: messages['dashboard.allLists'], exact: true }).click()
          await page.waitForURL('**/my/lists')
          await page.locator('[data-list-id="list-4"]').waitFor()
        }
        await page.unrouteAll({ behavior: 'wait' })
      }
      assert.deepEqual(errors, [], `${locale}/${width}`)
      await context.close()
    }
  }

  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'en' })
  await page.addInitScript(() => localStorage.setItem('wishix_analytics_consent_v1', 'denied'))
  let finishLoadingLists
  const listsPending = new Promise(resolve => { finishLoadingLists = resolve })
  let listsFail = true
  await mockDashboard(page, states[0])
  await page.unroute('**/api/lists')
  await page.route('**/api/lists', async route => {
    await listsPending
    if (listsFail) await route.fulfill({ status: 500, json: { error: 'unavailable' } })
    else await route.fulfill({ json: { lists: [] } })
  })
  await page.goto(`${origin}/my`)
  await page.locator('.wishes-empty').waitFor()
  const listsSection = page.locator('[aria-labelledby="dashboard-lists-title"]')
  await listsSection.getByRole('status').waitFor()
  assert.equal(await page.locator('.hero').count(), 0)
  await page.locator('.greeting a[href="/my/wishes/new"]').waitFor()
  finishLoadingLists()
  await listsSection.locator('.dashboard-error').waitFor()
  assert.equal(await page.locator('.hero').count(), 0)
  listsFail = false
  await listsSection.getByRole('button').click()
  await page.locator('.hero').waitFor()
  await page.unroute('**/api/wishes')
  await page.route('**/api/wishes', route => route.fulfill({ status: 500, json: { error: 'unavailable' } }))
  await page.reload()
  await page.locator('[aria-labelledby="dashboard-wishes-title"] .dashboard-error').waitFor()
  await listsSection.getByRole('link', { name: 'Create a list' }).waitFor()
  assert.equal(await page.locator('.hero').count(), 0)
  await page.close()
  console.log('Passed: dashboard state, section order, localized responsive layouts, navigation, loading and retry.')
} finally {
  await browser.close()
}
