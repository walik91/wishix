<script setup>
import { t } from '../../i18n.js'
import AppIcon from '../ui/AppIcon.vue'
import UiCard from '../ui/UiCard.vue'
defineProps({ icon: { type: String, default: 'list' }, warm: Boolean })
</script>

<template>
  <UiCard class="list-template">
    <div class="cover" :class="{ warm }" aria-hidden="true"><AppIcon :name="icon" /><span class="cover-line"></span></div>
    <div class="template-content"><p>{{ t('my.listTemplate') }}</p><span>{{ t('my.listTemplateDescription') }}</span></div>
  </UiCard>
</template>

<style scoped>
.list-template { display: flex; align-items: center; gap: var(--wx-space-4); padding: var(--wx-space-3); }
.cover { position: relative; display: flex; align-items: center; justify-content: center; flex: 0 0 64px; height: 64px; overflow: hidden; border-radius: var(--wx-radius-sm); color: var(--wx-primary); background: linear-gradient(130deg, var(--wx-background), var(--wx-lavender)); }
.cover.warm { color: var(--wx-text-muted); background: linear-gradient(130deg, var(--wx-background), var(--wx-cream)); }
.cover .icon { z-index: 1; width: 28px; height: 28px; }
.cover-line { position: absolute; width: 110px; height: 110px; top: 20px; left: 40%; border: 1px solid var(--wx-surface); border-radius: 50%; }
.template-content { min-width: 0; }
p { margin: 0 0 var(--wx-space-1); font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; }
.template-content > span { color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.5; }
@media (min-width: 48rem) {
  .list-template { display: block; padding: var(--wx-space-3); }
  .cover { width: 100%; height: 96px; margin-bottom: var(--wx-space-3); }
  .cover .icon { width: 32px; height: 32px; }
  .template-content { padding: 0 var(--wx-space-1) var(--wx-space-1); }
}
</style>
