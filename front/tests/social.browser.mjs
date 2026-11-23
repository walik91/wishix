import assert from 'node:assert/strict'
import { readFile, mkdir } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const origin = process.env.WISHIX_TEST_ORIGIN || 'http://localhost:5173'
const texts = Object.fromEntries(await Promise.all(['en', 'ru', 'ro'].map(async lang => [lang, JSON.parse(await readFile(new URL(`../src/locales/${lang}.json`, import.meta.url)))])))
const indexHTML = await readFile(new URL('../index.html', import.meta.url), 'utf8')
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const errors = []
await mkdir(new URL('../../.cache/social-screens/', import.meta.url), { recursive: true })
try {
  for (const lang of ['en', 'ru', 'ro']) for (const width of [320, 340, 390, 768, 1024, 1440]) {
    const context = await browser.newContext({ locale: lang, viewport: { width, height: 900 } })
    const page = await context.newPage()
    page.on('pageerror', e => errors.push(e.message))
    page.on('console', m => { if (m.text().includes('[Vue warn]')) errors.push(m.text()) })
    if (width === 340) await page.clock.install()
    let username = width === 340 ? 'a'.repeat(30) : 'example', name = { display_name: 'example' }, state = { self: false, following: false, blocked: false, can_follow: true }
    let details = { username, biography: '', birth_date: null, show_age: false, show_followers: false, show_following: false, has_avatar: false, username_changed_at: null }
    let reports = [], failDetails = false, failState = false
    let renameError = '', renameRequests = 0
    let guest = false, sessionFailure = false, showFollowers = true, showFollowing = true
    let publicPeople = []
    let peopleStatus = 200, nextCursor = '', pendingPeople
    const publicCursors = []
    const publicName = width <= 390 ? 'Bob Example ' + 'a'.repeat(88) : 'Bob Example'
    await page.route('**/api/auth/me', route => {
      if (sessionFailure) return route.fulfill({ status: 503, json: { error: 'unavailable' } })
      if (guest) return route.fulfill({ status: 401, json: { error: 'invalid_session' } })
      return route.fulfill({ json: { user: { username, email: 'owner@example.test' } } })
    })
    await page.route('**/api/lists', route => route.fulfill({ json: { lists: [] } }))
    await page.route('**/api/wishes', route => route.fulfill({ json: { wishes: [] } }))
    await page.route('**/u/bob', route => route.fulfill({ headers: { 'referrer-policy': 'no-referrer' }, contentType: 'text/html', body: indexHTML }))
    await page.route('**/api/profile/name', route => { if (route.request().method() === 'PUT') name = route.request().postDataJSON(); return route.fulfill({ json: name }) })
    await page.route('**/api/profile/details', route => { if (route.request().method() === 'PUT') details = { ...details, ...route.request().postDataJSON() }; return route.fulfill({ json: details }) })
    await page.route('**/api/profile/collection', route => route.fulfill({ json: { show_all_wishes: false } }))
    await page.route('**/api/profile/username', route => {
      renameRequests++
      assert.deepEqual(route.request().postDataJSON(), { username: 'example_new' })
      if (renameError) return route.fulfill({ status: 409, json: { error: renameError } })
      username = details.username = route.request().postDataJSON().username
      details.username_changed_at = new Date().toISOString()
      return route.fulfill({ json: details })
    })
    await page.route('**/api/profile/avatar*', route => {
      if (route.request().method() === 'PUT') { details.has_avatar = true; return route.fulfill({ json: { status: 'ok' } }) }
      if (route.request().method() === 'DELETE') { details.has_avatar = false; return route.fulfill({ json: { status: 'ok' } }) }
      return route.fulfill({ contentType: 'image/png', body: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/aXcAAAAASUVORK5CYII=', 'base64') })
    })
    await page.route('**/api/public/profiles/bob', route => route.fulfill({ json: { username: 'bob', display_name: publicName, show_all_wishes: width === 340, lists: [] } }))
    await page.route('**/api/public/profiles/bob/avatar*', route => route.fulfill({
      contentType: 'image/svg+xml',
      body: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 40"><circle cx="20" cy="20" r="20" fill="purple"/></svg>',
    }))
    await page.route('**/api/public/profiles/bob/details', route => { if (failDetails) { failDetails = false; return route.fulfill({ status: 503, json: { error: 'unavailable' } }) }; return route.fulfill({ json: { username: 'bob', biography: 'A public description', age: 25, has_avatar: false, show_followers: showFollowers, show_following: showFollowing } }) })
    await page.route('**/api/social/bob/state', route => { if (failState) { failState = false; return route.fulfill({ status: 503, json: { error: 'unavailable' } }) }; return route.fulfill({ json: state }) })
    await page.route('**/api/social/bob/follow', route => { state = { ...state, following: route.request().postDataJSON().follow }; return route.fulfill({ json: state }) })
    await page.route('**/api/social/bob/block', route => { const blocked = route.request().postDataJSON().block; state = { ...state, blocked, following: false, can_follow: !blocked }; return route.fulfill({ json: state }) })
    await page.route('**/api/social/bob/report', route => { reports.push(route.request().postDataJSON().message); return route.fulfill({ json: { status: 'ok' } }) })
    await page.route('**/api/social/*/people/*?*', route => route.fulfill({ json: { people: [{ username: 'bob', display_name: 'Bob Example' }], next_cursor: '' } }))
    await page.route(/\/api\/social\/bob\/people\/(followers|following)\?/, async route => {
      const url = new URL(route.request().url())
      publicCursors.push(url.searchParams.get('after'))
      await pendingPeople
      const visible = url.pathname.endsWith('/followers') ? showFollowers : showFollowing
      if (!visible || peopleStatus === 404) return route.fulfill({ status: 404, json: { error: 'list_not_found' } })
      if (peopleStatus !== 200) return route.fulfill({ status: peopleStatus, json: { error: 'unavailable' } })
      return route.fulfill({ json: { people: publicPeople, next_cursor: nextCursor } })
    })
    await page.goto(`${origin}/my/profile`)
    const profileDialog = page.locator('dialog.profile-dialog')
    assert.equal(await profileDialog.isVisible(), true)
    assert.equal(await profileDialog.getAttribute('aria-label'), texts[lang]['profile.settings'])
    assert.equal(await profileDialog.locator(':scope > header h2').textContent(), texts[lang]['profile.settings'])
    const nameField = profileDialog.locator('#profile-display-name')
    await profileDialog.locator('#profile-display-name:enabled').waitFor()
    assert.equal(await profileDialog.getByRole('tab', { name: texts[lang]['my.profile'], selected: true }).count(), 1)
    assert.equal(await nameField.inputValue(), 'example')
    const save = profileDialog.getByRole('button', { name: texts[lang]['profile.saveChanges'] })
    await nameField.fill('   ')
    await save.click()
    await profileDialog.getByRole('alert').getByText(texts[lang]['profile.nameRequired']).waitFor()
    assert.equal(name.display_name, 'example')
    await nameField.fill('Bob Example')
    await save.click()
    await profileDialog.getByRole('status').getByText(texts[lang]['profile.saved']).waitFor()
    assert.deepEqual(name, { display_name: 'Bob Example' })
    await profileDialog.locator('button').filter({ hasText: texts[lang]['profile.chooseAvatar'] }).waitFor()
    await page.locator('input[type=file]').setInputFiles({ name: 'photo.png', mimeType: 'image/png', buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/aXcAAAAASUVORK5CYII=', 'base64') })
    await page.getByRole('button', { name: texts[lang]['profile.removeAvatar'] }).waitFor()
    await page.getByLabel(texts[lang]['profile.birthDate'], { exact: true }).fill('2000-02-29')
    await page.getByRole('checkbox', { name: texts[lang]['profile.showAge'] }).check()
    await page.getByLabel(texts[lang]['profile.biography'], { exact: true }).fill('About me')
    await profileDialog.locator('.profile-section').first().click()
    await profileDialog.locator('.profile-section').nth(1).click()
    assert.equal(await page.getByLabel(texts[lang]['profile.biography'], { exact: true }).inputValue(), 'About me')
    const visibility = profileDialog.getByRole('region', { name: texts[lang]['profile.visibility'] })
    assert.equal(await visibility.getByRole('checkbox').count(), 4)
    await page.getByRole('checkbox', { name: texts[lang]['profile.showFollowers'] }).check()
    await page.getByRole('checkbox', { name: texts[lang]['profile.showFollowing'] }).check()
    await save.click()
    await page.getByText(texts[lang]['profile.saved'], { exact: true }).waitFor()
    assert.equal(details.birth_date, '2000-02-29'); assert.equal(details.show_age, true); assert.equal(details.show_followers, true); assert.equal(details.show_following, true); assert.equal(details.biography, 'About me'); assert.equal('show_avatar' in details, false); assert.equal('show_biography' in details, false)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/profile`)
    assert.equal(await profileDialog.evaluate(el => el.scrollWidth > el.clientWidth), false, `${lang}/${width}/dialog`)
    await profileDialog.locator('.profile-scroll').evaluate(el => { el.scrollTop = el.scrollHeight })
    const sticky = await profileDialog.evaluate(el => {
      const bounds = el.getBoundingClientRect(), header = el.querySelector(':scope > header').getBoundingClientRect(), nav = el.querySelector('.profile-section-list').getBoundingClientRect()
      return header.top >= bounds.top - 1 && header.bottom <= bounds.bottom + 1 && nav.top >= header.bottom - 1 && nav.bottom <= bounds.bottom + 1
    })
    assert.equal(sticky, true, `${lang}/${width}: dialog header and section navigation remain visible`)
    if (width < 1024) assert.equal(await page.locator('.bottom-navigation a, .bottom-navigation button').evaluateAll(nodes => nodes.every(node => { const rect = node.getBoundingClientRect(); return !rect.width || (rect.width >= 44 && rect.height >= 44) })), true, `${lang}/${width}/navigation`)
    const address = profileDialog.getByRole('region', { name: texts[lang]['profile.address'] })
    const editAddress = address.getByRole('button', { name: texts[lang]['profile.editAddress'], exact: true })
    const openProfile = address.getByRole('link', { name: texts[lang]['profile.openPublicProfile'], exact: true })
    const currentAddress = address.getByText(`${origin}/u/${username}`, { exact: true })
    const usernameField = address.getByLabel(texts[lang]['profile.newUsername'], { exact: true })
    const usernameForm = address.locator('#profile-address-editor')
    const change = address.getByRole('button', { name: texts[lang]['profile.changeUsername'], exact: true })
    const confirm = page.getByRole('dialog', { name: texts[lang]['profile.changeUsername'] })
    const confirmChange = confirm.getByRole('button', { name: texts[lang]['profile.changeUsername'], exact: true })
    assert.equal(await openProfile.getAttribute('href'), `/u/${username}`)
    assert.equal(await editAddress.getAttribute('aria-expanded'), 'false')
    assert.equal(await currentAddress.isVisible(), true)
    assert.equal(await usernameForm.isVisible(), false)
    assert.equal(await address.evaluate(el => el.scrollWidth > el.clientWidth), false, `${lang}/${width}: address width`)
    assert.equal(await address.locator('.profile-actions > *').evaluateAll(nodes => nodes.every(node => node.offsetWidth >= 44 && node.offsetHeight >= 44)), true)
    if (lang === 'ru' && [340, 390, 1440].includes(width)) {
      await currentAddress.scrollIntoViewIfNeeded()
      await page.screenshot({ path: fileURLToPath(new URL(`../../.cache/social-screens/profile-${width}.png`, import.meta.url)) })
    }
    await editAddress.click()
    assert.equal(await usernameField.evaluate(el => document.activeElement === el), true)
    assert.equal(await usernameField.inputValue(), username)
    assert.equal(await change.isDisabled(), true)
    assert.equal(await usernameForm.locator('input[type=checkbox]').count(), 0)
    assert.equal(await usernameForm.getByText(texts[lang]['profile.redirectHint']).isVisible(), true)
    assert.equal(await usernameForm.getByText(texts[lang]['profile.usernameHint']).isVisible(), true)
    if (width === 340) {
      await usernameField.fill('bad address!')
      await change.click()
      assert.equal(await usernameField.getAttribute('aria-invalid'), 'true')
      assert.equal(await address.locator('#profile-username-error').textContent(), texts[lang]['profile.usernameInvalid'])
      assert.equal(renameRequests, 0)
      assert.equal(await confirm.isVisible(), false)
      await address.getByRole('button', { name: texts[lang]['profile.cancelAddressEdit'], exact: true }).click()
      assert.equal(await usernameForm.isVisible(), false)
      await editAddress.click()
      assert.equal(await usernameField.inputValue(), username)
      await usernameField.fill('Example_New')
      for (const [error, message] of [['username_taken', 'profile.usernameTaken'], ['username_cooldown', 'profile.usernameCooldown']]) {
        renameError = error
        await change.click()
        await confirmChange.click()
        await confirm.waitFor({ state: 'hidden' })
        await usernameForm.getByText(texts[lang][message], { exact: true }).waitFor()
        assert.equal(await usernameField.inputValue(), 'Example_New')
        assert.equal(await currentAddress.isVisible(), true)
      }
      renameError = ''
    }
    await usernameField.fill('Example_New')
    await change.click()
    assert.equal(await confirm.getByText('/u/example_new', { exact: true }).isVisible(), true)
    assert.equal(renameRequests, width === 340 ? 2 : 0)
    await confirmChange.click()
    await profileDialog.locator('.profile-address').getByRole('status').getByText(texts[lang]['profile.usernameSaved']).waitFor()
    assert.equal(username, 'example_new')
    assert.equal(await usernameForm.isVisible(), false)
    assert.equal(await openProfile.getAttribute('href'), '/u/example_new')
    assert.equal(await editAddress.evaluate(el => document.activeElement === el), true)
    await editAddress.click()
    assert.equal(await usernameField.isDisabled(), true)
    assert.equal(await change.isDisabled(), true)
    const availableAt = new Date(Date.parse(details.username_changed_at) + 30 * 24 * 60 * 60 * 1000).toISOString()
    assert.equal(await address.locator('time').getAttribute('datetime'), availableAt)
    if (width === 340) {
      assert.equal(await profileDialog.locator('.profile-scroll').evaluate(el => el.scrollWidth > el.clientWidth), false)
      // Keep each Playwright clock jump within the signed 32-bit limit.
      const halfCooldown = 15 * 24 * 60 * 60 * 1000
      await page.clock.fastForward(halfCooldown)
      await page.clock.fastForward(halfCooldown + 1000)
      await usernameField.locator('xpath=self::*[not(@disabled)]').waitFor()
      assert.equal(await address.locator('time').count(), 0)
    }
    await profileDialog.locator(':scope > header > button.close').click()
    await page.waitForURL('**/my')
    assert.equal(new URL(page.url()).pathname, '/my')
    assert.equal(await page.locator('.app-navigation:visible a[href="/my"]').getAttribute('aria-current'), 'page')
    await page.locator('.app-navigation:visible').getByRole('button', { name: texts[lang]['my.profile'], exact: true }).click()
    assert.equal(await profileDialog.getAttribute('aria-label'), texts[lang]['profile.settings'])
    assert.equal(await profileDialog.getByRole('tab', { name: texts[lang]['my.profile'], selected: true }).count(), 1)
    await openProfile.waitFor()
    assert.equal(await openProfile.getAttribute('href'), '/u/example_new')
    await profileDialog.getByRole('link', { name: texts[lang]['profile.following'], exact: true }).click()
    await page.waitForURL(`${origin}/my/following`)
    await page.getByRole('heading', { name: texts[lang]['social.title'] }).waitFor()
    await page.getByRole('link', { name: /Bob Example/ }).waitFor()
    await page.locator('.people-tabs').getByRole('button', { name: texts[lang]['social.blocked'], exact: true }).click()
    await page.getByRole('link', { name: /Bob Example/ }).waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/following`)
    await page.goto(`${origin}/u/bob`)
    const profileHeading = page.getByRole('heading', { name: `${publicName} @bob`, exact: true })
    await profileHeading.waitFor()
    const skipLink = page.getByRole('link', { name: texts[lang]['home.skipToContent'], exact: true })
    await page.keyboard.press('Tab')
    await skipLink.and(page.locator(':focus-visible')).waitFor()
    await page.keyboard.press('Enter')
    await page.getByRole('main').and(page.locator(':focus')).waitFor()
    await page.getByText(`${texts[lang]['profile.age']}: 25`, { exact: true }).waitFor()
    const publicHeader = page.locator('.public-header')
    const mySpace = publicHeader.getByRole('link', { name: texts[lang]['auth.my'], exact: true })
    assert.equal(await mySpace.getAttribute('href'), '/my')
    const mySpaceBox = await mySpace.boundingBox()
    assert.ok(mySpaceBox.width >= 44 && mySpaceBox.height >= 44)
    if (width <= 390) {
      for (const item of [publicHeader.locator('.brand'), publicHeader.getByRole('combobox')]) {
        const box = await item.boundingBox()
        assert.ok(Math.abs(box.y + box.height / 2 - (mySpaceBox.y + mySpaceBox.height / 2)) <= 1, `${lang}/${width}/public header`)
      }
    }
    if (width === 340) {
      assert.equal(await page.getByRole('link', { name: texts[lang]['profile.allWishes'], exact: true }).getAttribute('href'), '/u/bob/wishes')
    }
    await page.getByRole('button', { name: texts[lang]['social.follow'], exact: true }).click()
    await page.getByRole('button', { name: texts[lang]['social.unfollow'], exact: true }).waitFor()
    const menuToggle = page.getByRole('button', { name: texts[lang]['social.profileActions'], exact: true })
    const blockAction = page.getByRole('button', { name: texts[lang]['social.block'], exact: true })
    const reportAction = page.getByRole('button', { name: texts[lang]['social.report'], exact: true })
    const isFocused = locator => locator.evaluate(el => document.activeElement === el)
    const outlineStyle = locator => locator.evaluate(el => getComputedStyle(el).outlineStyle)
    assert.equal(await menuToggle.getAttribute('aria-expanded'), 'false')
    assert.equal(await blockAction.count(), 0)
    assert.equal(await reportAction.count(), 0)
    await menuToggle.press('Enter')
    assert.equal(await menuToggle.getAttribute('aria-expanded'), 'true')
    const menuBox = await menuToggle.boundingBox()
    assert.ok(menuBox.width >= 44 && menuBox.height >= 44)
    for (const action of [blockAction, reportAction]) {
      const box = await action.boundingBox()
      assert.ok(box.height >= 44 && box.x >= 0 && box.x + box.width <= width)
    }
    await page.keyboard.press('Tab')
    assert.equal(await isFocused(blockAction), true)
    assert.equal(await outlineStyle(blockAction), 'solid')
    await page.keyboard.press('Tab')
    assert.equal(await isFocused(reportAction), true)
    await page.keyboard.press('Shift+Tab')
    assert.equal(await isFocused(blockAction), true)
    await page.keyboard.press('Escape')
    assert.equal(await isFocused(menuToggle), true)
    assert.equal(await menuToggle.getAttribute('aria-expanded'), 'false')
    if (lang === 'ru' && [390, 1440].includes(width)) {
      await page.screenshot({ path: fileURLToPath(new URL(`../../.cache/social-screens/public-profile-header-${width}.png`, import.meta.url)) })
    }
    await menuToggle.press('Space')
    await blockAction.press('Enter')
    const blockDialog = page.getByRole('dialog', { name: texts[lang]['social.block'] })
    await blockDialog.waitFor()
    await page.keyboard.press('Escape')
    await blockDialog.waitFor({ state: 'hidden' })
    assert.equal(state.blocked, false)
    assert.equal(await isFocused(menuToggle), true)
    await menuToggle.press('Enter')
    await blockAction.press('Enter')
    await blockDialog.getByRole('button', { name: texts[lang]['social.block'], exact: true }).click()
    await page.getByText(texts[lang]['social.blockedNotice'], { exact: true }).waitFor()
    await menuToggle.press('Enter')
    await page.getByRole('button', { name: texts[lang]['social.unblock'], exact: true }).press('Enter')
    await page.getByText(texts[lang]['social.unblockedNotice'], { exact: true }).waitFor()
    await menuToggle.press('Enter')
    await reportAction.press('Enter')
    const reportDialog = page.getByRole('dialog', { name: texts[lang]['social.report'] })
    await reportDialog.getByLabel(texts[lang]['social.reportReason']).fill('Problem')
    await reportDialog.getByRole('button', { name: texts[lang]['social.report'], exact: true }).click()
    await page.getByText(texts[lang]['social.reportSaved']).waitFor()
    assert.deepEqual(reports, ['Problem'])
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/public`)
    if (lang === 'en' && width === 320) {
      failDetails = true; await page.reload(); await page.getByText(texts[lang]['social.detailsError']).waitFor()
      await page.getByRole('button', { name: texts[lang]['checkAgain'] }).click()
      await page.getByText('A public description').waitFor()
      failState = true; await page.reload(); await page.getByText(texts[lang]['social.stateError']).waitFor()
      await page.getByRole('button', { name: texts[lang]['checkAgain'] }).click()
      await page.getByRole('button', { name: texts[lang]['social.follow'], exact: true }).waitFor()
    }
    const nav = page.getByRole('navigation', { name: texts[lang]['social.profileNavigation'] })
    const listsLink = nav.getByRole('link', { name: texts[lang]['social.wishLists'], exact: true })
    const followersLink = nav.getByRole('link', { name: texts[lang]['social.followers'], exact: true })
    const followingLink = nav.getByRole('link', { name: texts[lang]['social.following'], exact: true })
    const backLink = page.getByRole('link', { name: texts[lang]['social.backToProfile'].replace('{username}', '@bob'), exact: true })
    const active = nav.locator('[aria-current="page"]')
    const followersHeading = page.getByRole('heading', { name: texts[lang]['social.followers'], exact: true, level: 1 })
    const followingHeading = page.getByRole('heading', { name: texts[lang]['social.following'], exact: true, level: 1 })
    await followingLink.waitFor()
    await menuToggle.click()
    await profileHeading.click()
    assert.equal(await menuToggle.getAttribute('aria-expanded'), 'false')
    assert.equal(await active.getAttribute('href'), '/u/bob')
    assert.deepEqual(await nav.getByRole('link').evaluateAll(links => links.map(link => link.getAttribute('href'))), ['/u/bob', '/u/bob/followers', '/u/bob/following'])
    await followersLink.press('Enter')
    await page.waitForURL(`${origin}/u/bob/followers`)
    await followersHeading.and(page.locator(':focus')).waitFor()
    assert.equal(await outlineStyle(followersHeading), 'none')
    await page.getByText(texts[lang]['social.empty.followers'], { exact: true }).waitFor()
    assert.equal(await followersLink.getAttribute('aria-current'), 'page')
    assert.equal(await backLink.getAttribute('href'), '/u/bob')
    await followingLink.click()
    await page.waitForURL(`${origin}/u/bob/following`)
    await followingHeading.and(page.locator(':focus')).waitFor()
    assert.equal(await outlineStyle(followingHeading), 'none')
    assert.equal(await followingLink.getAttribute('aria-current'), 'page')
    await page.reload()
    assert.equal(await active.getAttribute('href'), '/u/bob/following')
    assert.equal(await isFocused(followingHeading), false)
    await page.goBack()
    await page.waitForURL(`${origin}/u/bob/followers`)
    assert.equal(await active.getAttribute('href'), '/u/bob/followers')
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/public followers`)
    assert.equal(await nav.getByRole('link').evaluateAll(links => links.every(link => link.offsetHeight >= 44 && link.offsetWidth >= 44)), true)
    if (lang === 'ru' && [390, 1440].includes(width)) {
      await page.screenshot({ path: fileURLToPath(new URL(`../../.cache/social-screens/public-followers-${width}.png`, import.meta.url)) })
    }
    await backLink.click()
    await page.waitForURL(`${origin}/u/bob`)
    await profileHeading.and(page.locator(':focus')).waitFor()
    assert.equal(await outlineStyle(profileHeading), 'none')
    assert.equal(await listsLink.getAttribute('aria-current'), 'page')
    if (lang === 'ru' && [390, 1440].includes(width)) {
      await page.screenshot({ path: fileURLToPath(new URL(`../../.cache/social-screens/public-profile-navigation-${width}.png`, import.meta.url)) })
    }
    if (width === 320) {
      state = { ...state, self: true }
      const selfState = page.waitForResponse('**/api/social/bob/state')
      await page.reload()
      await selfState
      await profileHeading.waitFor()
      assert.equal(await menuToggle.count(), 0)
      guest = true
      showFollowers = false
      await page.goto(`${origin}/u/bob/following`)
      await page.getByRole('heading', { name: texts[lang]['social.following'], exact: true }).waitFor()
      assert.equal(await active.getAttribute('href'), '/u/bob/following')
      assert.equal(await followersLink.count(), 0)
      await listsLink.click()
      await page.waitForURL(`${origin}/u/bob`)
      await followingLink.waitFor()
      assert.equal(await followersLink.count(), 0)
      await page.getByRole('link', { name: texts[lang]['social.signIn'], exact: true }).waitFor()
      assert.equal(await menuToggle.count(), 0)
      showFollowing = false
      await page.goto(`${origin}/u/bob/followers`)
      await page.getByText(texts[lang]['social.unavailableList'], { exact: true }).waitFor()
      assert.equal(await nav.getByRole('link').count(), 1)
      assert.equal(await active.count(), 0)
      assert.equal(await backLink.getAttribute('href'), '/u/bob')
    }
    showFollowers = showFollowing = true
    if (width === 320 || (lang === 'ru' && [390, 1440].includes(width))) {
      const peoplePage = page.locator('.people-page')
      const empty = peoplePage.getByText(texts[lang]['social.empty.followers'], { exact: true })
      const loadError = peoplePage.getByText(texts[lang]['social.loadError'], { exact: true })
      const detailsError = peoplePage.getByText(texts[lang]['social.detailsError'], { exact: true })
      const notAvailable = peoplePage.getByText(texts[lang]['social.unavailableList'], { exact: true })
      const retry = peoplePage.getByRole('button', { name: texts[lang]['checkAgain'], exact: true })
      const more = peoplePage.getByRole('button', { name: texts[lang]['search.more'], exact: true })
      const gate = Promise.withResolvers()
      pendingPeople = gate.promise
      peopleStatus = 503
      await page.goto(`${origin}/u/bob/followers`)
      await peoplePage.getByRole('status').filter({ hasText: texts[lang]['social.loading'] }).waitFor({ state: 'attached' })
      assert.equal(await peoplePage.locator('.people-skeleton').isVisible(), true)
      assert.equal(await empty.count(), 0)
      pendingPeople = undefined
      gate.resolve()
      await loadError.waitFor()
      assert.equal(await empty.count(), 0)
      if (lang === 'ru' && [390, 1440].includes(width)) {
        await page.screenshot({ path: fileURLToPath(new URL(`../../.cache/social-screens/people-error-${width}.png`, import.meta.url)) })
      }
      peopleStatus = 200
      await retry.click()
      await empty.waitFor()

      failDetails = true
      await page.reload()
      await detailsError.waitFor()
      await empty.waitFor()
      assert.equal(await followingLink.count(), 0)
      await retry.click()
      await followingLink.waitFor()
      assert.equal(await active.getAttribute('href'), '/u/bob/followers')

      publicPeople = [{ username: 'first' }]
      nextCursor = 'first'
      await page.reload()
      await more.waitFor()
      const first = peoplePage.getByRole('link', { name: 'first @first', exact: true })
      peopleStatus = 503
      await more.click()
      await loadError.waitFor()
      assert.equal(await first.isVisible(), true)
      peopleStatus = 200
      publicPeople = [{ username: 'second' }]
      nextCursor = 'second'
      await retry.click()
      await peoplePage.getByRole('link', { name: 'second @second', exact: true }).waitFor()
      assert.equal(await first.count(), 1)
      assert.deepEqual(publicCursors.slice(-2), ['first', 'first'])
      peopleStatus = 404
      await more.click()
      await notAvailable.waitFor()
      assert.equal(await first.count(), 0)
      assert.equal(await followersLink.count(), 0)
      assert.equal(await followingLink.isVisible(), true)
      assert.equal(await backLink.getAttribute('href'), '/u/bob')
      assert.equal(await empty.count(), 0)
      peopleStatus = 200
      nextCursor = ''

      if (lang === 'en' && width === 320) {
        peopleStatus = 404
        await page.route('**/api/public/profiles/bob/details', route => route.fulfill({ status: 404, json: { error: 'list_not_found' } }), { times: 1 })
        const detailsResponse = page.waitForResponse('**/api/public/profiles/bob/details')
        await page.reload()
        await detailsResponse
        await notAvailable.waitFor()
        assert.equal(await retry.count(), 0)
        assert.equal(await nav.getByRole('link').count(), 1)
        peopleStatus = 200

        guest = false
        sessionFailure = true
        await page.goto(`${origin}/my/following`)
        await loadError.waitFor()
        assert.equal(await peoplePage.locator('.people-skeleton').count(), 0)
        sessionFailure = false
        await retry.click()
        await peoplePage.getByRole('link', { name: 'Bob Example @bob', exact: true }).waitFor()
      }
    }
    const longPerson = { username: 'a'.repeat(30), display_name: 'ДлинноеИмя'.repeat(10), has_avatar: false }
    publicPeople = [{ username: 'bob', has_avatar: true }, longPerson]
    for (const section of ['followers', 'following']) {
      await page.goto(`${origin}/u/bob/${section}`)
      const people = page.getByRole('main').getByRole('list').filter({ hasText: longPerson.display_name })
      const shortRow = people.getByRole('link', { name: 'bob @bob', exact: true })
      const longRow = people.getByRole('link', { name: `${longPerson.display_name} @${longPerson.username}`, exact: true })
      await shortRow.waitFor()
      await page.waitForFunction(() => document.querySelector('.person-row img')?.naturalWidth > 0)
      assert.equal(await longRow.locator('.avatar').textContent(), 'Д')
      assert.equal(await longRow.getAttribute('href'), `/u/${longPerson.username}`)
      assert.equal(await people.getByRole('link').evaluateAll(links => links.every(link => link.offsetWidth >= 44 && link.offsetHeight >= 44 && link.scrollWidth <= link.clientWidth)), true)
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, `${lang}/${width}/${section} rows`)
      if (section === 'followers') {
        const box = await shortRow.boundingBox()
        await shortRow.click({ position: { x: box.width - 5, y: box.height / 2 } })
      } else {
        if (lang === 'ru' && [390, 1440].includes(width)) {
          await page.screenshot({ path: fileURLToPath(new URL(`../../.cache/social-screens/people-${width}.png`, import.meta.url)) })
        }
        await shortRow.press('Enter')
      }
      await page.waitForURL(`${origin}/u/bob`)
      await profileHeading.and(page.locator(':focus')).waitFor()
      await page.goBack()
      await page.waitForURL(`${origin}/u/bob/${section}`)
      await shortRow.and(page.locator(':focus')).waitFor()
      await page.keyboard.press('Tab')
      await longRow.and(page.locator(':focus-visible')).waitFor()
      assert.equal(await outlineStyle(longRow), 'solid')
      await page.keyboard.press('Shift+Tab')
      await shortRow.and(page.locator(':focus-visible')).waitFor()
    }
    if (lang === 'en' && width === 340) {
      await page.getByRole('link', { name: 'bob @bob', exact: true }).press('Enter')
      await profileHeading.and(page.locator(':focus')).waitFor()
      publicPeople = []
      await page.goBack()
      await followingHeading.and(page.locator(':focus')).waitFor()
    }
    await context.close()
  }
  assert.deepEqual(errors, [])
  console.log('Profile and social: EN/RU/RO × 320/340/390/768/1024/1440 passed, including public navigation and social list states.')
} finally { await browser.close() }
