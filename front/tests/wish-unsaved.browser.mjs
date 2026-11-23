// Run against local Vite. API calls are mocked.
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { join } from 'node:path'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ channel: 'chrome', ignoreDefaultArgs: ['--disable-back-forward-cache'] })
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5174'
const newPath = '/my/wishes/new'
const editPath = '/my/wishes/wish-id/edit'
const list = { id: 'birthday', title: 'Birthday', visibility: 'unlisted' }
const wish = { title: 'Existing wish', description: 'Notes', price: '12', currency: 'EUR', priority: 'high', shop_links: ['https://shop.test/item'], edit_version: 'version' }

async function setup(locale = 'en', width = 390) {
  const context = await browser.newContext({ locale, viewport: { width, height: width === 320 ? 640 : 900 } })
  await context.addInitScript(() => localStorage.setItem('wishix_analytics_consent_v1', 'denied'))
  await context.route(`${origin}/api/**`, route => {
    const path = new URL(route.request().url()).pathname
    const mock = {
      '/api/auth/me': { user: { id: 'owner', username: 'tester' } },
      '/api/lists': { lists: [list] },
      '/api/wishes': { wishes: [] },
      '/api/wishes/wish-id': { wish },
    }
    return mock[path] ? route.fulfill({ json: mock[path] }) : route.abort()
  })
  const page = await context.newPage()
  const messages = JSON.parse(await readFile(new URL(`../src/locales/${locale}.json`, import.meta.url)))
  const dialog = page.getByRole('dialog', { name: messages['discard.title'] })
  return {
    context, page, dialog, messages,
    title: page.locator('#wish-title'),
    menu: page.locator('.app-navigation a[href="/my"]:visible'),
    cancel: page.getByRole('link', { name: messages['wishes.cancel'], exact: true }),
    keep: dialog.getByRole('button', { name: messages['discard.continue'] }),
    leave: dialog.getByRole('button', { name: messages['discard.leave'] }),
  }
}

try {
  for (const locale of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const { context, page, title, menu, cancel, dialog, keep, leave } = await setup(locale, width)
    await page.goto(`${origin}${newPath}`)
    await title.fill('Typed wish')
    await menu.click()
    await dialog.waitFor()
    assert.equal(new URL(page.url()).pathname, newPath)
    const bounds = await dialog.boundingBox()
    assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= width, `${locale} ${width}: dialog overflow`)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false)
    if (locale === 'ru' && [390, 1440].includes(width) && process.env.WISHIX_TEST_SCREENSHOTS) {
      await page.screenshot({ path: join(process.env.WISHIX_TEST_SCREENSHOTS, `wish-discard-${width}.png`) })
    }
    await keep.click()
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(await title.inputValue(), 'Typed wish')
    await cancel.click()
    await dialog.waitFor()
    await page.keyboard.press('Escape')
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(await title.inputValue(), 'Typed wish')
    await cancel.click()
    await leave.click()
    await page.waitForURL(`${origin}/my/wishes`)
    await page.goBack({ waitUntil: 'commit' })
    await page.waitForFunction(() => document.querySelector('#wish-title')?.value === '')
    await context.close()
  }

  {
    const { context, page, title, menu, cancel, dialog, messages } = await setup()
    await page.goto(`${origin}${newPath}?list=${list.id}`)
    await page.getByRole('button', { name: messages['lists.wishLists'], exact: true }).click()
    const selection = page.getByRole('checkbox', { name: /Birthday/ })
    await selection.waitFor()
    assert.equal(await selection.isChecked(), true)
    await selection.uncheck()
    await menu.click()
    await dialog.waitFor()
    await page.keyboard.press('Escape')
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(await selection.isChecked(), false)
    await selection.check()
    await title.fill('Reverted title')
    await title.fill('')
    await page.getByRole('button', { name: messages['wishes.addLink'], exact: true }).click()
    await cancel.click()
    await page.waitForURL(`${origin}/my/lists/${list.id}`)

    await page.goto(`${origin}${editPath}`)
    await page.locator('#wish-price').fill('13.50')
    await cancel.click()
    await dialog.waitFor()
    await page.keyboard.press('Escape')
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(await page.locator('#wish-price').inputValue(), '13.50')
    await page.locator('#wish-price').fill(wish.price)
    await menu.click()
    await page.waitForURL(`${origin}/my`)
    await context.close()
  }

  {
    const { context, page, title, messages } = await setup()
    await page.goto(`${origin}/my`)
    await page.getByRole('link', { name: messages['my.addWish'], exact: true }).first().click()
    await title.pressSequentially('Keep on Back')
    const session = await context.newCDPSession(page)
    const { entries, currentIndex } = await session.send('Page.getNavigationHistory')
    const entryId = entries[currentIndex - 1].id
    for (const leavePage of [false, true]) {
      const response = page.waitForEvent('dialog').then(async dialog => {
        assert.equal(dialog.type(), 'beforeunload')
        await (leavePage ? dialog.accept() : dialog.dismiss())
      })
      await Promise.all([session.send('Page.navigateToHistoryEntry', { entryId }), response])
      if (!leavePage) {
        assert.equal(new URL(page.url()).pathname, newPath)
        assert.equal(await title.inputValue(), 'Keep on Back')
      }
    }
    await page.waitForURL(`${origin}/my`, { waitUntil: 'commit' })
    await page.goForward({ waitUntil: 'commit' })
    await page.waitForFunction(() => document.querySelector('#wish-title')?.value === '')
    await page.goBack({ waitUntil: 'commit' })
    await page.waitForURL(`${origin}/my`, { waitUntil: 'commit' })
    await context.close()
  }
  console.log('Wish discard: 15 locale and viewport combinations, restored defaults, and browser history passed')
} finally { await browser.close() }
