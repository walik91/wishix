<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { t } from '../../i18n.js'
import UiButton from '../ui/UiButton.vue'
import AppIcon from '../ui/AppIcon.vue'

const props = defineProps({ id: { type: String, required: true }, existingSrc: { type: String, default: '' }, error: { type: String, default: '' }, disabled: Boolean })
const file = defineModel({ default: null })
const emit = defineEmits(['validation', 'manual-change'])
const picker = ref(null)
const choose = ref(null)
const preview = ref('')
const localError = ref('')
const errorCode = computed(() => localError.value || props.error)
const previewURL = computed(() => preview.value || props.existingSrc)

watch(file, value => {
  if (preview.value) URL.revokeObjectURL(preview.value)
  preview.value = value ? URL.createObjectURL(value) : ''
}, { immediate: true })
onBeforeUnmount(() => { if (preview.value) URL.revokeObjectURL(preview.value) })
function setError(code) { localError.value = code; emit('validation', code) }
function select(event) {
  const selected = event.target.files?.[0]
  if (!selected) return
  emit('manual-change')
  event.target.value = ''
  setError('')
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(selected.type)) { setError('wish_image_invalid'); return }
  if (selected.size > 5 * 1024 * 1024) { setError('wish_image_too_large'); return }
  file.value = selected
}
async function clear() {
  emit('manual-change')
  file.value = null
  setError('')
  await nextTick()
  choose.value?.$el.focus()
}
</script>

<template>
  <section class="image-field" :aria-labelledby="`${id}-label`">
    <h2 :id="`${id}-label`">{{ t('wishes.imageLabel') }}</h2>
    <div class="image-picker">
      <div class="image-preview">
        <img v-if="previewURL" :src="previewURL" :alt="t('wishes.imagePreview')" @error="file && setError('wish_image_invalid')" />
        <AppIcon v-else name="image" />
      </div>
      <div class="image-actions">
        <input :id="id" ref="picker" class="sr-only" type="file" accept="image/jpeg,image/png,image/webp" tabindex="-1" :disabled="disabled" :aria-label="t('wishes.chooseImage')" @change="select" />
        <UiButton :id="`${id}-choose`" ref="choose" variant="secondary" :disabled="disabled" :aria-invalid="errorCode ? 'true' : undefined" :aria-describedby="[`${id}-hint`, errorCode && `${id}-error`].filter(Boolean).join(' ')" @click="picker.click()"><AppIcon name="image" />{{ t(previewURL ? 'wishes.replaceImage' : 'wishes.chooseImage') }}</UiButton>
        <UiButton v-if="file || localError" variant="secondary" :disabled="disabled" @click="clear">{{ t('wishes.clearImage') }}</UiButton>
        <p :id="`${id}-hint`" class="image-hint">{{ t('wishes.imageHint') }}</p>
        <p v-if="file" class="file-name">{{ file.name }}</p>
      </div>
    </div>
    <p v-if="errorCode" :id="`${id}-error`" class="image-error" role="alert">{{ t(`wishes.error.${errorCode}`) }}</p>
  </section>
</template>

<style scoped>
.image-field { min-width: 0; }
h2 { margin: 0; font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; }
.image-hint { flex-basis: 100%; margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.6; }
.image-picker { display: flex; align-items: center; gap: var(--wx-space-3); margin-top: var(--wx-space-2); min-width: 0; }
.image-preview { display: flex; align-items: center; justify-content: center; flex-shrink: 0; width: 80px; height: 80px; overflow: hidden; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); color: var(--wx-primary); background: var(--wx-background); }
.image-preview img { width: 100%; height: 100%; object-fit: contain; background: var(--wx-surface); }
.image-preview .icon { width: 24px; height: 24px; }
.image-actions { display: flex; flex-wrap: wrap; gap: var(--wx-space-2); min-width: 0; max-width: 100%; }
.file-name { flex-basis: 100%; min-width: 0; margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.6; overflow-wrap: anywhere; }
.image-error { margin: var(--wx-space-3) 0 0; color: var(--wx-error); font-size: var(--wx-text-sm); line-height: 1.6; }

</style>
