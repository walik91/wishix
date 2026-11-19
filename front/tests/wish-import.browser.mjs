// Run against a local Vite server. All API calls are intercepted; no accounts or wishes are changed.
import assert from 'node:assert/strict'
import { readFile, mkdir } from 'node:fs/promises'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ headless: true, channel: 'chrome' })
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5174'
const errors = []
const texts = {}
for (const lang of ['en', 'ru', 'ro']) texts[lang] = JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))
let checks = 0
async function setup(lang = 'en', width = 390) {
  const context = await browser.newContext({ locale: lang, viewport: { width, height: 900 } })
  const page = await context.newPage()
  page.on('pageerror', error => errors.push(error.message))
  page.on('console', message => { if (message.text().includes('[Vue warn]')) errors.push(message.text()) })
  await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { id: 'test', username: 'tester', email: 'tester@example.test' } } }))
  await page.route('**/api/lists', route => route.fulfill({ json: { lists: [] } }))
  await page.goto(`${origin}/my/wishes/new`)
  await page.locator('#wish-link-1').waitFor()
  const jpeg = await page.evaluate(() => { const c = document.createElement('canvas'); c.width = 40; c.height = 40; return c.toDataURL('image/jpeg').split(',')[1] })
  return { context, page, result: { status: 'ok', title: 'Imported product', description: 'Imported details', price: '49.95', currency: 'MDL', shop_name: 'Store', image: { mime: 'image/jpeg', data_base64: jpeg } } }
}
async function waitStatus(page, text) { await page.locator('.toast').filter({ hasText: text }).waitFor() }
function importButton(page, index = 1) { return page.getByRole('button', { name: `${texts.en['wishes.import.action']} ${index}`, exact: true }) }
try {
  // Exercise late responses, explicit clearing, and user image selection during import.
  {
    const { page, context, result } = await setup()
    let release
    const gate = new Promise(resolve => { release = resolve })
    let started
    const pending = new Promise(resolve => { started = resolve })
    await page.route('**/api/wishes/import-preview', async route => { started(); await gate; await route.fulfill({ json: result }).catch(() => {}) })
    await page.locator('#wish-title').fill('My own title')
    await page.locator('#wish-title').fill('')
    await page.locator('#wish-description').fill('My description')
    await page.locator('#wish-description').fill('')
    await page.locator('#wish-link-1').fill('https://shop.example.com/product')
    await pending
    await page.locator('#wish-price').fill('12.50')
    await page.locator('#wish-currency').fill('EUR')
    await page.locator('#wish-image').setInputFiles({ name: 'mine.jpg', mimeType: 'image/jpeg', buffer: Buffer.from(result.image.data_base64, 'base64') })
    release()
    await waitStatus(page, texts.en['wishes.import.kept'])
    assert.equal(await page.locator('#wish-title').inputValue(), '')
    assert.equal(await page.locator('#wish-description').inputValue(), '')
    assert.equal(await page.locator('#wish-price').inputValue(), '12.50')
    assert.equal(await page.locator('#wish-currency').inputValue(), 'EUR')
    assert.equal(await page.locator('.file-name').innerText(), 'mine.jpg')
    checks++; await context.close()
  }
  // Abort a stale response, import a changed first link explicitly, and preserve manual edits.
  {
    const { page, context, result } = await setup()
    let release
    const gate = new Promise(resolve => { release = resolve })
    const urls = []
    let started
    const pending = new Promise(resolve => { started = resolve })
    await page.route('**/api/wishes/import-preview', async route => {
      const { url } = route.request().postDataJSON(); urls.push(url)
      if (urls.length === 1) { started(); await gate }
      await route.fulfill({ json: { ...result, title: url.endsWith('second') ? 'Second product' : 'Stale product' } }).catch(() => {})
    })
    await page.locator('#wish-link-1').fill('https://shop.example.com/first'); await pending
    await page.locator('#wish-link-1').fill('https://shop.example.com/second')
    release()
    await page.getByRole('button', { name: texts.en['wishes.addLink'], exact: true }).click()
    await page.locator('#wish-link-2').fill('https://alternative.example.com/item')
    await page.locator('#wish-link-2').blur()
    await page.waitForTimeout(950)
    assert.equal(await page.locator('#wish-title').inputValue(), '')
    assert.equal(urls.length, 1)
    await importButton(page).click()
    await waitStatus(page, texts.en['wishes.import.ok'])
    assert.equal(await page.locator('#wish-title').inputValue(), 'Second product')
    assert.equal(await page.locator('#wish-description').inputValue(), 'Imported details')
    assert.deepEqual(urls, ['https://shop.example.com/first', 'https://shop.example.com/second'])
    // Keep manual edits, remove pristine values from the previous source on URL change.
    await page.locator('#wish-title').fill('Keep this title')
    await page.locator('#wish-link-1').fill('https://shop.example.com/third')
    assert.equal(await page.locator('#wish-title').inputValue(), 'Keep this title')
    assert.equal(await page.locator('#wish-price').inputValue(), '')
    assert.equal(await page.locator('#wish-description').inputValue(), '')
    assert.equal(await page.locator('.file-name').count(), 0)
    checks++; await context.close()
  }
  // A manually edited imported description survives source changes and retries.
  {
    const { page, context, result } = await setup()
    await page.route('**/api/wishes/import-preview', route => route.fulfill({ json: result }))
    await page.locator('#wish-link-1').fill('https://shop.example.com/first')
    await waitStatus(page, texts.en['wishes.import.ok'])
    await page.locator('#wish-description').fill('Keep my description')
    await page.locator('#wish-link-1').fill('https://shop.example.com/second')
    assert.equal(await page.locator('#wish-description').inputValue(), 'Keep my description')
    await importButton(page).click()
    await waitStatus(page, texts.en['wishes.import.ok'])
    assert.equal(await page.locator('#wish-description').inputValue(), 'Keep my description')
    checks++; await context.close()
  }
  // Any link can be selected; switching sources does not mix old automatic values.
  {
    const { page, context, result } = await setup()
    const urls = []
    await page.route('**/api/wishes/import-preview', async route => {
      const { url } = route.request().postDataJSON(); urls.push(url)
      const next = url.endsWith('second') ? { status: 'partial', title: 'Other product' } : result
      await route.fulfill({ json: next })
    })
    await page.locator('#wish-link-1').fill('https://shop.example.com/first')
    await waitStatus(page, texts.en['wishes.import.ok'])
    await page.getByRole('button', { name: texts.en['wishes.addLink'], exact: true }).click()
    await page.locator('#wish-link-2').fill('https://shop.example.com/second')
    await page.locator('#wish-description').fill('My own details')
    await importButton(page, 2).click()
    await waitStatus(page, texts.en['wishes.import.partial'])
    assert.deepEqual(urls, ['https://shop.example.com/first', 'https://shop.example.com/second'])
    assert.equal(await page.locator('#wish-title').inputValue(), 'Other product')
    assert.equal(await page.locator('#wish-price').inputValue(), '')
    assert.equal(await page.locator('.file-name').count(), 0)
    assert.equal(await page.locator('#wish-description').inputValue(), 'My own details')
    await page.locator('#wish-link-1').fill('https://shop.example.com/third')
    assert.equal(await page.locator('#wish-title').inputValue(), 'Other product')
    await page.locator('#wish-link-2').fill('https://shop.example.com/changed')
    assert.equal(await page.locator('#wish-title').inputValue(), '')
    assert.equal(await page.locator('#wish-description').inputValue(), 'My own details')
    checks++; await context.close()
  }
  // Starting an import from another link cancels the previous request.
  {
    const { page, context, result } = await setup()
    let release, started
    const gate = new Promise(resolve => { release = resolve })
    const pending = new Promise(resolve => { started = resolve })
    await page.route('**/api/wishes/import-preview', async route => {
      const { url } = route.request().postDataJSON()
      if (url.endsWith('first')) { started(); await gate }
      await route.fulfill({ json: { ...result, title: url.endsWith('first') ? 'Stale' : 'Selected' } }).catch(() => {})
    })
    await page.locator('#wish-link-1').fill('https://shop.example.com/first'); await pending
    await page.getByRole('button', { name: texts.en['wishes.addLink'], exact: true }).click()
    await page.locator('#wish-link-2').fill('https://shop.example.com/second')
    await importButton(page, 2).click()
    await waitStatus(page, texts.en['wishes.import.ok'])
    release()
    assert.equal(await page.locator('#wish-title').inputValue(), 'Selected')
    checks++; await context.close()
  }
  // Successful feedback disappears automatically; invalid links cannot be imported.
  {
    const { page, context, result } = await setup()
    await page.route('**/api/wishes/import-preview', route => route.fulfill({ json: result }))
    assert.equal(await importButton(page).isDisabled(), true)
    await page.locator('#wish-link-1').fill('https://shop.example.com/first')
    await waitStatus(page, texts.en['wishes.import.ok'])
    await page.locator('.toast').waitFor({ state: 'hidden', timeout: 6000 })
    checks++; await context.close()
  }
  // The per-link import action is available and labelled to keyboard users.
  {
    const { page, context, result } = await setup()
    const urls = []
    await page.route('**/api/wishes/import-preview', route => { urls.push(route.request().postDataJSON().url); return route.fulfill({ json: result }) })
    await page.locator('#wish-link-1').fill('https://shop.example.com/first')
    await waitStatus(page, texts.en['wishes.import.ok'])
    await page.getByRole('button', { name: texts.en['wishes.addLink'], exact: true }).click()
    await page.locator('#wish-link-2').fill('https://shop.example.com/second')
    await page.locator('#wish-link-2').press('Tab')
    assert.equal(await importButton(page, 2).evaluate(el => document.activeElement === el), true)
    const response = page.waitForResponse(r => r.url().endsWith('/api/wishes/import-preview'))
    await page.keyboard.press('Enter')
    await response
    assert.deepEqual(urls, ['https://shop.example.com/first', 'https://shop.example.com/second'])
    checks++; await context.close()
  }
  // Deleting the source cancels its pending request and removes only its untouched data.
  {
    const { page, context, result } = await setup()
    let release, started
    const gate = new Promise(resolve => { release = resolve })
    const pending = new Promise(resolve => { started = resolve })
    await page.route('**/api/wishes/import-preview', async route => {
      const { url } = route.request().postDataJSON()
      if (url.endsWith('second')) { started(); await gate }
      await route.fulfill({ json: result }).catch(() => {})
    })
    await page.locator('#wish-link-1').fill('https://shop.example.com/first')
    await waitStatus(page, texts.en['wishes.import.ok'])
    await page.getByRole('button', { name: texts.en['wishes.addLink'], exact: true }).click()
    await page.locator('#wish-link-2').fill('https://shop.example.com/second')
    await importButton(page, 2).click(); await pending
    await page.getByRole('button', { name: `${texts.en['wishes.removeLink']} 2`, exact: true }).click()
    release()
    assert.equal(await page.locator('#wish-title').inputValue(), 'Imported product')
    await page.locator('#wish-description').fill('Keep my details')
    await page.getByRole('button', { name: texts.en['wishes.addLink'], exact: true }).click()
    await page.getByRole('button', { name: `${texts.en['wishes.removeLink']} 1`, exact: true }).click()
    assert.equal(await page.locator('#wish-title').inputValue(), '')
    assert.equal(await page.locator('#wish-description').inputValue(), 'Keep my details')
    checks++; await context.close()
  }
  // Save stays available when import fails, returns a category, or is in progress.
  for (const mode of ['failed', 'not_product', 'loading']) {
    const { page, context, result } = await setup()
    let started
    const pending = new Promise(resolve => { started = resolve })
    await page.route('**/api/wishes/import-preview', async route => {
      started()
      if (mode === 'loading') { await new Promise(resolve => page.once('close', resolve)); return }
      await route.fulfill(mode === 'failed' ? { status: 422, json: { error: 'import_unavailable' } } : { json: { status: 'not_product' } })
    })
    let saved
    await page.route('**/api/wishes', async route => {
      if (route.request().method() === 'POST') { saved = route.request().postDataJSON(); await route.fulfill({ status: 201, json: { wish: { id: 'saved' } } }) }
      else await route.fulfill({ json: { wishes: [] } })
    })
    await page.locator('#wish-link-1').fill('https://shop.example.com/item'); await pending
    if (mode !== 'loading') await waitStatus(page, texts.en[`wishes.import.${mode}`])
    await page.locator('#wish-title').fill('Manual gift')
    await page.locator('.new-wish-page form button[type=submit]').click()
    await page.waitForURL('**/my/wishes?notice=wish-added')
    assert.equal(saved.title, 'Manual gift'); assert.equal(saved.price, null)
    checks++; await context.close()
  }
  // Imported description can be edited and is sent with the saved wish.
  {
    const { page, context, result } = await setup()
    await page.route('**/api/wishes/import-preview', route => route.fulfill({ json: { ...result, status: 'partial', image: undefined } }))
    let saved
    await page.route('**/api/wishes', async route => {
      if (route.request().method() === 'POST') { saved = route.request().postDataJSON(); await route.fulfill({ status: 201, json: { wish: { id: 'saved' } } }) }
      else await route.fulfill({ json: { wishes: [] } })
    })
    await page.locator('#wish-link-1').fill('https://shop.example.com/item')
    await waitStatus(page, texts.en['wishes.import.partial'])
    assert.equal(await page.locator('#wish-description').inputValue(), 'Imported details')
    await page.locator('#wish-description').fill('Edited details')
    await page.locator('.new-wish-page form button[type=submit]').click()
    await page.waitForURL('**/my/wishes?notice=wish-added')
    assert.equal(saved.description, 'Edited details')
    checks++; await context.close()
  }
  // Inspect the imported state, loading/error messages, touch targets and overflow in all layouts/languages.
  await mkdir(new URL('../../.cache/import-screens/', import.meta.url), { recursive: true })
  for (const lang of ['en', 'ru', 'ro']) {
    for (const width of [320, 390, 768, 1024, 1440]) {
      const { page, context, result } = await setup(lang, width)
      await page.route('**/api/wishes/import-preview', route => route.fulfill({ json: { ...result, title: `${result.title} ${'Long name '.repeat(14)}` } }))
      await page.locator('#wish-link-1').fill('https://shop.example.com/product')
      await waitStatus(page, texts[lang]['wishes.import.ok'])
      await page.getByRole('button', { name: texts[lang]['wishes.addLink'], exact: true }).click()
      await page.locator('#wish-link-2').fill('https://shop.example.com/alternative')
      await page.evaluate(() => document.fonts.ready)
      assert.equal(await page.locator('html').getAttribute('lang'), lang)
      assert.equal(await page.locator('#wish-description').inputValue(), 'Imported details')
      assert.equal(await page.getByLabel(`${texts[lang]['wishes.shopLink']} 2`, { exact: true }).count(), 1)
      assert.equal(await page.locator('.link-row label').first().evaluate(el => getComputedStyle(el).position), 'absolute')
      assert.equal(await page.locator('.image-preview img').evaluate(img => img.complete && img.naturalWidth > 0), true)
      const state = await page.evaluate(() => {
        const visible = el => el.getClientRects().length && !el.classList.contains('sr-only')
        const width = document.documentElement.clientWidth
        return { overflow: document.documentElement.scrollWidth > width,
          outside: [...document.querySelectorAll('main *')].filter(visible).filter(el => { const r = el.getBoundingClientRect(); return r.left < -1 || r.right > width + 1 }).map(el => el.className),
          small: [...document.querySelectorAll('a,button,input,select,textarea')].filter(visible).filter(el => !el.classList.contains('skip-link')).filter(el => { const r = el.getBoundingClientRect(); return r.width < 43.9 || r.height < 43.9 }).map(el => el.id || el.textContent) }
      })
      assert.equal(state.overflow, false, `${lang} ${width}`)
      assert.deepEqual(state.outside, []); assert.deepEqual(state.small, [])
      if (lang === 'ru' && [390, 1440].includes(width)) await page.screenshot({ path: new URL(`../../.cache/import-screens/${lang}-${width}.png`, import.meta.url).pathname.replace(/^\/(\w:)/, '$1'), fullPage: true })
      checks++; await context.close()
    }
  }
  assert.deepEqual(errors, [])
  console.log(`Passed ${checks} browser scenarios; EN/RU/RO × 320/390/768/1024/1440; no overflow, undersized controls, or Vue errors.`)
} finally { await browser.close() }
