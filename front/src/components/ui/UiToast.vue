<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { t } from '../../i18n.js'
import AppIcon from './AppIcon.vue'

const props = defineProps({ duration: { type: Number, default: 6000 }, variant: { type: String, default: 'success' } })
const emit = defineEmits(['dismiss'])
const toast = ref(null)
const target = ref('body')
const inDialog = ref(false)
let observer, timer

function stopTimer() { clearTimeout(timer) }
function startTimer() {
  stopTimer()
  if (props.duration > 0) timer = setTimeout(() => emit('dismiss'), props.duration)
}
async function updateTarget() {
  const dialogs = [...document.querySelectorAll('dialog[open]')].filter(dialog => dialog.matches(':modal'))
  const active = dialogs.at(-1) || document.body
  inDialog.value = active !== document.body
  target.value = active
  await nextTick()
  // A popover enters the browser's top layer above a modal and is not clipped
  // by a scrollable dialog. Its parent keeps it inside the modal focus scope.
  if (toast.value?.showPopover && !toast.value.matches(':popover-open')) toast.value.showPopover()
}
onMounted(() => {
  updateTarget()
  observer = new MutationObserver(updateTarget)
  observer.observe(document.body, { subtree: true, attributes: true, attributeFilter: ['open'] })
  startTimer()
})
onBeforeUnmount(() => { observer?.disconnect(); stopTimer() })
</script>

<template>
  <Teleport :to="target">
    <div ref="toast" class="toast" :class="{ 'in-dialog': inDialog, 'is-info': variant === 'info' }" popover="manual" role="status" @mouseenter="stopTimer" @mouseleave="startTimer" @focusin="stopTimer" @focusout="startTimer">
      <AppIcon :name="variant === 'info' ? 'link' : 'check'" /><p><slot /></p>
      <button type="button" :aria-label="t('wishes.dismiss')" @click="emit('dismiss')"><AppIcon name="close" /></button>
    </div>
  </Teleport>
</template>

<style scoped>
.toast { position: fixed; z-index: 50; inset: auto auto calc(88px + env(safe-area-inset-bottom)) 50vw; transform: translateX(-50%); display: flex; align-items: center; gap: var(--wx-space-3); width: max-content; max-width: min(360px, calc(100vw - 32px)); margin: 0; padding: var(--wx-space-2) var(--wx-space-3) var(--wx-space-2) var(--wx-space-4); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); color: var(--wx-success); background: var(--wx-surface); box-shadow: var(--wx-shadow-panel); }
.toast.in-dialog { bottom: max(var(--wx-space-4), env(safe-area-inset-bottom)); }
.toast.is-info { color: var(--wx-text); }
p { flex: 1; min-width: 0; margin: 0; font-size: var(--wx-text-sm); line-height: 1.5; overflow-wrap: anywhere; }
button { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; height: 44px; border: 0; border-radius: var(--wx-radius-sm); color: var(--wx-text-muted); background: transparent; cursor: pointer; }
button:hover { background: var(--wx-background); }
@media (min-width: 64rem) { .toast, .toast.in-dialog { inset: auto max(var(--wx-space-6), env(safe-area-inset-right)) max(var(--wx-space-6), env(safe-area-inset-bottom)) auto; transform: none; } }
</style>
