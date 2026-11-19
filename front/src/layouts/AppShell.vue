<script setup>
import { computed, ref, watch } from 'vue'
import { locale, t } from '../i18n.js'
import { getCollectionSetting, setCollectionSetting } from '../api/lists.js'
import { getProfileName, setProfileName } from '../api/search.js'
import { googleStatus, startGoogle, facebookStatus, startFacebook, addEmail, confirmAddedEmail, resendCode } from '../api/auth.js'
import { useSession } from '../composables/useSession.js'
import { authError, useAuthForm } from '../composables/useAuthForm.js'
import AppNavigation from '../components/navigation/AppNavigation.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import LanguagePicker from '../components/ui/LanguagePicker.vue'
import UserAvatar from '../components/ui/UserAvatar.vue'
import UiDialog from '../components/ui/UiDialog.vue'
import UiButton from '../components/ui/UiButton.vue'
import CookieSettingsLink from '../components/ui/CookieSettingsLink.vue'
import { analyticsConfigured } from '../analytics.js'
import SocialAuthButtons from '../components/auth/SocialAuthButtons.vue'
import CodeVerification from '../components/auth/CodeVerification.vue'
import TextField from '../components/ui/TextField.vue'
import logo from '../assets/brand/logo-lockup.svg'

const { user, checking, unavailable, signOut, refresh } = useSession()
defineEmits(['cookie-settings'])
const account = computed(() => !checking.value && !unavailable.value ? user.value : null)
const googleResult = new URLSearchParams(window.location.search).get('google')
const facebookResult = new URLSearchParams(window.location.search).get('facebook')
const profileOpen = ref(['linked', 'error', 'conflict'].includes(googleResult) || ['linked', 'error', 'conflict'].includes(facebookResult) || new URLSearchParams(window.location.search).has('add_email'))
const googleEnabled = ref(false), googleLinked = ref(false), googleBusy = ref(false), googleError = ref('')
const facebookEnabled = ref(false), facebookLinked = ref(false), facebookBusy = ref(false), facebookError = ref('')
const emailInput = ref(''), emailChallenge = ref('')
const { busy: emailBusy, error: emailError, remaining: emailRemaining, cooldown: emailCooldown, run: emailRun } = useAuthForm()
const signingOut = ref(false)
const error = ref('')
const collectionEnabled = ref(false), collectionLoading = ref(false), collectionReady = ref(false), collectionError = ref('')
const displayName = ref(''), showDisplayName = ref(false), nameReady = ref(false), nameLoading = ref(false), nameError = ref(''), nameSaved = ref(false)
watch(profileOpen, async open => {
  if (!open) return
  nameLoading.value = true; nameReady.value = false; nameError.value = ''; nameSaved.value = false
  try { const data = await getProfileName(); displayName.value = data.display_name; showDisplayName.value = data.show_display_name; nameReady.value = true }
  catch { nameError.value = t('profile.nameError') }
  finally { nameLoading.value = false }
  try { const status = await googleStatus(); googleEnabled.value = status.enabled; googleLinked.value = status.linked }
  catch { googleEnabled.value = false }
  try { const status = await facebookStatus(); facebookEnabled.value = status.enabled; facebookLinked.value = status.linked }
  catch { facebookEnabled.value = false }
  collectionLoading.value = true; collectionReady.value = false; collectionError.value = ''
  try { collectionEnabled.value = (await getCollectionSetting()).show_all_wishes; collectionReady.value = true }
  catch { collectionError.value = t('profile.settingsError') }
  finally { collectionLoading.value = false }
}, { immediate: true })
watch(displayName, value => { if (!value.trim()) showDisplayName.value = false; nameSaved.value = false })
async function saveName() {
  if (nameLoading.value) return
  nameError.value = ''; nameSaved.value = false
  if ([...displayName.value.trim()].length > 100) { nameError.value = t('profile.nameTooLong'); return }
  nameLoading.value = true
  try { const data = await setProfileName(displayName.value, showDisplayName.value); displayName.value = data.display_name; showDisplayName.value = data.show_display_name; nameSaved.value = true }
  catch { nameError.value = t('profile.nameError') }
  finally { nameLoading.value = false }
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
async function changeCollection(event) {
  const next = event.target.checked
  collectionLoading.value = true; collectionError.value = ''
  try { collectionEnabled.value = (await setCollectionSetting(next)).show_all_wishes }
  catch { collectionReady.value = false; collectionError.value = t('profile.settingsError'); event.target.checked = collectionEnabled.value }
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
      <AppNavigation :profile-available="!!account" @profile="profileOpen = true" />
      <div class="sidebar-note">
        <AppIcon name="heart" />
        <p>{{ t('my.sidebarNote') }}</p>
      </div>
      <div v-if="account" class="sidebar-account">
        <button class="account-link" type="button" :aria-label="t('my.openAccount')" @click="profileOpen = true">
          <UserAvatar :username="account.username" />
          <span class="account-text">
            <strong>{{ account.username }}</strong>
            <span>{{ t('my.personalSpace') }}</span>
          </span>
        </button>
      </div>
    </aside>

    <div class="workspace">
      <header class="app-header">
        <a class="brand mobile-brand" href="/my" :aria-label="t('appName')">
          <img :src="logo" :alt="t('appName')" width="118" height="36" />
        </a>
        <span class="header-title">{{ t('my.personalSpace') }}</span>
        <div class="header-actions">
          <LanguagePicker />
          <button class="profile-button" type="button" :disabled="!account" :aria-label="t('my.openAccount')" @click="profileOpen = true">
            <UserAvatar :username="account?.username" />
            <span v-if="account" class="header-username">{{ account.username }}</span>
            <AppIcon class="profile-chevron" name="chevron" />
          </button>
        </div>
      </header>
      <main id="main-content" class="app-content" :class="{ 'app-content-with-footer': analyticsConfigured }" tabindex="-1">
        <slot />
      </main>
      <footer v-if="analyticsConfigured" class="app-footer"><CookieSettingsLink @open="$emit('cookie-settings')" /></footer>
    </div>

    <div class="bottom-navigation">
      <AppNavigation :profile-available="!!account" @profile="profileOpen = true" />
    </div>

    <UiDialog v-model:open="profileOpen" :title="t('my.profile')">
      <template v-if="account">
        <div class="profile-identity">
          <UserAvatar :username="account.username" />
          <div>
            <strong>{{ account.username }}</strong>
            <p>{{ t('my.accountDetails') }}</p>
          </div>
        </div>
        <dl>
          <dt>{{ t('register.username') }}</dt>
          <dd>{{ account.username }}</dd>
          <dt>{{ t('login.email') }}</dt>
          <dd>{{ account.email || t('profile.noEmail') }}</dd>
        </dl>
        <form class="name-setting" @submit.prevent="saveName">
          <TextField id="profile-display-name" v-model="displayName" :label="t('profile.displayName')" :hint="t('profile.nameHint')" maxlength="200" :disabled="nameLoading || !nameReady" />
          <label class="name-visibility"><input v-model="showDisplayName" type="checkbox" :disabled="nameLoading || !nameReady || !displayName.trim()" /><strong>{{ t('profile.showName') }}</strong></label>
          <p class="name-hint">{{ t('profile.showNameHint') }}</p>
          <UiButton type="submit" variant="secondary" :loading="nameLoading" :disabled="!nameReady">{{ t('profile.saveName') }}</UiButton>
          <p v-if="nameSaved" role="status">{{ t('profile.nameSaved') }}</p>
          <p v-if="nameError" class="auth-error" role="alert">{{ nameError }}</p>
        </form>
        <div v-if="!account.email" class="email-setting">
          <p>{{ t('profile.emailForMail') }}</p>
          <template v-if="emailChallenge"><p>{{ t('reservation.codeSent') }} {{ emailInput }}</p><CodeVerification :busy="emailBusy" :remaining="emailRemaining" @verify="verifyAddedEmail" @resend="resendEmailCode" /></template>
          <form v-else @submit.prevent="sendEmailCode">
            <TextField id="profile-email" v-model="emailInput" type="email" required maxlength="254" autocomplete="email" :label="t('login.email')" />
            <UiButton type="submit" :loading="emailBusy">{{ t('profile.addEmail') }}</UiButton>
          </form>
          <p v-if="emailError" class="auth-error" role="alert">{{ emailError }}</p>
        </div>
        <div v-if="googleEnabled" class="google-setting">
          <strong>{{ t('google.methods') }}</strong>
          <p v-if="googleLinked" role="status">{{ t('google.linked') }}</p>
          <SocialAuthButtons v-else :busy="googleBusy" :label="t('google.connect')" @select="linkGoogle" />
          <p v-if="googleResult === 'error' || googleResult === 'conflict'" class="auth-error" role="alert">{{ t(googleResult === 'conflict' ? 'google.linkConflict' : 'google.error') }}</p>
          <p v-if="googleError" class="auth-error" role="alert">{{ googleError }}</p>
        </div>
        <div v-if="facebookEnabled" class="google-setting">
          <strong v-if="!googleEnabled">{{ t('google.methods') }}</strong>
          <p v-if="facebookLinked" role="status">{{ t('facebook.linked') }}</p>
          <SocialAuthButtons v-else :google="false" facebook :busy="facebookBusy" :label="t('facebook.connect')" @select="linkFacebook" />
          <p v-if="facebookResult === 'error' || facebookResult === 'conflict'" class="auth-error" role="alert">{{ t(facebookResult === 'conflict' ? 'facebook.linkConflict' : 'facebook.error') }}</p>
          <p v-if="facebookError" class="auth-error" role="alert">{{ facebookError }}</p>
        </div>
        <div class="collection-setting">
          <label><input type="checkbox" :checked="collectionEnabled" :disabled="collectionLoading || !collectionReady" @change="changeCollection" /><strong>{{ t('profile.showAll') }}</strong></label>
          <p>{{ t('profile.showAllWarning') }}</p>
          <p v-if="collectionError" class="auth-error" role="alert">{{ collectionError }}</p>
        </div>
        <UiButton class="profile-public-link" variant="secondary" href="/my/following">{{ t('social.title') }}</UiButton>
        <UiButton class="profile-public-link" variant="secondary" href="/my/profile">{{ t('profile.editProfile') }}</UiButton>
        <UiButton class="profile-public-link" variant="secondary" :href="`/u/${encodeURIComponent(account.username)}`">{{ t('lists.publicProfile') }}</UiButton>
        <UiButton class="logout-button" variant="secondary" :loading="signingOut" @click="logout">
          <AppIcon name="logout" />{{ t('auth.logout') }}
        </UiButton>
        <p v-if="error" class="auth-error" role="alert">{{ error }}</p>
      </template>
    </UiDialog>
  </div>
</template>

<style scoped>
.app-shell { min-height: 100svh; }
.sidebar { display: none; }
.workspace { display: flex; flex-direction: column; min-width: 0; min-height: 100svh; }
.app-header { display: flex; align-items: center; justify-content: space-between; gap: var(--wx-space-2); min-height: 80px; padding: var(--wx-space-4); border-bottom: 1px solid var(--wx-border); background: var(--wx-surface); }
.brand { display: inline-flex; align-items: center; flex-shrink: 0; min-height: 44px; width: fit-content; border-radius: var(--wx-radius-sm); }
.brand img { display: block; max-width: 100%; height: auto; }
.header-title { display: none; color: var(--wx-text-muted); font-size: var(--wx-text-sm); }
.header-actions { display: flex; align-items: center; gap: var(--wx-space-1); min-width: 0; }
.profile-button, .account-link { display: flex; align-items: center; gap: var(--wx-space-2); min-width: 44px; min-height: 44px; padding: 0; border: 0; border-radius: var(--wx-radius-md); color: var(--wx-text); background: transparent; cursor: pointer; }
.profile-button:hover:not(:disabled), .account-link:hover { background: var(--wx-background); }
.profile-button:active:not(:disabled), .account-link:active { background: var(--wx-lavender); }
.profile-button:disabled { opacity: .55; cursor: default; }
.header-username, .profile-chevron { display: none; }
.app-content { flex: 1; width: 100%; max-width: var(--wx-content-max); margin-inline: auto; padding: var(--wx-space-6) var(--wx-space-4) calc(104px + env(safe-area-inset-bottom)); }
.app-content-with-footer { padding-bottom: var(--wx-space-6); }
.app-footer { width: 100%; max-width: var(--wx-content-max); margin-inline: auto; padding: 0 var(--wx-space-4) calc(128px + env(safe-area-inset-bottom)); }
.app-footer :deep(.cookie-settings-link) { scroll-margin-bottom: calc(128px + env(safe-area-inset-bottom)); }
.bottom-navigation { position: fixed; z-index: 5; inset: auto 0 0; padding: var(--wx-space-2) var(--wx-space-2) max(var(--wx-space-2), env(safe-area-inset-bottom)); border-top: 1px solid var(--wx-border); background: var(--wx-surface); }
.profile-identity { display: flex; align-items: center; gap: var(--wx-space-3); min-width: 0; }
.profile-identity strong { line-height: 1.5; }
.profile-identity p { margin: var(--wx-space-1) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); }
dl { margin: var(--wx-space-6) 0; padding: var(--wx-space-5); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); }
dt { margin-bottom: var(--wx-space-1); color: var(--wx-text-muted); font-size: var(--wx-text-xs); }
dd { margin: 0; line-height: 1.5; }
dd + dt { margin-top: var(--wx-space-4); }
.profile-note { margin-block: var(--wx-space-5); color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.6; }
.collection-setting { margin-block: var(--wx-space-5); }.collection-setting label { display: flex; align-items: center; gap: var(--wx-space-3); min-height: 44px; cursor: pointer; }.collection-setting input { width: 20px; height: 20px; accent-color: var(--wx-primary); }.collection-setting p { margin: var(--wx-space-1) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.5; }
.name-setting { display: grid; gap: var(--wx-space-3); margin-block: var(--wx-space-5); }.name-setting p { margin: 0; }.name-visibility { display: flex; align-items: center; gap: var(--wx-space-3); min-height: 44px; cursor: pointer; }.name-visibility input { width: 20px; height: 20px; accent-color: var(--wx-primary); }.name-hint { color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.5; }
.google-setting { display: grid; gap: var(--wx-space-3); margin-block: var(--wx-space-5); }.google-setting p { margin: 0; line-height: 1.5; }
.email-setting, .email-setting form { display: grid; gap: var(--wx-space-3); margin-block: var(--wx-space-4); }.email-setting p { margin: 0; line-height: 1.5; }
.logout-button { width: 100%; }
.profile-public-link { width: 100%; margin-bottom: var(--wx-space-3); }
@media (min-width: 48rem) {
  .app-header { min-height: 88px; padding-inline: var(--wx-space-6); }
  .header-actions { gap: var(--wx-space-4); }
  .header-username { display: inline; max-width: 160px; overflow: hidden; text-overflow: ellipsis; font-size: var(--wx-text-sm); font-weight: 600; }
  .profile-chevron { display: block; }
  .profile-button { padding-right: var(--wx-space-2); }
  .app-content { padding: var(--wx-space-8) var(--wx-space-6) calc(104px + env(safe-area-inset-bottom)); }
  .app-content-with-footer { padding-bottom: var(--wx-space-6); }
  .app-footer { padding-inline: var(--wx-space-6); }
}
@media (min-width: 64rem) {
  .app-shell { display: grid; grid-template-columns: var(--wx-sidebar-width) minmax(0, 1fr); width: min(100%, 1600px); margin-inline: auto; border-inline: 1px solid var(--wx-border); }
  .sidebar { position: sticky; top: 0; display: flex; flex-direction: column; gap: var(--wx-space-8); height: 100svh; padding: var(--wx-space-5) var(--wx-space-4); border-right: 1px solid var(--wx-border); background: var(--wx-surface); }
  .sidebar > .brand { margin: 0 var(--wx-space-3) var(--wx-space-4); }
  .sidebar-note { margin-top: auto; padding: var(--wx-space-5); border-radius: var(--wx-radius-card); background: linear-gradient(135deg, var(--wx-background), var(--wx-lavender)); }
  .sidebar-note .icon { color: var(--wx-primary); }
  .sidebar-note p { margin: var(--wx-space-2) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.6; }
  .sidebar-account { padding-top: var(--wx-space-4); border-top: 1px solid var(--wx-border); }
  .account-link { width: 100%; gap: var(--wx-space-3); padding: var(--wx-space-2); text-align: left; }
  .account-text { display: grid; gap: var(--wx-space-1); min-width: 0; }
  .account-text strong { overflow: hidden; text-overflow: ellipsis; font-size: var(--wx-text-sm); }
  .account-text > span { color: var(--wx-text-muted); font-size: var(--wx-text-xs); }
  .mobile-brand { display: none; }
  .header-title { display: block; }
  .app-header { padding-inline: var(--wx-space-8); }
  .app-content { padding: var(--wx-space-8); }
  .app-footer { padding: 0 var(--wx-space-8) var(--wx-space-6); }
  .bottom-navigation { display: none; }
}
</style>
