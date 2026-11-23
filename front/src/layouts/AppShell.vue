<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { locale, t } from '../i18n.js'
import { getCollectionSetting, setCollectionSetting } from '../api/lists.js'
import { googleStatus, startGoogle, facebookStatus, startFacebook, addEmail, confirmAddedEmail, resendCode } from '../api/auth.js'
import { useSession } from '../composables/useSession.js'
import { authError, useAuthForm } from '../composables/useAuthForm.js'
import AppNavigation from '../components/navigation/AppNavigation.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import LanguagePicker from '../components/ui/LanguagePicker.vue'
import UserAvatar from '../components/ui/UserAvatar.vue'
import UiDialog from '../components/ui/UiDialog.vue'
import UiButton from '../components/ui/UiButton.vue'
import DiscardChangesDialog from '../components/ui/DiscardChangesDialog.vue'
import SiteFooter from '../components/layout/SiteFooter.vue'
import SocialAuthButtons from '../components/auth/SocialAuthButtons.vue'
import CodeVerification from '../components/auth/CodeVerification.vue'
import TextField from '../components/ui/TextField.vue'
import ProfilePage from '../pages/ProfilePage.vue'
import logo from '../assets/brand/logo-lockup.svg'

const { user, checking, unavailable, signOut, refresh } = useSession()
defineEmits(['cookie-settings'])
const account = computed(() => !checking.value && !unavailable.value ? user.value : null)
const profileAccount = computed(() => !unavailable.value ? user.value : null)
const query = new URLSearchParams(window.location.search)
const googleResult = query.get('google')
const facebookResult = query.get('facebook')
const profileLink = window.location.pathname.replace(/\/+$/, '') === '/my/profile'
const accountLink = ['linked', 'error', 'conflict'].includes(googleResult)
  || ['linked', 'error', 'conflict'].includes(facebookResult) || query.has('add_email')
const profileOpen = ref(profileLink || accountLink)
const profileSection = ref(profileLink && !accountLink ? 'profile' : 'account')
const profileVisited = ref(profileSection.value === 'profile')
const profilePage = ref(null), discardProfileOpen = ref(false)
const settingsViewport = ref(null)
const google = ref(null), googleLoading = ref(false), googleBusy = ref(false), googleError = ref('')
const facebookEnabled = ref(false), facebookLinked = ref(false), facebookBusy = ref(false), facebookError = ref('')
const emailInput = ref(''), emailChallenge = ref('')
const { busy: emailBusy, error: emailError, remaining: emailRemaining, cooldown: emailCooldown, run: emailRun } = useAuthForm()
const signingOut = ref(false)
const error = ref('')
const collectionEnabled = ref(false), savedCollection = ref(null), collectionLoading = ref(false), collectionError = ref('')
const collectionReady = computed(() => savedCollection.value !== null)
const collectionChanged = computed(() => collectionReady.value && collectionEnabled.value !== savedCollection.value)
const profileSaving = computed(() => (collectionReady.value && collectionLoading.value) || !!profilePage.value?.busy)

