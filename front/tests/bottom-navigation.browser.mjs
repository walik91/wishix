import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5173'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const sections = ['/my/wishes', '/my/lists', '/my/search', '/my']
const widths = [320, 340, 390, 768, 1024, 1440]
const safeBottom = 34

async function preparePage(page) {
  await page.addInitScript(() => localStorage.setItem('wishix_analytics_consent_v1', 'denied'))
  await page.route(origin + '/api/**', route => {
    const path = new URL(route.request().url()).pathname
    if (path === '/api/auth/me') return route.fulfill({ json: { user: { username: 'tester', email: 'test@example.com' } } })
    if (path === '/api/wishes') return route.fulfill({ json: { wishes: [] } })
    if (path === '/api/lists') return route.fulfill({ json: { lists: [] } })
    if (path.startsWith('/api/social/tester/people/')) return route.fulfill({ json: { people: [] } })
    return route.fulfill({ status: 404, json: { error: 'not_found' } })
  })
}

async function checkMobileNavigation(page, navigation, label) {
  const bounds = await navigation.locator('.nav-item:visible').evaluateAll(items => items.map(item => {
    const icon = item.querySelector('svg').getBoundingClientRect()
    const text = item.querySelector('span').firstChild
    const range = document.createRange()
    const words = [...text.data.matchAll(/\S+/gu)].map(word => {
      range.setStart(text, word.index)
      range.setEnd(text, word.index + word[0].length)
      return { value: word[0], boxes: [...range.getClientRects()].map(({ left, right, top }) => ({ left, right, top })) }
    })
    const { left, right, width, height } = item.getBoundingClientRect()
    return { left, right, width, height, iconTop: icon.top, fontSize: parseFloat(getComputedStyle(item).fontSize), words }
  }))
  assert.equal(bounds.length, 5, label)
  for (const item of bounds) {
    assert.ok(item.width >= 44 && item.height >= 44, label + ' touch target')
    assert.ok(item.fontSize >= 12, label + ' readable labels')
    assert.equal(item.iconTop, bounds[0].iconTop, label + ' aligned icons')
    assert.equal(item.words[0].boxes[0].top, bounds[0].words[0].boxes[0].top, label + ' aligned labels')
    for (const word of item.words) {
      assert.equal(word.boxes.length, 1, label + ' whole word ' + word.value)
      assert.ok(word.boxes[0].left >= item.left - 1 && word.boxes[0].right <= item.right + 1, label + ' fits ' + word.value)
    }
  }
  const footerAction = page.locator('.site-footer').locator('a:visible, button:visible').last()
  await footerAction.evaluate(element => {
    window.scrollTo(0, document.documentElement.scrollHeight)
    element.focus({ preventScroll: true })
  })
  const footerReachable = await footerAction.evaluate(element => {
    const { x, y, width, height, bottom } = element.getBoundingClientRect()
    const barTop = document.querySelector('.bottom-navigation').getBoundingClientRect().top
    return bottom <= barTop && element.contains(document.elementFromPoint(x + width / 2, y + height / 2))
  })
  assert.equal(footerReachable, true, label + ' footer action is reachable')
}

