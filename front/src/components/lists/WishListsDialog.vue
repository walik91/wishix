<script setup>
import { ref, watch } from 'vue'
import { t } from '../../i18n.js'
import { listLists, getWishLists, setWishLists } from '../../api/lists.js'
import { listFailure } from '../../composables/useLists.js'
import ListCheckboxes from './ListCheckboxes.vue'
import UiDialog from '../ui/UiDialog.vue'
import UiButton from '../ui/UiButton.vue'
const props = defineProps({ wish: { type: Object, default: null } })
const open = defineModel('open', { type: Boolean, default: false })
const emit = defineEmits(['updated'])
const lists = ref([]), selected = ref([]), version = ref(0), loading = ref(false), busy = ref(false), error = ref('')
let generation = 0
async function load() {
  const current = ++generation
  loading.value = true; error.value = ''
  try {
    const [all, membership] = await Promise.all([listLists(), getWishLists(props.wish.id)])
    if (current !== generation || !open.value) return
    lists.value = all.lists; selected.value = membership.list_ids; version.value = membership.version
  } catch (failure) { if (current === generation) error.value = listFailure(failure) }
  finally { if (current === generation) loading.value = false }
}
watch(open, value => { if (value && props.wish) load(); else generation++ })
async function save() {
  if (busy.value || loading.value) return
  busy.value = true; error.value = ''
  try { await setWishLists(props.wish.id, selected.value, version.value); open.value = false; emit('updated') }
  catch (failure) { error.value = listFailure(failure) }
  finally { busy.value = false }
}
</script>
<template>
  <UiDialog v-model:open="open" :busy="busy" :title="t('lists.wishLists')">
    <p class="list-selected-title">{{ wish?.title }}</p><p class="list-muted">{{ t('lists.membershipHint') }}</p>
    <p v-if="loading" role="status">{{ t('lists.loading') }}</p>
    <p v-else-if="error" class="list-error" role="alert">{{ t(`lists.error.${error}`) }} <UiButton variant="secondary" @click="load">{{ t('checkAgain') }}</UiButton></p>
    <template v-else><ListCheckboxes v-model="selected" :lists="lists" :disabled="busy" /><p v-if="!lists.length" class="list-muted">{{ t('lists.noLists') }}</p><UiButton v-if="!lists.length" variant="secondary" href="/my/lists/new">{{ t('lists.create') }}</UiButton></template>
    <div class="list-form-actions"><UiButton :loading="busy" :disabled="loading || !!error" @click="save">{{ t('lists.save') }}</UiButton><UiButton variant="secondary" :disabled="busy" @click="open = false">{{ t('wishes.cancel') }}</UiButton></div>
  </UiDialog>
</template>
