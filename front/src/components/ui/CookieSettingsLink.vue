<script setup>
import { t } from '../../i18n.js'
import { analyticsConfigured } from '../../analytics.js'

defineEmits(['open'])
</script>

<template>
  <button v-if="analyticsConfigured" class="cookie-settings-link" type="button" @click="$emit('open')">{{ t('cookies.settings') }}</button>
</template>

<style scoped>
.cookie-settings-link { display: inline-flex; align-items: center; min-height: 44px; padding: 0; border: 0; border-radius: var(--wx-radius-sm); background: transparent; color: inherit; font-size: var(--wx-text-sm); text-align: left; cursor: pointer; }
.cookie-settings-link:hover { color: var(--wx-surface); text-decoration: underline; text-underline-offset: 4px; }
.cookie-settings-link:active { color: var(--wx-secondary); }
.cookie-settings-link:focus-visible { outline-color: var(--wx-surface); }
</style>
