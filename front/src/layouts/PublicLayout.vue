<script setup>
import PublicHeader from '../components/navigation/PublicHeader.vue'
import SiteFooter from '../components/layout/SiteFooter.vue'
import { t } from '../i18n.js'
defineEmits(['cookie-settings'])
</script>

<template>
  <div class="public-layout">
    <a class="skip-link" href="#main-content">{{ t('home.skipToContent') }}</a>
    <div class="public-header-wrap"><PublicHeader /></div>
    <main id="main-content" class="public-content" tabindex="-1"><slot /></main>
    <SiteFooter @cookie-settings="$emit('cookie-settings')" />
  </div>
</template>

<style scoped>
.public-layout { display: flex; flex-direction: column; min-height: 100svh; }
.public-header-wrap, .public-content { width: min(100% - 32px, 1120px); margin-inline: auto; }
.public-content { display: flex; flex-direction: column; flex: 1; min-width: 0; padding-bottom: var(--wx-space-8); }
@media (min-width: 48rem) {
  .public-header-wrap, .public-content { width: min(100% - 48px, 1120px); }
}
</style>
