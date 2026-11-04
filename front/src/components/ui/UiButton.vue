<script setup>
defineProps({
  type: { type: String, default: 'button' },
  variant: { type: String, default: 'primary' },
  disabled: Boolean,
  loading: Boolean,
  href: { type: String, default: '' },
})
</script>

<template>
  <component :is="href ? 'a' : 'button'" :href="href && !disabled && !loading ? href : undefined" :type="href ? undefined : type" class="button" :class="variant" :disabled="!href && (disabled || loading) ? true : undefined" :aria-disabled="href && (disabled || loading) ? true : undefined" :aria-busy="loading || undefined" @click="(disabled || loading) && $event.preventDefault()">
    <span v-if="loading" class="spinner" aria-hidden="true"></span>
    <slot />
  </component>
</template>

<style scoped>
.button {
  display: inline-flex;
  justify-content: center;
  align-items: center;
  gap: var(--wx-space-3);
  min-height: 48px;
  padding: var(--wx-space-3) var(--wx-space-4);
  border: 1px solid transparent;
  border-radius: var(--wx-radius-md);
  font-size: var(--wx-text-sm);
  font-weight: 600;
  line-height: 1.5;
  cursor: pointer;
  text-decoration: none;
  text-align: center;
  transition: background-color 150ms, border-color 150ms;
}
.primary { color: var(--wx-surface); background: var(--wx-primary); }
.primary:hover:not(:disabled, [aria-disabled='true']) { background: var(--wx-primary-hover); }
.primary:active:not(:disabled, [aria-disabled='true']) { background: var(--wx-primary-pressed); }
.secondary { color: var(--wx-text); background: var(--wx-surface); border-color: var(--wx-border); }
.secondary:hover:not(:disabled, [aria-disabled='true']) { background: var(--wx-background); border-color: var(--wx-secondary); }
.secondary:active:not(:disabled, [aria-disabled='true']) { background: var(--wx-lavender); }
.destructive { color: var(--wx-surface); background: var(--wx-error); }
.destructive:hover:not(:disabled, [aria-disabled='true']) { background: color-mix(in srgb, var(--wx-error) 88%, var(--wx-text)); }
.destructive:active:not(:disabled, [aria-disabled='true']) { background: color-mix(in srgb, var(--wx-error) 75%, var(--wx-text)); }
.button:disabled, .button[aria-disabled='true'] { opacity: .55; cursor: not-allowed; }
.button[aria-busy='true'] { cursor: wait; }
.spinner { width: 18px; height: 18px; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; animation: spin 700ms linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
