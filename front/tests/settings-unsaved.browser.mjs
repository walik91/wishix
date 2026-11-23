// Run against local Vite. API calls are mocked.
import assert from 'node:assert/strict'
import { mkdir, readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'

const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const browser = await chromium.launch({ channel: 'chrome' })
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://127.0.0.1:5174'
const screenshots = new URL('../../.cache/settings-unsaved-screens/', import.meta.url)
const originalDetails = {
  username: 'tester', biography: 'Saved bio', birth_date: '2000-02-29',
  show_age: false, show_followers: false, show_following: false, has_avatar: false,
}

async function setup(locale = 'en', width = 390) {
  const context = await browser.newContext({ locale, viewport: { width, height: width === 320 ? 640 : 900 } })
  await context.addInitScript(() => localStorage.setItem('wishix_analytics_consent_v1', 'denied'))
  const failures = new Set()
  const writes = []
  let details = { ...originalDetails }, displayName = 'Saved name', collection = false, googleLinked = false
  await context.route(`${origin}/api/**`, route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (path === '/api/auth/google/start') {
      assert.equal(request.method(), 'POST')
      assert.deepEqual(request.postDataJSON(), { mode: 'link' })
      googleLinked = true
      return route.fulfill({ json: { url: `${origin}/my?google=linked` } })
    }
    if (request.method() === 'PUT') {
      writes.push(path)
      if (failures.delete(path)) return route.fulfill({ status: 503, json: { error: 'unavailable' } })
      const input = request.postDataJSON()
      if (path === '/api/profile/name') displayName = input.display_name.trim()
      if (path === '/api/profile/details') details = { ...details, ...input, biography: input.biography.trim() }
      if (path === '/api/profile/username') details.username = input.username
      if (path === '/api/profile/collection') collection = input.show_all_wishes
    }
    const data = {
      '/api/auth/me': { user: { username: details.username, email: 'tester@example.test' } },
      '/api/auth/google/status': { enabled: true, linked: googleLinked },
      '/api/auth/facebook/status': { enabled: false },
      '/api/lists': { lists: [] },
      '/api/wishes': { wishes: [] },
      '/api/profile/name': { display_name: displayName },
      '/api/profile/details': details,
      '/api/profile/username': details,
      '/api/profile/collection': { show_all_wishes: collection },
    }
    return data[path] ? route.fulfill({ json: data[path] }) : route.abort()
  })
  const messages = JSON.parse(await readFile(new URL(`../src/locales/${locale}.json`, import.meta.url)))
  const page = await context.newPage()
  await page.goto(`${origin}/my`)
  const opener = page.getByRole('button', { name: messages['my.openAccount'] })
  const settings = page.locator('dialog.profile-dialog')
  const confirmation = page.getByRole('dialog', { name: messages['discard.title'] })
  const keep = confirmation.getByRole('button', { name: messages['discard.continue'] })
  const leave = confirmation.getByRole('button', { name: messages['discard.leave'] })
  const name = settings.locator('#profile-display-name')
  const bio = settings.locator('#profile-biography')
  const collectionBox = settings.getByRole('checkbox', { name: messages['profile.showAll'] })
  const close = settings.locator(':scope > header > button')
  const accountTab = settings.getByRole('button', { name: messages['profile.account'], exact: true })
  const profileTab = settings.getByRole('button', { name: messages['my.profile'], exact: true })
  const openSettings = async () => {
    await opener.click()
    await settings.getByText('tester@example.test', { exact: true }).waitFor()
    assert.equal(await settings.getAttribute('aria-label'), messages['profile.settings'])
    assert.equal(await accountTab.getAttribute('aria-pressed'), 'true')
    assert.equal(await name.isVisible(), false)
  }
  const openProfile = async () => {
    await profileTab.click()
    await bio.waitFor()
    await settings.locator('#profile-display-name:enabled').waitFor()
  }
  const waitClosed = async () => {
    await page.locator('button.profile-button[aria-expanded=false]').waitFor()
    assert.equal(await confirmation.isVisible(), false)
  }
  await openSettings()
  return {
    context, page, settings, confirmation, keep, leave, messages, name, bio, collectionBox,
    close, accountTab, profileTab, openSettings, openProfile, waitClosed, failures, writes,
  }
}