watch(profileOpen, (open, _, onCleanup) => {
  if (!open) {
    profileVisited.value = false
    if (window.location.pathname.replace(/\/+$/, '') === '/my/profile') window.history.replaceState(null, '', '/my')
    return
  }
  let active = true
  onCleanup(() => { active = false })
  collectionLoading.value = true; savedCollection.value = null; collectionError.value = ''
  google.value = null; googleLoading.value = true
  facebookEnabled.value = false
  getCollectionSetting()
    .then(data => { if (active) collectionEnabled.value = savedCollection.value = data.show_all_wishes })
    .catch(() => { if (active) collectionError.value = t('profile.settingsError') })
    .finally(() => { if (active) collectionLoading.value = false })
  googleStatus()
    .then(status => { if (active) google.value = status })
    .catch(() => { if (active) google.value = null })
    .finally(() => { if (active) googleLoading.value = false })
  facebookStatus()
    .then(status => { if (active) { facebookEnabled.value = status.enabled; facebookLinked.value = status.linked } })
    .catch(() => { if (active) facebookEnabled.value = false })
}, { immediate: true })
watch(profileOpen, (open, _, onCleanup) => {
  const viewport = window.visualViewport
  if (!open || !viewport) return
  function updateViewport() {
    settingsViewport.value = {
      '--settings-viewport-height': `${viewport.height}px`,
      '--settings-viewport-top': `${viewport.offsetTop}px`,
    }
  }
  function resizeViewport() {
    updateViewport()
    nextTick(() => {
      const focused = document.activeElement
      if (focused?.closest('.profile-dialog .settings-scroll')) focused.scrollIntoView({ block: 'nearest' })
    })
  }
  updateViewport()
  viewport.addEventListener('resize', resizeViewport)
  viewport.addEventListener('scroll', updateViewport)
  onCleanup(() => {
    viewport.removeEventListener('resize', resizeViewport)
    viewport.removeEventListener('scroll', updateViewport)
  })
}, { immediate: true })
function beforeProfileClose() {
  if (!collectionChanged.value && !profilePage.value?.hasUnsavedChanges) return true
  discardProfileOpen.value = true
  return false
}
async function discardProfileChanges() {
  discardProfileOpen.value = false
  await nextTick()
  profileOpen.value = false
}
function selectProfileSection(section) {
  if (section === 'profile') profileVisited.value = true
  profileSection.value = section
}
function openSettings(section) {
  selectProfileSection(section)
  profileOpen.value = true
}
async function linkGoogle() {
  if (googleBusy.value) return
  googleBusy.value = true; googleError.value = ''
  try { const result = await startGoogle('link'); window.location.assign(result.url) }
  catch (failure) { googleError.value = authError(failure); googleBusy.value = false }
}
async function linkFacebook() {
  if (facebookBusy.value) return
  facebookBusy.value = true; facebookError.value = ''
  try { const result = await startFacebook('link'); window.location.assign(result.url) }
  catch (failure) { facebookError.value = authError(failure); facebookBusy.value = false }
}
function sendEmailCode() { return emailRun(async () => { const result = await addEmail({ email: emailInput.value, locale: locale.value }); emailChallenge.value = result.challenge; emailCooldown(result.resend_after) }) }
function resendEmailCode() { return emailRun(async () => { const result = await resendCode(emailChallenge.value); emailCooldown(result.resend_after) }) }
function verifyAddedEmail(code) { return emailRun(async () => { await confirmAddedEmail(emailChallenge.value, code); emailChallenge.value = ''; await refresh() }) }
function changeCollection(event) {
  collectionEnabled.value = event.target.checked
  saveCollection()
}
async function saveCollection() {
  if (collectionLoading.value) return
  collectionLoading.value = true; collectionError.value = ''
  try {
    const data = await setCollectionSetting(collectionEnabled.value)
    collectionEnabled.value = savedCollection.value = data.show_all_wishes
  } catch { collectionError.value = t('profile.settingsError') }
  finally { collectionLoading.value = false }
}
async function logout() {
  if (signingOut.value) return
  signingOut.value = true
  error.value = ''
  try { await signOut() }
  catch (failure) { error.value = authError(failure) }
  finally { signingOut.value = false }
}
</script>

