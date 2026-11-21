// This test loads the real Google tag but blocks all measurement requests.
// Start Vite with a test stream ID and the intended stream settings first.
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5193'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
try {
  const context = await browser.newContext()
  const page = await context.newPage()
  const requests = []
  let tagStatus
  page.on('response', response => { if (response.url().includes('/gtag/js?')) tagStatus = response.status() })
  await page.route('**/api/auth/me', route => route.fulfill({ json: { user: null } }))
  await context.route('https://shop.example/**', route => route.abort())
  await context.route('**/g/collect**', route => {
    const request = route.request()
    requests.push({ url: request.url(), body: request.postData() || '' })
    return route.abort()
  })
  await page.goto(origin + '/?q=test-code-918273&email=codex-test%40example.com')
  await page.locator('.cookie-actions button').first().click()
  await page.waitForTimeout(4000)
  await page.evaluate(() => {
    history.pushState(null, '', '/u/person/wishlist/list-id/fake-share-token?code=918273')
    const form = document.createElement('form')
    form.innerHTML = '<input name="email" value="codex-test@example.com"><button>Submit</button>'
    form.addEventListener('submit', event => event.preventDefault())
    document.body.append(form)
    form.querySelector('input').dispatchEvent(new Event('change', { bubbles: true }))
    form.querySelector('button').click()
    const link = document.createElement('a')
    link.id = 'test-shop-link'
    link.href = 'https://shop.example/product?code=918273'
    link.textContent = 'Shop'
    link.target = '_blank'
    document.body.append(link)
  })
  await page.locator('#test-shop-link').click()
  await page.waitForTimeout(4000)
  assert.equal(tagStatus, 200, 'real Google tag must load')
  assert.ok(requests.length, 'capture measurement traffic')
  const leaks = requests.flatMap(request => {
    const url = new URL(request.url)
    return [...url.searchParams].filter(([, value]) => /918273|fake-share-token|codex-test/i.test(value))
      .map(([name]) => (url.searchParams.get('en') || 'event') + ':' + name)
      .concat(/918273|fake-share-token|codex-test/i.test(decodeURIComponent(request.body)) ? ['request-body'] : [])
  })
  assert.deepEqual(leaks, [], 'test codes and tokens must not enter any Google measurement request')
  const events = requests.map(({ url }) => new URL(url)).filter(url => url.hostname === 'www.google-analytics.com')
  assert.equal(events.filter(url => url.searchParams.get('en') === 'page_view').length, 1, 'one page view, including after history change')
  assert.ok(events.some(url => url.searchParams.get('dl') === origin + '/?q=test-code-'))
  if (process.env.WISHIX_GA_BASIC_ONLY !== '1') {
    for (const name of ['form_start', 'view_search_results', 'click']) assert.ok(events.some(url => url.searchParams.get('en') === name), 'capture real enhanced event ' + name)
    assert.ok(events.some(url => url.searchParams.get('en') === 'click' && url.searchParams.get('ep.link_domain') === 'shop.example'), 'retain the shop domain')
  }
  const countBeforeShare = requests.length
  const sharePage = await context.newPage()
  await sharePage.addInitScript(() => {
    if (location.pathname === '/') history.replaceState(null, '', '/u/person/wishlist/12345678-1234-4234-8234-123456789abc/fake-share-token?email=codex-test%40example.com&notice=ok')
  })
  await sharePage.goto(origin + '/')
  await sharePage.waitForFunction(() => !!window.gtag)
  await sharePage.waitForTimeout(8000)
  const shareEvents = requests.slice(countBeforeShare).map(({ url }) => new URL(url))
  assert.ok(shareEvents.some(url => url.searchParams.get('dl') === origin + '/u/person/wishlist/12345678-1234-4234-8234-123456789abc?notice=ok'), 'preserve username and list ID; received ' + JSON.stringify(shareEvents.map(url => url.searchParams.get('dl'))))
  assert.ok(!JSON.stringify(requests).includes('fake-share-token') && !JSON.stringify(requests).includes('codex-test'), 'do not transmit share token or email')
  console.log('Passed: real Google tag, sanitized measurement requests and one page view; all measurement traffic was blocked.')
} finally { await browser.close() }
