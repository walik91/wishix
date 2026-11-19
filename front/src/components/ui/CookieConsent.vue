<script setup>
import { nextTick, ref } from 'vue'
import { t } from '../../i18n.js'
import { analyticsConfigured, analyticsConsent, setAnalyticsConsent } from '../../analytics.js'
import UiButton from './UiButton.vue'

const settingsOpen = ref(false)
const selection = ref(analyticsConsent.value !== 'denied')
const panel = ref(null)
function showSettings() { selection.value = analyticsConsent.value !== 'denied'; settingsOpen.value = true; nextTick(() => panel.value?.focus()) }
function save(value) { settingsOpen.value = false; setAnalyticsConsent(value) }
defineExpose({ showSettings })
</script>

<template>
  <div v-if="analyticsConfigured" class="cookie-controls">
    <section v-if="analyticsConsent === null || settingsOpen" ref="panel" class="cookie-panel" :aria-label="t('cookies.title')" role="region" tabindex="-1">
      <h2>{{ t('cookies.title') }}</h2>
      <p>{{ t('cookies.description') }}</p>
      <template v-if="settingsOpen">
        <div class="cookie-option"><div><strong>{{ t('cookies.essential') }}</strong><p>{{ t('cookies.essentialDescription') }}</p></div><span>{{ t('cookies.always') }}</span></div>
        <label class="cookie-option"><div><strong>{{ t('cookies.analytics') }}</strong><p>{{ t('cookies.analyticsDescription') }}</p></div><input v-model="selection" type="checkbox" /></label>
        <div class="cookie-actions"><UiButton @click="save(true)">{{ t('cookies.accept') }}</UiButton><UiButton variant="secondary" @click="save(selection)">{{ t('cookies.save') }}</UiButton></div>
      </template>
      <div v-else class="cookie-actions"><UiButton @click="save(true)">{{ t('cookies.accept') }}</UiButton><UiButton variant="secondary" @click="showSettings">{{ t('cookies.settings') }}</UiButton></div>
    </section>
  </div>
</template>

<style scoped>
.cookie-controls { position: fixed; z-index: 8; left: max(12px, env(safe-area-inset-left)); bottom: calc(88px + env(safe-area-inset-bottom)); width: min(420px, calc(100vw - 24px)); }
.cookie-panel { max-height: min(70svh, 600px); overflow: auto; padding: var(--wx-space-5); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); background: var(--wx-surface); box-shadow: 0 12px 28px rgba(42,29,91,.15); }
.cookie-panel h2 { margin: 0; font-size: var(--wx-text-lg); }.cookie-panel p { margin: var(--wx-space-2) 0 var(--wx-space-4); color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.5; }
.cookie-actions { display: flex; flex-wrap: wrap; gap: var(--wx-space-2); }.cookie-actions > * { flex: 1 1 auto; }
.cookie-option { display: flex; align-items: center; justify-content: space-between; gap: var(--wx-space-3); min-height: 48px; margin-bottom: var(--wx-space-3); }.cookie-option p { margin: var(--wx-space-1) 0 0; }.cookie-option span { color: var(--wx-text-muted); font-size: var(--wx-text-xs); }.cookie-option input { flex: 0 0 24px; width: 24px; height: 24px; accent-color: var(--wx-primary); }
@media (min-width: 64rem) { .cookie-controls { bottom: max(16px, env(safe-area-inset-bottom)); } }
</style>