<template>
  <div class="app-shell">
    <a class="skip-link" href="#main-content">{{ t('home.skipToContent') }}</a>
    <aside class="sidebar">
      <a class="brand" href="/my" :aria-label="t('appName')">
        <img :src="logo" :alt="t('appName')" width="147" height="45" />
      </a>
      <AppNavigation :profile-available="!!account" :profile-active="profileOpen" @profile="openSettings('profile')" />
    </aside>

    <div class="workspace">
      <header class="app-header">
        <a class="brand mobile-brand" href="/my" :aria-label="t('appName')">
          <img :src="logo" :alt="t('appName')" width="118" height="36" />
        </a>
        <div class="header-actions">
          <LanguagePicker />
          <button class="profile-button" type="button" aria-haspopup="dialog" :aria-expanded="profileOpen" :disabled="!account" :aria-label="t('my.openAccount')" @click="openSettings('account')">
            <UserAvatar :username="account?.username" />
            <span v-if="account" class="header-username">{{ account.username }}</span>
            <AppIcon class="profile-chevron" name="chevron" />
          </button>
        </div>
      </header>
      <main id="main-content" class="app-content" tabindex="-1">
        <slot />
      </main>
      <SiteFooter bottom-navigation @cookie-settings="$emit('cookie-settings')" />
    </div>

    <div class="bottom-navigation">
      <AppNavigation :profile-available="!!account" :profile-active="profileOpen" @profile="openSettings('profile')" />
    </div>

    <UiDialog v-model:open="profileOpen" class="profile-dialog" :style="settingsViewport" :busy="profileSaving" :before-close="beforeProfileClose" :title="t('profile.settings')">
      <template v-if="profileAccount">
        <nav class="profile-sections" :aria-label="t('profile.settings')">
          <div class="profile-section-list">
            <button type="button" class="profile-section" :class="{ active: profileSection === 'account' }" :aria-pressed="profileSection === 'account'" @click="selectProfileSection('account')"><AppIcon name="lock" /><span>{{ t('profile.account') }}</span></button>
            <button type="button" class="profile-section" :class="{ active: profileSection === 'profile' }" :aria-pressed="profileSection === 'profile'" @click="selectProfileSection('profile')"><AppIcon name="user" /><span>{{ t('my.profile') }}</span></button>
          </div>
        </nav>
        <div v-show="profileSection === 'account'" class="settings-scroll account-content">
          <dl class="account-details">
            <div><dt>{{ t('login.email') }}</dt><dd>{{ profileAccount.email || t('profile.noEmail') }}</dd></div>
          </dl>
          <div v-if="!profileAccount.email" class="email-setting">
            <p>{{ t('profile.emailForMail') }}</p>
            <template v-if="emailChallenge"><p>{{ t('reservation.codeSent') }} {{ emailInput }}</p><CodeVerification :busy="emailBusy" :remaining="emailRemaining" @verify="verifyAddedEmail" @resend="resendEmailCode" /></template>
            <form v-else @submit.prevent="sendEmailCode">
              <TextField id="profile-email" v-model="emailInput" type="email" required maxlength="254" autocomplete="email" :label="t('login.email')" />
              <UiButton type="submit" :loading="emailBusy">{{ t('profile.addEmail') }}</UiButton>
            </form>
            <p v-if="emailError" class="auth-error" role="alert">{{ emailError }}</p>
          </div>
          <div class="provider-setting">
            <div class="provider-row" :aria-busy="googleLoading || undefined">
              <div>
                <strong>Google</strong>
                <p v-if="!googleLoading" role="status">{{ t(google?.linked ? 'google.linked' : google?.enabled ? 'google.notLinked' : 'google.unavailable') }}</p>
              </div>
              <SocialAuthButtons v-if="google?.enabled && !google.linked" :busy="googleBusy" :label="t('google.connect')" @select="linkGoogle" />
            </div>
            <p v-if="googleResult === 'error' || googleResult === 'conflict'" class="auth-error" role="alert">{{ t(googleResult === 'conflict' ? 'google.linkConflict' : 'google.error') }}</p>
            <p v-if="googleError" class="auth-error" role="alert">{{ googleError }}</p>
          </div>
          <div v-if="facebookEnabled" class="provider-setting">
            <strong>Facebook</strong>
            <p v-if="facebookLinked" role="status">{{ t('facebook.linked') }}</p>
            <SocialAuthButtons v-else :google="false" facebook :busy="facebookBusy" :label="t('facebook.connect')" @select="linkFacebook" />
            <p v-if="facebookResult === 'error' || facebookResult === 'conflict'" class="auth-error" role="alert">{{ t(facebookResult === 'conflict' ? 'facebook.linkConflict' : 'facebook.error') }}</p>
            <p v-if="facebookError" class="auth-error" role="alert">{{ facebookError }}</p>
          </div>
          <div class="account-actions">
            <UiButton variant="secondary" :loading="signingOut" @click="logout">
              <AppIcon name="logout" />{{ t('auth.logout') }}
            </UiButton>
            <p v-if="error" class="auth-error" role="alert">{{ error }}</p>
          </div>
        </div>
        <ProfilePage v-if="profileOpen && profileVisited" v-show="profileSection === 'profile'" ref="profilePage">
          <template #collection>
            <div class="collection-setting">
              <label><input type="checkbox" :checked="collectionEnabled" :disabled="collectionLoading || !collectionReady" @change="changeCollection" /><strong>{{ t('profile.showAll') }}</strong></label>
              <p>{{ t('profile.showAllWarning') }}</p>
              <p v-if="collectionError" class="auth-error" role="alert">{{ collectionError }}</p>
              <UiButton v-if="collectionError && collectionChanged" variant="secondary" @click="saveCollection">{{ t('checkAgain') }}</UiButton>
            </div>
          </template>
          <template #links>
            <UiButton variant="ghost" :href="`/u/${encodeURIComponent(profileAccount.username)}`">{{ t('lists.publicProfile') }}</UiButton>
            <UiButton variant="ghost" href="/my/following">{{ t('profile.following') }}</UiButton>
          </template>
        </ProfilePage>
      </template>
    </UiDialog>
    <DiscardChangesDialog v-model:open="discardProfileOpen" :description="t('profile.discardDescription')" @discard="discardProfileChanges" />
  </div>
