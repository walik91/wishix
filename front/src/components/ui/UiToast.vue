<script setup>
import { t } from '../../i18n.js'
import AppIcon from './AppIcon.vue'
defineEmits(['dismiss'])
</script>

<template>
  <div class="toast" role="status">
    <AppIcon name="check" /><p><slot /></p>
    <button type="button" :aria-label="t('wishes.dismiss')" @click="$emit('dismiss')"><AppIcon name="close" /></button>
  </div>
</template>

<style scoped>
.toast { display: flex; align-items: center; gap: var(--wx-space-3); margin-bottom: var(--wx-space-6); padding: var(--wx-space-2) var(--wx-space-3) var(--wx-space-2) var(--wx-space-4); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); color: var(--wx-success); background: var(--wx-surface); }
p { flex: 1; min-width: 0; margin: 0; font-size: var(--wx-text-sm); line-height: 1.5; overflow-wrap: anywhere; }
button { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; height: 44px; border: 0; border-radius: var(--wx-radius-sm); color: var(--wx-text-muted); background: transparent; cursor: pointer; }
button:hover { background: var(--wx-background); }
</style>
