<script setup>
import { languages, locale, t } from '../../i18n.js'
import AppIcon from './AppIcon.vue'
</script>

<template>
  <label class="language-picker">
    <AppIcon name="globe" />
    <span class="sr-only">{{ t('language') }}</span>
    <select v-model="locale">
      <option v-for="language in languages" :key="language.code" :value="language.code">{{ language.name }}</option>
    </select>
    <AppIcon name="chevron" />
  </label>
</template>

<style scoped>
.language-picker { position: relative; display: flex; align-items: center; color: var(--wx-text-muted); }
.language-picker > .icon:first-child { display: none; }
.language-picker > .icon:last-child { position: absolute; right: 8px; width: 16px; pointer-events: none; }
select { appearance: none; min-height: 44px; max-width: 120px; padding: 8px 28px 8px 8px; border: 0; border-radius: var(--wx-radius-sm); color: inherit; background: transparent; font-size: var(--wx-text-sm); cursor: pointer; }
select:hover { background: var(--wx-lavender); }
@media (min-width: 48rem) { .language-picker > .icon:first-child { display: block; } }
</style>