</template>

<style scoped>
.app-shell { min-height: 100svh; }
.sidebar { display: none; }
.workspace { display: flex; flex-direction: column; min-width: 0; min-height: 100svh; }
.app-header { display: flex; align-items: center; justify-content: space-between; gap: var(--wx-space-2); min-height: 80px; padding: var(--wx-space-4); border-bottom: 1px solid var(--wx-border); background: var(--wx-surface); }
.brand { display: inline-flex; align-items: center; flex-shrink: 0; min-height: 44px; width: fit-content; border-radius: var(--wx-radius-sm); }
.brand img { display: block; max-width: 100%; height: auto; }
.header-actions { display: flex; align-items: center; gap: var(--wx-space-1); min-width: 0; }
.profile-button { display: flex; align-items: center; gap: var(--wx-space-2); min-width: 44px; min-height: 44px; padding: 0; border: 0; border-radius: var(--wx-radius-md); color: var(--wx-text); background: transparent; cursor: pointer; }
.profile-button:hover:not(:disabled) { background: var(--wx-background); }
.profile-button:active:not(:disabled) { background: var(--wx-lavender); }
.profile-button:disabled { opacity: .55; cursor: default; }
.header-username, .profile-chevron { display: none; }
.app-content { flex: 1; width: 100%; max-width: var(--wx-content-max); margin-inline: auto; padding: var(--wx-space-6) var(--wx-space-4); }
.bottom-navigation {
  position: fixed;
  z-index: 5;
  inset: auto 0 0;
  padding-block: var(--wx-space-2) max(var(--wx-space-2), env(safe-area-inset-bottom));
  padding-inline: max(var(--wx-space-1), env(safe-area-inset-left)) max(var(--wx-space-1), env(safe-area-inset-right));
  border-top: 1px solid var(--wx-border);
  background: var(--wx-surface);
}
.profile-dialog {
  --settings-padding-inline: max(var(--wx-space-4), env(safe-area-inset-left), env(safe-area-inset-right));
  --settings-padding-bottom: max(var(--wx-space-3), env(safe-area-inset-bottom));
  --settings-top-gap: max(var(--wx-space-2), env(safe-area-inset-top));
  inset: calc(var(--settings-viewport-top, 0px) + var(--settings-top-gap)) 0 auto;
  height: calc(var(--settings-viewport-height, 100dvh) - var(--settings-top-gap));
  max-height: none;
  padding: 0;
  overflow: hidden;
}
.profile-dialog[open] { display: flex; flex-direction: column; }
.profile-dialog > :deep(header) { flex: none; margin: 0; padding: var(--wx-space-2) var(--settings-padding-inline); }
.profile-dialog > :deep(header h2) { font-size: var(--wx-text-lg); }
.profile-sections { flex: none; padding: 0 var(--settings-padding-inline) var(--wx-space-2); border-bottom: 1px solid var(--wx-border); }
.profile-section-list { display: flex; gap: var(--wx-space-2); }
.profile-section { display: flex; align-items: center; gap: var(--wx-space-2); min-width: 44px; min-height: 44px; padding: var(--wx-space-2) var(--wx-space-3); border: 0; border-radius: var(--wx-radius-sm); color: var(--wx-text-muted); background: transparent; font-size: var(--wx-text-sm); font-weight: 600; cursor: pointer; }
.profile-section:hover { background: var(--wx-background); }
.profile-section.active { color: var(--wx-primary); background: var(--wx-lavender); }
.profile-section .icon { width: 18px; height: 18px; }
.account-content { min-height: 0; overflow: auto; overscroll-behavior: contain; padding: var(--wx-space-4) var(--settings-padding-inline) var(--settings-padding-bottom); }
.account-details { margin: 0; padding-bottom: var(--wx-space-4); border-bottom: 1px solid var(--wx-border); font-size: var(--wx-text-sm); }
.account-details dt { margin-bottom: var(--wx-space-1); font-weight: 600; }
.account-details dd { margin: 0; line-height: 1.5; }
.provider-setting, .email-setting { padding-block: var(--wx-space-4); border-bottom: 1px solid var(--wx-border); }
.email-setting form { display: grid; gap: var(--wx-space-3); }
.collection-setting { display: grid; gap: var(--wx-space-2); padding-bottom: var(--wx-space-4); border-bottom: 1px solid var(--wx-border); }
.collection-setting .button { justify-self: start; }
.collection-setting label { display: flex; align-items: center; gap: var(--wx-space-3); min-height: 44px; cursor: pointer; font-size: var(--wx-text-sm); }
.collection-setting input { flex: 0 0 auto; width: 20px; height: 20px; accent-color: var(--wx-primary); }
.collection-setting p { margin: 0; font-size: var(--wx-text-sm); line-height: 1.5; }
.collection-setting p:not(.auth-error), .provider-row p { color: var(--wx-text-muted); }
.provider-setting { display: grid; gap: var(--wx-space-2); font-size: var(--wx-text-sm); }
.provider-setting p { margin: 0; line-height: 1.5; }
.provider-row { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: var(--wx-space-3); }
.email-setting { display: grid; gap: var(--wx-space-3); }.email-setting p { margin: 0; line-height: 1.5; }
.account-actions { padding-top: var(--wx-space-4); }
@media (min-width: 48rem) {
  .profile-dialog {
    --settings-padding-inline: var(--wx-space-6);
    --settings-padding-bottom: var(--wx-space-4);
    --settings-window-gap: clamp(8px, calc(var(--settings-viewport-height, 100dvh) / 20), 24px);
    inset: calc(var(--settings-viewport-top, 0px) + var(--settings-viewport-height, 100dvh) / 2) 0 auto;
    width: min(720px, 100% - 48px);
    height: fit-content;
    max-height: calc(var(--settings-viewport-height, 100dvh) - 2 * var(--settings-window-gap));
    margin: 0 auto;
    transform: translateY(-50%);
  }
  .app-header { min-height: 88px; padding-inline: var(--wx-space-6); }
  .header-actions { gap: var(--wx-space-4); }
  .header-username { display: inline; max-width: 160px; overflow: hidden; text-overflow: ellipsis; font-size: var(--wx-text-sm); font-weight: 600; }
  .profile-chevron { display: block; }
  .profile-button { padding-right: var(--wx-space-2); }
  .app-content { padding: var(--wx-space-8) var(--wx-space-6); }
}
@media (min-width: 64rem) {
  .app-shell { display: grid; grid-template-columns: var(--wx-sidebar-width) minmax(0, 1fr); width: min(100%, 1600px); margin-inline: auto; border-inline: 1px solid var(--wx-border); }
  .sidebar { position: sticky; top: 0; display: flex; flex-direction: column; gap: var(--wx-space-6); height: 100svh; padding: var(--wx-space-5) var(--wx-space-4); border-right: 1px solid var(--wx-border); background: var(--wx-surface); }
  .sidebar > .brand { margin-inline: var(--wx-space-3); }
  .mobile-brand, .header-username { display: none; }
  .app-header { justify-content: flex-end; padding-inline: var(--wx-space-8); }
  .app-content { padding: var(--wx-space-8); }
  .bottom-navigation { display: none; }
}
</style>
