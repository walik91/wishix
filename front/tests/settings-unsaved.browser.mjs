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
  let details = { ...originalDetails }, displayName = 'Saved name', collection = false
  await context.route(`${origin}/api/**`, route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (request.method() === 'PUT') {
      if (failures.delete(path)) return route.fulfill({ status: 503, json: { error: 'unavailable' } })
      const input = request.postDataJSON()
      if (path === '/api/profile/name') displayName = input.display_name.trim()
      if (path === '/api/profile/details') details = { ...details, ...input, biography: input.biography.trim() }
      if (path === '/api/profile/username') details.username = input.username
      if (path === '/api/profile/collection') collection = input.show_all_wishes
    }
    const data = {
      '/api/auth/me': { user: { username: details.username, email: 'tester@example.test' } },
      '/api/auth/google/status': { enabled: false },
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
  const generalTab = settings.locator('.profile-section').first()
  const profileTab = settings.locator('.profile-section').last()
  const openSettings = async () => {
    await opener.click()
    await settings.locator('#profile-display-name:enabled').waitFor()
  }
  const openProfile = async () => {
    await profileTab.click()
    await bio.waitFor()
  }
  const waitClosed = async () => {
    await page.locator('button.profile-button[aria-expanded=false]').waitFor()
    assert.equal(await confirmation.isVisible(), false)
  }
  await openSettings()
  return {
    context, page, settings, confirmation, keep, leave, messages, name, bio, collectionBox,
    close, generalTab, profileTab, openSettings, openProfile, waitClosed, failures,
  }
}

try {
  for (const locale of ['en', 'ru', 'ro']) for (const width of [320, 390, 768, 1024, 1440]) {
    const ui = await setup(locale, width)
    const { page, settings, confirmation, keep, leave, name, bio, close, generalTab, profileTab } = ui
    await name.fill('Draft name')
    await ui.openProfile()
    await bio.fill('Draft bio')
    await generalTab.click()
    assert.equal(await name.inputValue(), 'Draft name')
    await profileTab.click()
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
    assert.equal(await name.inputValue(), 'Saved name')
    await ui.openProfile()
    assert.equal(await bio.inputValue(), originalDetails.biography)
    await close.click()
    await ui.waitClosed()
    await ui.context.close()
  }

  {
    const ui = await setup()
    const { page, settings, confirmation, keep, name, bio, close, generalTab, profileTab, messages, failures, collectionBox } = ui
    const username = settings.locator('#profile-username')
    const date = settings.locator('#profile-birth-date')
    const showAge = settings.getByRole('checkbox', { name: messages['profile.showAge'] })
    const showFollowers = settings.getByRole('checkbox', { name: messages['profile.showFollowers'] })
    const showFollowing = settings.getByRole('checkbox', { name: messages['profile.showFollowing'] })
    const saveName = settings.getByRole('button', { name: messages['profile.saveName'] })
    const saveDetails = settings.getByRole('button', { name: messages['profile.saveSettings'] })

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
    failures.add('/api/profile/name')
    await name.fill('  New name  ')
    await saveName.click()
    await settings.getByRole('alert').getByText(messages['profile.nameError']).waitFor()
    await close.click()
    await keep.click()
    assert.equal(await name.inputValue(), '  New name  ')
    await saveName.click()
    await settings.getByRole('status').getByText(messages['profile.nameSaved']).waitFor()
    assert.equal(await name.inputValue(), 'New name')

    failures.add('/api/profile/collection')
    await collectionBox.check()
    await settings.getByRole('alert').getByText(messages['profile.settingsError']).waitFor()
    await close.click()
    await keep.click()
    assert.equal(await collectionBox.isChecked(), true)
    await settings.locator('.collection-setting').getByRole('button', { name: messages.checkAgain }).click()
    await settings.locator('.collection-setting input:enabled').waitFor()

    await ui.openProfile()
    failures.add('/api/profile/details')
    await bio.fill('  New bio  ')
    await saveDetails.click()
    await settings.getByRole('alert').getByText(messages['profile.saveError']).waitFor()
    await close.click()
    await keep.click()
    assert.equal(await bio.inputValue(), '  New bio  ')
    await username.fill('new_address')
    await saveDetails.click()
    await settings.getByRole('status').getByText(messages['profile.saved'], { exact: true }).waitFor()
    assert.equal(await bio.inputValue(), 'New bio')
    assert.equal(await username.inputValue(), 'new_address')
    await generalTab.click()
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
    await close.click()
    await ui.waitClosed()
    await ui.context.close()
  }
  console.log('Settings discard passed in 15 locale and viewport combinations, including save failures and independent edits.')
} finally { await browser.close() }
