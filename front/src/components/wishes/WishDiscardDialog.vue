<script setup>
import { t } from '../../i18n.js'
import UiButton from '../ui/UiButton.vue'
import UiDialog from '../ui/UiDialog.vue'

const open = defineModel('open', { type: Boolean })
defineEmits(['discard'])
</script>

<template>
  <UiDialog v-model:open="open" :title="t('wishes.discard.title')" aria-describedby="wish-discard-description">
    <p id="wish-discard-description">{{ t('wishes.discard.description') }}</p>
    <div class="discard-actions">
      <UiButton @click="open = false">{{ t('wishes.discard.continue') }}</UiButton>
      <UiButton variant="destructive" @click="$emit('discard')">{{ t('wishes.discard.leave') }}</UiButton>
    </div>
  </UiDialog>
</template>

<style scoped>
p { margin: 0; color: var(--wx-text-muted); line-height: 1.5; }
.discard-actions { display: flex; flex-wrap: wrap; gap: var(--wx-space-3); margin-top: var(--wx-space-6); }
.discard-actions > * { flex: 1 1 auto; }
</style>
