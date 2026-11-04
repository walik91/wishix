<script setup>
import { computed, ref, watch } from 'vue'
import { t } from '../../i18n.js'
import { listWishes } from '../../api/wishes.js'
import { addListWishes } from '../../api/lists.js'
import { listFailure } from '../../composables/useLists.js'
import UiDialog from '../ui/UiDialog.vue'
import UiButton from '../ui/UiButton.vue'
import TextField from '../ui/TextField.vue'
const props = defineProps({ listId: { type: String, required: true }, existing: { type: Array, default: () => [] } })
const open = defineModel('open', { type: Boolean, default: false }), emit = defineEmits(['updated'])
const wishes = ref([]), selected = ref([]), query = ref(''), loading = ref(false), busy = ref(false), error = ref('')
let generation = 0
const available = computed(() => wishes.value.filter(wish => !props.existing.includes(wish.id)))
const filtered = computed(() => available.value.filter(wish => wish.title.toLocaleLowerCase().includes(query.value.trim().toLocaleLowerCase())))
async function load() {
  const current = ++generation; loading.value = true; error.value = ''
  try { const data = await listWishes(); if (current === generation && open.value) wishes.value = data.wishes }
  catch (failure) { if (current === generation) error.value = listFailure(failure) }
  finally { if (current === generation) loading.value = false }
}
watch(open, value => { if (value) { selected.value = []; query.value = ''; load() } else generation++ })
async function save() {
  if (busy.value || !selected.value.length) return
  busy.value = true; error.value = ''
  try { await addListWishes(props.listId, selected.value); open.value = false; emit('updated') }
  catch (failure) { error.value = listFailure(failure) }
  finally { busy.value = false }
}
</script>
<template>
  <UiDialog v-model:open="open" :busy="busy" :title="t('lists.fromCollection')">
    <p class="list-muted">{{ t('lists.pickerHint') }}</p>
    <p v-if="loading" role="status">{{ t('wishes.loading') }}</p>
    <p v-else-if="error" class="list-error" role="alert">{{ t(`lists.error.${error}`) }} <UiButton variant="secondary" @click="load">{{ t('checkAgain') }}</UiButton></p>
    <template v-else><TextField v-if="available.length" id="list-wish-search" v-model="query" :label="t('lists.searchWishes')" type="search" :disabled="busy" /><div class="list-checkboxes"><label v-for="wish in filtered" :key="wish.id" class="list-check-row"><input v-model="selected" type="checkbox" :value="wish.id" :disabled="busy" /><span><strong>{{ wish.title }}</strong></span></label></div><p v-if="!filtered.length" class="list-muted">{{ t(available.length ? 'lists.noSearchResults' : 'lists.noAvailableWishes') }}</p><UiButton v-if="!available.length" variant="secondary" :href="`/my/wishes/new?list=${listId}`">{{ t('my.addWish') }}</UiButton></template>
    <div class="list-form-actions"><UiButton :loading="busy" :disabled="loading || !!error || !selected.length" @click="save">{{ t('lists.addSelected') }} <span v-if="selected.length">({{ selected.length }})</span></UiButton><UiButton variant="secondary" :disabled="busy" @click="open = false">{{ t('wishes.cancel') }}</UiButton></div>
  </UiDialog>
</template>
