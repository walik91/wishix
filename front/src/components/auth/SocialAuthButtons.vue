<script setup>
import { t } from '../../i18n.js'
import UiButton from '../ui/UiButton.vue'
import googleMark from '../../assets/brand/google-g.png'

defineEmits(['select'])
defineProps({ busy: Boolean, label: { type: String, default: '' } })
</script>

<template>
  <div class="social-buttons">
    <UiButton variant="secondary" :loading="busy" @click="$emit('select', 'google')">
      <img class="provider-icon" :src="googleMark" alt="" width="20" height="20" />
      {{ label || t('login.google') }}
    </UiButton>
  </div>
</template>

<style scoped>
@font-face { font-family: 'Google Sans'; src: url('../../assets/fonts/GoogleSans-cyrillic.woff2') format('woff2'); font-weight: 500; font-display: swap; unicode-range: U+0301, U+0400-045F, U+0490-0491, U+04B0-04B1, U+2116; }
@font-face { font-family: 'Google Sans'; src: url('../../assets/fonts/GoogleSans-latin-ext.woff2') format('woff2'); font-weight: 500; font-display: swap; unicode-range: U+0100-02BA, U+02BD-02C5, U+02C7-02CC, U+02CE-02D7, U+02DD-02FF, U+0304, U+0308, U+0329, U+1D00-1DBF, U+1E00-1E9F, U+1EF2-1EFF, U+2020, U+20A0-20AB, U+20AD-20C4, U+2113, U+2C60-2C7F, U+A720-A7FF; }
@font-face { font-family: 'Google Sans'; src: url('../../assets/fonts/GoogleSans-latin.woff2') format('woff2'); font-weight: 500; font-display: swap; unicode-range: U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA, U+02DC, U+0304, U+0308, U+0329, U+2000-206F, U+20AC, U+2122, U+2191, U+2193, U+2212, U+2215, U+FEFF, U+FFFD; }
.social-buttons { display: grid; gap: var(--wx-space-3); }
.social-buttons :deep(.button) { gap: 10px; padding-inline: 12px; border-color: #747775; color: #1f1f1f; background: #fff; font: 500 14px/20px 'Google Sans', sans-serif; }
.social-buttons :deep(.button:hover:not(:disabled)) { background: #f8f8f8; }
.provider-icon { width: 20px; height: 20px; flex: 0 0 auto; background: #fff; }
</style>
