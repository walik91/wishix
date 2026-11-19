<script setup>
import { t } from '../i18n.js'
import PublicHeader from '../components/navigation/PublicHeader.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import CookieSettingsLink from '../components/ui/CookieSettingsLink.vue'
import logo from '../assets/brand/logo-lockup.svg'
import { useSession } from '../composables/useSession.js'
const { user } = useSession()
defineEmits(['cookie-settings'])
</script>

<template>
  <div class="public-layout">
    <a class="skip-link" href="#main-content">{{ t('home.skipToContent') }}</a>
    <PublicHeader />
    <main id="main-content" class="public-content" tabindex="-1"><slot /></main>
    <footer class="public-footer">
      <a class="brand" href="/" :aria-label="t('appName')"><img :src="logo" :alt="t('appName')" width="118" height="36" loading="lazy" /></a>
      <p>{{ t('login.footer') }}</p>
      <CookieSettingsLink @open="$emit('cookie-settings')" />
      <a class="login-link" :href="user ? '/my' : '/login'">{{ t(user ? 'auth.my' : 'login.submit') }}<AppIcon name="arrow" /></a>
    </footer>
  </div>
</template>

<style scoped>
.public-layout { display: flex; flex-direction: column; width: min(100% - 32px, 1120px); min-height: 100svh; margin-inline: auto; }
.public-content { display: flex; flex-direction: column; flex: 1; min-width: 0; }
.public-footer { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--wx-space-3) var(--wx-space-6); padding: var(--wx-space-8) 0 max(var(--wx-space-8), env(safe-area-inset-bottom)); }
.brand { display: inline-flex; align-items: center; min-height: 44px; width: fit-content; max-width: 100%; border-radius: var(--wx-radius-sm); }
.brand img { display: block; max-width: 100%; height: auto; }
.public-footer p { flex-basis: 100%; order: 1; margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.6; }
.login-link { display: inline-flex; align-items: center; justify-content: center; gap: var(--wx-space-2); min-height: 44px; border-radius: var(--wx-space-1); color: var(--wx-primary); font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; text-decoration: none; }
.login-link:hover { color: var(--wx-primary-hover); text-decoration: underline; text-underline-offset: 4px; }
@media (min-width: 48rem) {
  .public-layout { width: min(100% - 48px, 1120px); }
  .public-footer p { flex-basis: auto; order: 0; }
}
</style>
