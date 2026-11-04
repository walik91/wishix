<script setup>
import AppIcon from './AppIcon.vue'
defineProps({ icon: { type: String, default: 'heart' }, title: { type: String, required: true }, description: { type: String, required: true } })
</script>

<template>
  <div class="empty-state">
    <span class="empty-icon" aria-hidden="true"><AppIcon :name="icon" /></span>
    <h3>{{ title }}</h3>
    <p>{{ description }}</p>
    <div v-if="$slots.default" class="empty-action"><slot /></div>
  </div>
</template>

<style scoped>
.empty-state { display: flex; flex-direction: column; align-items: center; text-align: center; }
.empty-icon { display: inline-flex; align-items: center; justify-content: center; width: 56px; height: 56px; margin-bottom: var(--wx-space-4); border-radius: var(--wx-radius-card); color: var(--wx-primary); background: var(--wx-lavender); }
.empty-icon .icon { width: 26px; height: 26px; }
h3 { margin: 0; font-size: var(--wx-text-lg); line-height: 1.4; }
p { max-width: 440px; margin: var(--wx-space-2) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.6; }
.empty-action { margin-top: var(--wx-space-5); }
</style>