try {
  for (const locale of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const ui = await setup(locale, width)
    const { page, settings, confirmation, keep, leave, messages, name, bio, close, accountTab, profileTab } = ui
    await settings.getByText(messages['google.notLinked'], { exact: true }).waitFor()
    await settings.getByRole('button', { name: messages['google.connect'] }).waitFor()
    await settings.getByRole('button', { name: messages['auth.logout'] }).waitFor()
    assert.equal(await settings.evaluate(el => el.scrollWidth > el.clientWidth), false, `${locale}/${width}: account width`)
    if (locale === 'ru' && [390, 1440].includes(width)) {
      await mkdir(screenshots, { recursive: true })
      await page.screenshot({ path: fileURLToPath(new URL(`settings-account-${width}.png`, screenshots)) })
    }
    await ui.openProfile()
    const save = settings.getByRole('button', { name: messages['profile.saveChanges'] })
    assert.equal(await save.isVisible(), false)
    const tabs = await Promise.all([accountTab.boundingBox(), profileTab.boundingBox()])
    assert.equal(tabs[0].y, tabs[1].y, `${locale}/${width}: horizontal tabs`)
    assert.ok((await settings.boundingBox()).width <= 720, `${locale}/${width}: settings width`)
    assert.equal(await settings.getByText('tester@example.test', { exact: true }).isVisible(), false)
    assert.equal(await settings.getByRole('button', { name: messages['auth.logout'] }).isVisible(), false)
    assert.equal(await settings.getByLabel(messages['profile.address'], { exact: true }).inputValue(), originalDetails.username)
    assert.equal(await settings.evaluate(el => el.scrollWidth > el.clientWidth), false, `${locale}/${width}: profile width`)
    if (locale === 'ru' && [390, 1440].includes(width)) {
      await page.screenshot({ path: fileURLToPath(new URL(`settings-profile-${width}.png`, screenshots)) })
    }
    await name.fill('Draft name')
    await bio.fill('Draft bio')
    if (locale === 'ru' && [390, 1440].includes(width)) {
      await page.screenshot({ path: fileURLToPath(new URL(`settings-profile-dirty-${width}.png`, screenshots)) })
    }
    await accountTab.click()
    assert.equal(await name.isVisible(), false)
    await profileTab.click()
    assert.equal(await name.inputValue(), 'Draft name')
    assert.equal(await bio.inputValue(), 'Draft bio')

    await close.click()
    await keep.waitFor()
    assert.equal(await settings.getAttribute('open'), '')
    assert.equal(await keep.evaluate(button => document.activeElement === button), true)
    const bounds = await confirmation.boundingBox()
    assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= width, `${locale}/${width}: confirmation width`)
    assert.equal(await confirmation.evaluate(el => el.scrollWidth > el.clientWidth), false)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false)
    if (locale === 'ru' && [390, 1440].includes(width)) {
      await mkdir(screenshots, { recursive: true })
      await page.screenshot({ path: fileURLToPath(new URL(`settings-discard-${width}.png`, screenshots)) })
    }
    await keep.click()
    await confirmation.waitFor({ state: 'hidden' })
    assert.equal(await bio.inputValue(), 'Draft bio')

    await bio.focus()
    await page.keyboard.press('Escape')
    await keep.waitFor()
    await page.keyboard.press('Escape')
    await confirmation.waitFor({ state: 'hidden' })
    assert.equal(await bio.evaluate(field => document.activeElement === field), true)
    await page.mouse.click(4, 4)
    await leave.click()
    await ui.waitClosed()

    await ui.openSettings()
    await ui.openProfile()
    assert.equal(await name.inputValue(), 'Saved name')
    assert.equal(await bio.inputValue(), originalDetails.biography)
    await close.click()
    await ui.waitClosed()
    await ui.context.close()
  }

  {
    const ui = await setup()
    const { page, settings, confirmation, keep, name, bio, close, accountTab, profileTab, messages, failures, collectionBox } = ui
    await settings.getByRole('button', { name: messages['google.connect'] }).click()
    await page.waitForURL(`${origin}/my?google=linked`)
    await settings.getByText(messages['google.linked'], { exact: true }).waitFor()
    assert.equal(await accountTab.getAttribute('aria-pressed'), 'true')
    assert.equal(await settings.getByRole('button', { name: messages['google.connect'] }).isVisible(), false)
    const username = settings.locator('#profile-username')
    const date = settings.locator('#profile-birth-date')
    const showAge = settings.getByRole('checkbox', { name: messages['profile.showAge'] })
    const showFollowers = settings.getByRole('checkbox', { name: messages['profile.showFollowers'] })
    const showFollowing = settings.getByRole('checkbox', { name: messages['profile.showFollowing'] })
    const save = settings.getByRole('button', { name: messages['profile.saveChanges'] })
    const cancel = settings.locator('.profile-footer').getByRole('button', { name: messages['profile.cancelChanges'] })

    await ui.openProfile()
    const edits = [
      [() => username.fill('draft_address'), () => username.fill(originalDetails.username)],
      [() => date.fill('2001-01-02'), () => date.fill(originalDetails.birth_date)],
      [() => showAge.check(), () => showAge.uncheck()],
      [() => showFollowers.check(), () => showFollowers.uncheck()],
      [() => showFollowing.check(), () => showFollowing.uncheck()],
    ]
    for (const [edit, revert] of edits) {
      await edit()
      await close.click()
      await keep.click()
      await confirmation.waitFor({ state: 'hidden' })
      await revert()
    }
    await page.keyboard.press('Escape')
    await ui.waitClosed()

    await ui.openSettings()
    await ui.openProfile()
    await name.fill('Cancelled name')
    await bio.fill('Cancelled bio')
    await showFollowers.check()
    await username.fill('cancel_preserves_address')
    await cancel.click()
    assert.equal(await name.inputValue(), 'Saved name')
    assert.equal(await bio.inputValue(), originalDetails.biography)
    assert.equal(await showFollowers.isChecked(), false)
    assert.equal(await username.inputValue(), 'cancel_preserves_address')
    assert.equal(await save.isVisible(), false)
    assert.deepEqual(ui.writes, [])
    await username.fill(originalDetails.username)

    failures.add('/api/profile/name')
    await name.fill('  New name  ')
    await save.click()
    await settings.getByRole('alert').getByText(messages['profile.nameSaveError']).waitFor()
    await close.click()
    await keep.click()
    assert.equal(await name.inputValue(), '  New name  ')
    await save.click()
    await settings.getByRole('status').getByText(messages['profile.saved']).waitFor()
    assert.equal(await name.inputValue(), 'New name')

    failures.add('/api/profile/collection')
    await collectionBox.click()
    await page.getByRole('dialog', { name: messages['profile.publishCollection'] })
      .getByRole('button', { name: messages['profile.publishCollection'] }).click()
    await settings.getByRole('alert').getByText(messages['profile.settingsError']).waitFor()
    await close.click()
    await keep.click()
    assert.equal(await collectionBox.isChecked(), true)
    await settings.locator('.collection-setting').getByRole('button', { name: messages.checkAgain }).click()
    await settings.locator('.collection-setting input:enabled').waitFor()

    failures.add('/api/profile/details')
    await name.fill('Partially saved name')
    await bio.fill('  New bio  ')
    await save.click()
    await settings.getByRole('alert').getByText(messages['profile.saveError']).waitFor()
    assert.deepEqual(ui.writes.slice(-2), ['/api/profile/name', '/api/profile/details'])
    await close.click()
    await keep.click()
    assert.equal(await bio.inputValue(), '  New bio  ')
    await username.fill('new_address')
    const beforeRetry = ui.writes.length
    await save.click()
    await settings.getByRole('status').getByText(messages['profile.saved'], { exact: true }).waitFor()
    assert.deepEqual(ui.writes.slice(beforeRetry), ['/api/profile/details'])
    assert.equal(await save.isVisible(), false)
    assert.equal(await bio.inputValue(), 'New bio')
    assert.equal(await username.inputValue(), 'new_address')
    await accountTab.click()
    await close.click()
    await keep.click()
    await profileTab.click()
    await username.fill(originalDetails.username)
    await page.mouse.click(4, 4)
    await ui.waitClosed()

    const upload = Promise.withResolvers()
    await page.route(`${origin}/api/profile/avatar*`, async route => {
      if (route.request().method() === 'PUT') {
        await upload.promise
        return route.fulfill({ json: { status: 'ok' } })
      }
      return route.fulfill({ contentType: 'image/png', body: avatar })
    })
    const avatar = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/aXcAAAAASUVORK5CYII=', 'base64')
    await ui.openSettings()
    await ui.openProfile()
    await settings.locator('input[type=file]').setInputFiles({ name: 'avatar.png', mimeType: 'image/png', buffer: avatar })
    assert.equal(await close.isDisabled(), true)
    await page.keyboard.press('Escape')
    assert.equal(await settings.getAttribute('open'), '')
    assert.equal(await confirmation.isVisible(), false)
    upload.resolve()
    await settings.getByRole('button', { name: messages['profile.removeAvatar'] }).waitFor()
    await settings.locator('.profile-photo').getByRole('status').getByText(messages['profile.avatarSaved']).waitFor()
    await close.click()
    await ui.waitClosed()
    await ui.context.close()
  }

  for (const width of [390, 844]) {
    const { context, page, settings, bio, messages, openProfile } = await setup('ro', width)
    await openProfile()
    await bio.fill('A draft at the bottom of the visible area')
    const scroll = settings.locator('.profile-scroll')
    const footer = settings.locator('.profile-footer')
    await page.evaluate(() => {
      Object.defineProperty(visualViewport, 'height', { configurable: true, value: 360 })
      Object.defineProperty(visualViewport, 'offsetTop', { configurable: true, value: 16 })
      visualViewport.dispatchEvent(new Event('resize'))
    })
    const dialog = await settings.boundingBox()
    assert.ok(dialog.y >= 16 && dialog.y + dialog.height <= 376, 'The dialog fits in the reduced visual viewport')
    const area = await scroll.boundingBox(), field = await bio.boundingBox(), actions = await footer.boundingBox()
    assert.ok(field.y >= area.y && field.y + field.height <= actions.y, 'The focused field fits above the actions with the keyboard open')
    const lastControl = settings.getByRole('link', { name: messages['profile.following'] })
    await lastControl.focus()
    const last = await lastControl.boundingBox()
    assert.ok(last.y >= area.y && last.y + last.height <= actions.y, 'The last control can scroll into view above the actions')
    assert.equal(await scroll.evaluate(el => el.scrollWidth > el.clientWidth), false)
    await context.close()
  }
  console.log('Settings layout and drafts passed in 15 locale and viewport combinations, including partial saves, independent photo upload and a reduced visual viewport.')
} finally { await browser.close() }
