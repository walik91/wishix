<script setup>
import { computed, ref } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { authError } from '../composables/useAuthForm.js'
import AppNavigation from '../components/navigation/AppNavigation.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import LanguagePicker from '../components/ui/LanguagePicker.vue'
import UserAvatar from '../components/ui/UserAvatar.vue'
import UiDialog from '../components/ui/UiDialog.vue'
import UiButton from '../components/ui/UiButton.vue'
import logo from '../assets/brand/logo-lockup.svg'

const { user, checking, unavailable, signOut } = useSession()
const account = computed(() => !checking.value && !unavailable.value ? user.value : null)
const profileOpen = ref(false)
const signingOut = ref(false)
const error = ref('')
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
      <main id="main-content" class="app-content" tabindex="-1">
        <slot />
      </main>
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
          <dd>{{ account.email }}</dd>
        </dl>
        <p class="profile-note">{{ t('my.profileSoon') }}</p>
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
.workspace { min-width: 0; }
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
.app-content { width: 100%; max-width: var(--wx-content-max); margin-inline: auto; padding: var(--wx-space-6) var(--wx-space-4) calc(104px + env(safe-area-inset-bottom)); }
.bottom-navigation { position: fixed; z-index: 5; inset: auto 0 0; padding: var(--wx-space-2) var(--wx-space-2) max(var(--wx-space-2), env(safe-area-inset-bottom)); border-top: 1px solid var(--wx-border); background: var(--wx-surface); }
.profile-identity { display: flex; align-items: center; gap: var(--wx-space-3); min-width: 0; }
.profile-identity strong { line-height: 1.5; }
.profile-identity p { margin: var(--wx-space-1) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); }
dl { margin: var(--wx-space-6) 0; padding: var(--wx-space-5); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); }
dt { margin-bottom: var(--wx-space-1); color: var(--wx-text-muted); font-size: var(--wx-text-xs); }
dd { margin: 0; line-height: 1.5; }
dd + dt { margin-top: var(--wx-space-4); }
.profile-note { margin-block: var(--wx-space-5); color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.6; }
.logout-button { width: 100%; }
.profile-public-link { width: 100%; margin-bottom: var(--wx-space-3); }
@media (min-width: 48rem) {
  .app-header { min-height: 88px; padding-inline: var(--wx-space-6); }
  .header-actions { gap: var(--wx-space-4); }
  .header-username { display: inline; max-width: 160px; overflow: hidden; text-overflow: ellipsis; font-size: var(--wx-text-sm); font-weight: 600; }
  .profile-chevron { display: block; }
  .profile-button { padding-right: var(--wx-space-2); }
  .app-content { padding: var(--wx-space-8) var(--wx-space-6) calc(104px + env(safe-area-inset-bottom)); }
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
  .bottom-navigation { display: none; }
}
</style>
