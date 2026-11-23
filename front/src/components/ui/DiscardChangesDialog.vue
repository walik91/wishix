<script setup>
import { useId } from 'vue'
import { t } from '../../i18n.js'
import UiButton from './UiButton.vue'
import UiDialog from './UiDialog.vue'

const open = defineModel('open', { type: Boolean })
defineProps({ description: { type: String, required: true } })
defineEmits(['discard'])
const descriptionId = useId()
</script>

<template>
  <UiDialog v-model:open="open" :title="t('discard.title')" :aria-describedby="descriptionId">
    <p :id="descriptionId">{{ description }}</p>
    <div class="discard-actions">
      <UiButton autofocus @click="open = false">{{ t('discard.continue') }}</UiButton>
      <UiButton variant="destructive" @click="$emit('discard')">{{ t('discard.leave') }}</UiButton>
    </div>
  </UiDialog>
</template>

<style scoped>
p { margin: 0; color: var(--wx-text-muted); line-height: 1.5; }
.discard-actions { display: flex; flex-wrap: wrap; gap: var(--wx-space-3); margin-top: var(--wx-space-6); }
.discard-actions > * { flex: 1 1 auto; }
</style>
