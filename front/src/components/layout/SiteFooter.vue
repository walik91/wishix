<script setup>
import { t } from '../../i18n.js'
import CookieSettingsLink from '../ui/CookieSettingsLink.vue'
import logo from '../../assets/brand/logo-lockup-light.svg'

defineProps({ bottomNavigation: Boolean })
defineEmits(['cookie-settings'])
</script>

<template>
  <footer class="site-footer" :class="{ 'with-bottom-navigation': bottomNavigation }">
    <div class="footer-content">
      <div class="footer-brand">
        <a class="brand" href="/" :aria-label="t('appName')"><img :src="logo" :alt="t('appName')" width="147" height="45" loading="lazy" /></a>
        <p>{{ t('login.footer') }}</p>
      </div>
      <div class="footer-links"><CookieSettingsLink @open="$emit('cookie-settings')" /></div>
    </div>
  </footer>
</template>

<style scoped>
.site-footer { background: var(--wx-text); color: var(--wx-lavender); }
.footer-content { display: flex; flex-direction: column; align-items: flex-start; gap: var(--wx-space-4); width: min(100%, 1120px); margin-inline: auto; padding: var(--wx-space-8) var(--wx-space-4) max(var(--wx-space-8), env(safe-area-inset-bottom)); }
.footer-brand { min-width: 0; }
.brand { display: inline-flex; align-items: center; min-height: 44px; max-width: 100%; border-radius: var(--wx-radius-sm); }
.brand img { display: block; max-width: 100%; height: auto; }
.brand:focus-visible { outline-color: var(--wx-surface); }
.footer-brand p { margin: var(--wx-space-2) 0 0; font-size: var(--wx-text-sm); line-height: 1.5; overflow-wrap: anywhere; }
.footer-links { display: flex; flex-wrap: wrap; gap: var(--wx-space-2) var(--wx-space-6); }
.with-bottom-navigation .footer-content { padding-bottom: calc(128px + env(safe-area-inset-bottom)); }
.with-bottom-navigation :deep(.cookie-settings-link) { scroll-margin-bottom: calc(128px + env(safe-area-inset-bottom)); }
@media (min-width: 48rem) { .footer-content { flex-direction: row; align-items: center; justify-content: space-between; gap: var(--wx-space-8); padding-inline: var(--wx-space-6); } }
@media (min-width: 64rem) { .with-bottom-navigation .footer-content { padding-bottom: var(--wx-space-8); } }
</style>
