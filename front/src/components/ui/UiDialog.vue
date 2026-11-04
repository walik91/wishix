<script setup>
import { onMounted, ref, watch } from 'vue'
import { t } from '../../i18n.js'
import AppIcon from './AppIcon.vue'
const props = defineProps({ open: Boolean, title: { type: String, required: true } })
const emit = defineEmits(['update:open'])
const dialog = ref(null)
function syncOpen() {
  if (props.open && !dialog.value.open) dialog.value.showModal()
  else if (!props.open && dialog.value.open) dialog.value.close()
}
function close() { dialog.value.close() }
function trapFocus(event) {
  if (event.key !== 'Tab') return
  const targets = [...dialog.value.querySelectorAll('a[href], button, input, select, textarea, [tabindex]')]
    .filter(element => !element.disabled && element.tabIndex >= 0 && element.getClientRects().length)
  const first = targets[0]
  const last = targets.at(-1)
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}
function backdrop(event) {
  if (event.target !== dialog.value) return
  const rect = dialog.value.getBoundingClientRect()
  if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) close()
}
onMounted(syncOpen)
watch(() => props.open, syncOpen)
</script>

<template>
  <dialog ref="dialog" class="dialog" :aria-label="title" @close="emit('update:open', false)" @click="backdrop" @keydown="trapFocus">
    <header><h2>{{ title }}</h2><button type="button" class="close" autofocus :aria-label="t('my.close')" @click="close"><AppIcon name="close" /></button></header>
    <slot />
  </dialog>
</template>

<style scoped>
.dialog { position: fixed; inset: auto 0 0; width: 100%; max-width: 100%; max-height: calc(100svh - 24px); margin: 0; padding: var(--wx-space-6) var(--wx-space-5) max(var(--wx-space-6), env(safe-area-inset-bottom)); overflow: auto; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-hero) var(--wx-radius-hero) 0 0; color: var(--wx-text); background: var(--wx-surface); box-shadow: var(--wx-shadow-panel); overflow-wrap: anywhere; }
.dialog::backdrop { background: color-mix(in srgb, var(--wx-text) 35%, transparent); }
header { display: flex; align-items: center; justify-content: space-between; gap: var(--wx-space-4); margin-bottom: var(--wx-space-6); }
h2 { margin: 0; font-size: var(--wx-text-xl); line-height: 1.3; }
.close { display: inline-flex; flex-shrink: 0; align-items: center; justify-content: center; width: 44px; height: 44px; border: 0; border-radius: var(--wx-radius-md); color: var(--wx-text-muted); background: var(--wx-background); cursor: pointer; }
.close:hover { background: var(--wx-lavender); }
.close:active { color: var(--wx-primary-pressed); }
@media (min-width: 48rem) { .dialog { inset: 0; width: min(480px, 100% - 48px); max-height: calc(100svh - 48px); margin: auto; padding: var(--wx-space-8); border-radius: var(--wx-radius-hero); } }
</style>
