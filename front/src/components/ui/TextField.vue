<script setup>
import { ref } from 'vue'

defineOptions({ inheritAttrs: false })
defineProps({
  id: { type: String, required: true },
  label: { type: String, required: true },
  error: { type: String, default: '' },
  hint: { type: String, default: '' },
  multiline: Boolean,
  visuallyHiddenLabel: Boolean,
})
const value = defineModel({ type: String, default: '' })
const input = ref(null)
defineExpose({ focus: () => input.value?.focus() })
</script>

<template>
  <div class="field">
    <label :for="id" :class="{ 'sr-only': visuallyHiddenLabel }">{{ label }}</label>
    <div class="control" :class="{ invalid: error }">
      <textarea v-if="multiline" :id="id" ref="input" v-model="value" v-bind="$attrs" :aria-invalid="error ? 'true' : undefined" :aria-describedby="[hint && `${id}-hint`, error && `${id}-error`].filter(Boolean).join(' ') || undefined"></textarea>
      <input v-else :id="id" ref="input" v-model="value" v-bind="$attrs" :aria-invalid="error ? 'true' : undefined" :aria-describedby="[hint && `${id}-hint`, error && `${id}-error`].filter(Boolean).join(' ') || undefined" />
      <slot />
    </div>
    <p v-if="hint" :id="`${id}-hint`" class="hint">{{ hint }}</p>
    <p v-if="error" :id="`${id}-error`" class="error">{{ error }}</p>
  </div>
</template>

<style scoped>
.field { display: grid; gap: var(--wx-space-2); min-width: 0; }
label { font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; }
.control { display: flex; min-height: 48px; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); background: var(--wx-surface); }
.control:hover { border-color: var(--wx-secondary); }
.control:focus-within { border-color: var(--wx-primary); box-shadow: var(--wx-focus-ring); }
input, textarea { min-width: 0; width: 100%; min-height: 46px; padding: var(--wx-space-3) var(--wx-space-4); border: 0; border-radius: inherit; color: var(--wx-text); background: transparent; font: inherit; font-size: var(--wx-text-md); line-height: 1.5; }
textarea { min-height: 128px; resize: vertical; }
input::placeholder, textarea::placeholder { color: var(--wx-text-muted); opacity: 1; }
input:focus-visible, textarea:focus-visible { outline: none; }
.invalid { border-color: var(--wx-error); }
.error { margin: 0; color: var(--wx-error); font-size: var(--wx-text-xs); line-height: 1.5; }
.hint { margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.5; }
</style>
