<script setup>
import { ref, watch } from 'vue'
import { t } from '../../i18n.js'
import TextField from '../ui/TextField.vue'
import UiButton from '../ui/UiButton.vue'
const props = defineProps({ initial: { type: Object, default: null }, busy: Boolean, error: { type: String, default: '' } })
const emit = defineEmits(['submit', 'cancel'])
const title = ref(''), description = ref(''), date = ref(''), visibility = ref('link'), invalid = ref({}), form = ref(null)
watch(() => props.initial, value => { title.value = value?.title || ''; description.value = value?.description || ''; date.value = value?.event_date || ''; visibility.value = value?.visibility || 'link'; invalid.value = {} }, { immediate: true })
function fieldError(field) { const code = invalid.value[field] || (props.error === `list_${field}_invalid` ? props.error : ''); return code ? t(`lists.error.${code}`) : '' }
function submit() {
  invalid.value = {}
  if (!title.value.trim() || [...title.value.trim()].length > 120) invalid.value.title = 'list_title_invalid'
  if ([...description.value.trim()].length > 2000) invalid.value.description = 'list_description_invalid'
  if (Object.keys(invalid.value).length) { setTimeout(() => form.value?.querySelector('[aria-invalid=true]')?.focus()); return }
  const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  const timezone = props.initial && date.value === (props.initial.event_date || '') ? (props.initial.event_timezone || browserTimezone) : browserTimezone
  emit('submit', { title: title.value.trim(), description: description.value.trim(), event_date: date.value || null, event_timezone: timezone, visibility: visibility.value, revision: props.initial?.revision || 0 })
}
</script>
<template>
  <form ref="form" class="list-form" novalidate :aria-busy="busy" @submit.prevent="submit">
    <fieldset :disabled="busy">
      <legend class="sr-only">{{ t('lists.details') }}</legend>
      <TextField id="list-title" v-model="title" :label="t('lists.titleLabel')" :hint="t('lists.titleHint')" :error="fieldError('title')" required maxlength="240" autocomplete="off" />
      <TextField id="list-description" v-model="description" :label="t('lists.descriptionLabel')" :hint="t('lists.descriptionHint')" :error="fieldError('description')" multiline rows="3" maxlength="4000" />
      <TextField id="list-date" v-model="date" :label="t('lists.dateLabel')" :hint="t('lists.dateHint')" :error="fieldError('date')" type="date" min="0001-01-01" max="9999-12-31" />
      <div class="list-select"><label for="list-visibility">{{ t('lists.visibilityLabel') }}</label><select id="list-visibility" v-model="visibility" aria-describedby="list-visibility-hint"><option v-for="value in ['private', 'link', 'public']" :key="value" :value="value">{{ t(`lists.visibility.${value}`) }}</option></select><p id="list-visibility-hint" class="list-muted">{{ t(`lists.visibilityHint.${visibility}`) }}</p></div>
    </fieldset>
    <p v-if="error" class="list-error" role="alert">{{ t(`lists.error.${error}`) }}</p>
    <div class="list-form-actions"><UiButton type="submit" :loading="busy">{{ t(initial ? 'lists.save' : 'lists.create') }}</UiButton><UiButton variant="secondary" :disabled="busy" @click="emit('cancel')">{{ t('wishes.cancel') }}</UiButton></div>
  </form>
</template>