try {
  for (const locale of ['en', 'ru', 'ro']) {
    const messages = JSON.parse(await readFile(new URL(`../src/locales/${locale}.json`, import.meta.url)))
    for (const width of widths) {
      const mobile = width < 1024
      const context = await browser.newContext({ locale, viewport: { width, height: 844 }, isMobile: mobile, hasTouch: mobile })
      const page = await context.newPage()
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      await preparePage(page)
      if (mobile) {
        const session = await context.newCDPSession(page)
        await session.send('Emulation.setSafeAreaInsetsOverride', { insets: { bottom: safeBottom } })
      }

      await page.goto(origin + '/my')
      const navigation = page.locator(mobile ? '.bottom-navigation nav' : '.sidebar nav')
      const profile = navigation.locator('button')
      await profile.waitFor()
      await page.evaluate(() => document.fonts.ready)
      const label = `${locale}/${width}`
      assert.equal(await navigation.getAttribute('aria-label'), messages['my.navigation'], label)
      const expectedKeys = ['auth.my', 'my.myWishes', 'lists.nav', 'my.search']
      if (!mobile) expectedKeys.push('my.following')
      expectedKeys.push('my.profile')
      assert.deepEqual(await navigation.locator('.nav-item:visible').allTextContents(), expectedKeys.map(key => messages[key]), label)
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, label)
      if (mobile) {
        assert.equal(await page.locator('.bottom-navigation').evaluate(bar => parseFloat(getComputedStyle(bar).paddingBottom)), safeBottom, label)
        await checkMobileNavigation(page, navigation, label)
      } else {
        const sidebar = page.locator('.sidebar')
        assert.equal((await sidebar.boundingBox()).width, 240, label)
        assert.deepEqual(await navigation.locator('.nav-group').evaluateAll(groups => groups.map(group =>
          [...group.querySelectorAll('.nav-item')].map(item => item.textContent)
        )), [expectedKeys.slice(0, 3), expectedKeys.slice(3)].map(group => group.map(key => messages[key])), label)
        assert.equal(await sidebar.getByRole('button').count(), 1, label)
        const header = page.locator('.app-header')
        assert.equal(await header.getByRole('combobox', { name: messages.language }).inputValue(), locale, label)
        const avatar = header.getByRole('button', { name: messages['my.openAccount'] })
        await avatar.click()
        await page.locator('.profile-dialog[open]').waitFor()
        assert.equal(await avatar.getAttribute('aria-expanded'), 'true', label)
        await page.keyboard.press('Escape')
        await navigation.locator('button[aria-expanded="false"]').waitFor()
      }

      if (width === 340 || width === 390 || width === 1440) {
        for (const section of sections) {
          await navigation.locator(`a[href="${section}"]`).click()
          await page.waitForURL(origin + section)
          assert.equal(await navigation.locator('.active[aria-current="page"]').getAttribute('href'), section)
          if (mobile) await checkMobileNavigation(page, navigation, `${label} ${section}`)
        }
        await profile.click()
        const dialog = page.locator('.profile-dialog[open]')
        await dialog.waitFor()
        assert.equal(await profile.getAttribute('aria-expanded'), 'true')
        assert.ok(await profile.evaluate(button => button.classList.contains('active')))
        const following = mobile ? dialog : navigation
        if (!mobile) {
          await page.keyboard.press('Escape')
          await navigation.locator('button[aria-expanded="false"]').waitFor()
        }
        await following.locator('a[href="/my/following"]').click()
        await page.waitForURL(origin + '/my/following')
        if (!mobile) assert.equal(await navigation.locator('.active[aria-current="page"]').getAttribute('href'), '/my/following')
      }
      assert.deepEqual(errors, [], label)
      await context.close()
    }
  }

  for (const width of [390, 1440]) {
    const mobile = width < 1024
    const page = await browser.newPage({ viewport: { width, height: 844 }, isMobile: mobile, hasTouch: mobile })
    await preparePage(page)
    const navigation = page.locator(mobile ? '.bottom-navigation nav' : '.sidebar nav')
    for (const [path, active] of [
      ['/my/wishes/new', '/my/wishes'],
      ['/my/wishes/wish-1/edit', '/my/wishes'],
      ['/my/lists/new', '/my/lists'],
      ['/my/lists/list-1', '/my/lists'],
      ['/my/search/', '/my/search'],
    ]) {
      await page.goto(origin + path)
      assert.equal(await navigation.locator('.active[aria-current="page"]').getAttribute('href'), active)
    }
    await page.goto(origin + '/my/profile')
    await page.locator('.profile-dialog[open]').waitFor()
    assert.equal(await navigation.locator('button.active').getAttribute('aria-expanded'), 'true')
    await page.close()
  }
  console.log('Passed: localized navigation, mobile safe area and footer, desktop groups, profile and route states.')
} finally {
  await browser.close()
}
