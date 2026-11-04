<script setup>
import { ref, watch } from 'vue'
import { t } from '../../i18n.js'
import { uploadWishImage, deleteWishImage } from '../../api/wishes.js'
import UiDialog from '../ui/UiDialog.vue'
import UiButton from '../ui/UiButton.vue'
import WishImageInput from './WishImageInput.vue'

const props = defineProps({ wish: { type: Object, default: null } })
const open = defineModel('open', { type: Boolean, default: false })
const emit = defineEmits(['updated'])
const file = ref(null)
const invalid = ref('')
const error = ref('')
const busy = ref(false)
watch(open, () => { file.value = null; error.value = ''; invalid.value = '' })
async function save(remove = false) {
  if (busy.value || (!remove && (!file.value || invalid.value))) return
  busy.value = true
  error.value = ''
  try {
    const result = remove ? await deleteWishImage(props.wish.id) : await uploadWishImage(props.wish.id, file.value)
    emit('updated', props.wish.id, result.image_url)
    open.value = false
    file.value = null
  } catch (failure) {
    if (failure.status === 401) { window.location.replace('/login'); return }
    error.value = ['wish_image_invalid', 'wish_image_too_large', 'wish_image_dimensions', 'wish_image_busy'].includes(failure.code) ? failure.code : 'image_save_failed'
  } finally { busy.value = false }
}
</script>

<template>
  <UiDialog v-model:open="open" :busy="busy" :title="t('wishes.manageImage')">
    <template v-if="wish">
      <p class="wish-name">{{ wish.title }}</p>
      <WishImageInput id="edit-wish-image" v-model="file" :existing-src="wish.image_url || ''" :disabled="busy" @validation="invalid = $event; error = ''" />
      <p v-if="error" class="image-error" role="alert">{{ t(`wishes.error.${error}`) }}</p>
      <div class="actions"><UiButton :loading="busy" :disabled="!file || !!invalid" @click="save()">{{ t('wishes.saveImage') }}</UiButton><UiButton variant="secondary" :disabled="busy" @click="open = false">{{ t('wishes.cancel') }}</UiButton></div>
      <UiButton v-if="wish.image_url" class="remove-image" variant="secondary" :disabled="busy" @click="save(true)">{{ t('wishes.removeImage') }}</UiButton>
    </template>
  </UiDialog>
</template>

<style scoped>
.wish-name { margin: 0 0 var(--wx-space-5); font-weight: 600; line-height: 1.5; overflow-wrap: anywhere; }
.actions { display: flex; flex-direction: column; gap: var(--wx-space-3); margin-top: var(--wx-space-6); }
.image-error { margin: var(--wx-space-4) 0 0; color: var(--wx-error); font-size: var(--wx-text-sm); line-height: 1.6; }
.remove-image { width: 100%; margin-top: var(--wx-space-4); }
@media (min-width: 48rem) { .actions { flex-direction: row; } .actions > .button { flex: 1; } }
</style>
