// Run against a local Vite server with mocked APIs.
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5174'
const text = JSON.parse(await readFile(new URL('../src/locales/ru.json', import.meta.url)))
const browser = await chromium.launch({ headless: true, channel: 'chrome' })

try {
  const page = await browser.newPage({ locale: 'ru', viewport: { width: 390, height: 844 } })
  const wish = { id: 'gift', title: 'Wish', description: '', price: '12.50', currency: 'NOK', priority: 'medium', shop_links: [], edit_version: 'a'.repeat(64) }
  const requests = []
  let account = 'owner'
  await page.route('**/api/auth/me', route => route.fulfill({ json: { user: { id: account, username: 'tester' } } }))
  await page.route('**/api/lists', route => route.fulfill({ json: { lists: [] } }))
  await page.route('**/api/wishes', async route => {
    if (route.request().method() === 'POST') {
      requests.push(route.request().postDataJSON())
      await route.fulfill({ status: 201, json: { wish } })
    } else await route.fulfill({ json: { wishes: [] } })
  })
  await page.route(`**/api/wishes/${wish.id}`, async route => {
    if (route.request().method() === 'PUT') {
      requests.push(route.request().postDataJSON())
      await route.fulfill({ json: { status: 'ok' } })
    } else await route.fulfill({ json: { wish } })
  })
  await page.route('**/api/wishes/import-preview', route => route.fulfill({ json: { status: 'partial', title: 'Product', price: '49.95', currency: 'USD' } }))

  const title = page.locator('#wish-title')
  const amount = page.locator('#wish-price')
  const currency = page.getByRole('combobox', { name: text['wishes.currencyLabel'] })
  const details = page.getByRole('button', { name: text['wishes.addDetails'] })
  const save = page.getByRole('button', { name: text['wishes.save'], exact: true })
  const cancel = page.getByRole('link', { name: text['wishes.cancel'], exact: true })
  const link = page.locator('#wish-link-1')

  async function openNewWish() {
    await page.goto(`${origin}/my/wishes/new`)
    await details.click()
    await currency.waitFor()
  }

  await openNewWish()
  assert.equal(await page.getByRole('group', { name: text['wishes.priceLabel'] }).count(), 1)
  assert.equal(await currency.inputValue(), '')
  assert.equal(await currency.locator('option:checked').innerText(), text['wishes.currencyPlaceholder'])
  assert.deepEqual((await currency.locator('option').evaluateAll(options => options.map(option => option.value))).slice(1, 4), ['MDL', 'RON', 'EUR'])

  await title.fill('Gift with price')
  await amount.fill('120,50')
  await details.click()
  await save.click()
  const error = page.locator('#wish-currency-error')
  assert.equal(await error.innerText(), text['wishes.error.wish_currency_required'])
  assert.equal(await currency.getAttribute('aria-describedby'), 'wish-currency-error')
  assert.equal(await currency.evaluate(element => element === document.activeElement), true)
  assert.equal(requests.length, 0)

  await currency.selectOption('other')
  const code = page.locator('#wish-currency-code')
  assert.equal(await code.evaluate(element => element === document.activeElement), true)
  await code.fill('mxn')
  await save.click()
  await page.waitForURL('**/my/wishes?notice=wish-added')
  assert.equal(requests.at(-1).price, '120.50')
  assert.equal(requests.at(-1).currency, 'MXN')

  await openNewWish()
  assert.equal(await currency.inputValue(), 'MXN')
  await cancel.click()
  await page.waitForURL('**/my/wishes')

  await openNewWish()
  await link.fill('https://shop.example.test/item')
  await link.blur()
  await page.locator('.toast').filter({ hasText: text['wishes.import.partial'] }).waitFor()
  assert.equal(await currency.inputValue(), 'USD')
  await link.fill('')
  assert.equal(await amount.inputValue(), '')
  assert.equal(await currency.inputValue(), 'MXN')
  await title.fill('Gift without price')
  await save.click()
  await page.waitForURL('**/my/wishes?notice=wish-added')
  assert.equal(requests.at(-1).price, null)
  assert.equal(requests.at(-1).currency, '')

  await page.goto(`${origin}/my/wishes/${wish.id}/edit`)
  await currency.waitFor()
  assert.equal(await currency.inputValue(), 'NOK')
  await title.fill('Edited wish')
  await save.click()
  await page.waitForURL('**/my/wishes?notice=wish-updated')
  assert.equal(requests.at(-1).price, '12.50')
  assert.equal(requests.at(-1).currency, 'NOK')

  await openNewWish()
  assert.equal(await currency.inputValue(), 'MXN')
  account = 'another-owner'
  await openNewWish()
  assert.equal(await currency.inputValue(), '')
  console.log('Wish price: grouping, validation, currency preference, import, and editing passed')
} finally { await browser.close() }
