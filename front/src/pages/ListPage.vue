<script setup>
import { computed, nextTick, ref, watch, watchEffect } from 'vue'
import { locale, t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { listFailure, useList } from '../composables/useLists.js'
import { sharedListPath } from '../sharedListPath.js'
import { updateList, deleteList, clearList, copyList, rotateListLink, removeListWish } from '../api/lists.js'
import ListForm from '../components/lists/ListForm.vue'
import WishPickerDialog from '../components/lists/WishPickerDialog.vue'
import WishImageDialog from '../components/wishes/WishImageDialog.vue'
import WishCard from '../components/wishes/WishCard.vue'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiDialog from '../components/ui/UiDialog.vue'
import UiToast from '../components/ui/UiToast.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import TextField from '../components/ui/TextField.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import AppIcon from '../components/ui/AppIcon.vue'
const props = defineProps({ id: { type: String, required: true } })
const { user, checking, unavailable, refresh } = useSession()
const { list, wishes, loading, error, load } = useList(props.id)
const dialog = ref(''), busy = ref(false), operationError = ref(''), selected = ref(null), copyTitle = ref(''), pickerOpen = ref(false), imageOpen = ref(false), imageWish = ref(null), notice = ref(''), shareInput = ref(null), manageButton = ref(null)
const validNotices = ['created', 'updated', 'copied', 'cleared', 'removed', 'added', 'imageUpdated', 'linkUpdated', 'copiedLink']
const query = new URLSearchParams(window.location.search)
if (query.get('notice') === 'wish-added') notice.value = 'added'
else if (validNotices.includes(query.get('notice'))) notice.value = query.get('notice')
let initialAction = query.has('manage') ? 'manage' : query.has('edit') ? 'edit' : ''
if (query.size) window.history.replaceState(null, '', window.location.pathname)
const shareURL = computed(() => {
  const path = sharedListPath(user.value?.username, list.value?.share_path)
  return path ? new URL(path, window.location.origin).href : ''
})
const date = computed(() => list.value?.event_date ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'long', timeZone: 'UTC' }).format(new Date(`${list.value.event_date}T00:00:00Z`)) : '')
watchEffect(() => { document.title = `${list.value?.title || t('lists.details')} · ${t('appName')}` })
watch([user, checking, unavailable], async ([account, pending, failed]) => { if (account && !pending && !failed) { await load(); if (list.value && initialAction) { show(initialAction); initialAction = '' } } }, { immediate: true })
function show(kind, wish = null) { operationError.value = ''; selected.value = wish; copyTitle.value = [...`${list.value.title} (${t('lists.copySuffix')})`].slice(0, 120).join(''); dialog.value = kind }
async function restoreFocus() { await nextTick(); if (!dialog.value) manageButton.value?.$el.focus({ preventScroll: true }) }
watch(dialog, value => { if (!value) restoreFocus() })
async function retry() { await refresh(); if (!user.value && !unavailable.value) window.location.replace('/login') }
async function reload() { dialog.value = ''; operationError.value = ''; await load(); await restoreFocus() }
async function updated() { notice.value = 'added'; await load(); await restoreFocus() }
async function save(input) {
  if (busy.value) return; busy.value = true; operationError.value = ''
  try { await updateList(props.id, input); dialog.value = ''; notice.value = 'updated'; await load(); await restoreFocus() }
  catch (failure) { operationError.value = listFailure(failure) }
  finally { busy.value = false }
}
async function perform() {
  if (busy.value) return; busy.value = true; operationError.value = ''
  const action = dialog.value
  try {
    if (action === 'copy') { const { list: created } = await copyList(props.id, copyTitle.value); window.location.assign(`/my/lists/${created.id}?notice=copied`); return }
    if (action === 'delete') { await deleteList(props.id, list.value.revision); window.location.assign('/my/lists?notice=deleted'); return }
    if (action === 'clear') await clearList(props.id, list.value.revision)
    if (action === 'remove') await removeListWish(props.id, selected.value.id)
    if (action === 'rotate') await rotateListLink(props.id, list.value.revision)
    dialog.value = ''; notice.value = { clear: 'cleared', remove: 'removed', rotate: 'linkUpdated' }[action]
    await load(); if (action === 'rotate' && list.value) dialog.value = 'share'; await nextTick(); if (!dialog.value) manageButton.value?.$el.focus()
  } catch (failure) { operationError.value = listFailure(failure) }
  finally { busy.value = false }
}
async function copyLink() {
  try { await navigator.clipboard.writeText(shareURL.value); notice.value = 'copiedLink' }
  catch { shareInput.value?.focus(); shareInput.value?.select(); notice.value = 'selectLink' }
}
function imageUpdated(id, url) { const wish = wishes.value.find(wish => wish.id === id); if (wish) wish.image_url = url; notice.value = 'imageUpdated' }
</script>
<template>
  <div class="lists-page" :aria-busy="checking || loading">
    <UiButton href="/my/lists" variant="secondary" :disabled="busy">{{ t('lists.back') }}</UiButton>
    <UiToast v-if="notice" @dismiss="notice = ''">{{ t(`lists.notice.${notice}`) }}</UiToast>
    <UiSkeleton v-if="checking || (user && loading)" class="list-form-skeleton list-form-heading" />
    <UiCard v-else-if="unavailable || error" class="list-empty list-form-heading"><EmptyState :icon="error === 'list_not_found' ? 'lock' : 'globe'" :title="t(error === 'list_not_found' ? 'lists.notFound' : 'lists.loadError')" :description="t(`lists.error.${error || 'unavailable'}`)"><UiButton v-if="error !== 'list_not_found'" @click="unavailable ? retry() : load()">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <template v-else-if="user && list">
      <header class="list-page-heading list-form-heading"><div><p class="list-eyebrow">{{ t(`lists.visibility.${list.visibility}`) }}</p><h1>{{ list.title }}</h1><p v-if="date" class="list-muted"><time :datetime="list.event_date">{{ date }}</time></p><p v-if="list.description" class="list-muted list-description">{{ list.description }}</p><p class="list-muted">{{ t('lists.wishesCount') }} {{ wishes.length }}</p></div></header>
      <div class="list-toolbar"><UiButton :href="`/my/wishes/new?list=${id}`"><AppIcon name="plus" />{{ t('my.addWish') }}</UiButton><UiButton variant="secondary" @click="pickerOpen = true">{{ t('lists.fromCollection') }}</UiButton><UiButton variant="secondary" @click="show('share')"><AppIcon name="share" />{{ t('lists.share') }}</UiButton><UiButton ref="manageButton" variant="secondary" @click="show('manage')"><AppIcon name="more" />{{ t('lists.manage') }}</UiButton></div>
      <div v-if="wishes.length" class="wish-grid"><WishCard v-for="wish in wishes" :key="wish.id" :wish="wish" remove-from-list @delete="show('remove', $event)" @image="imageWish = $event; imageOpen = true" /></div>
      <UiCard v-else class="list-empty"><EmptyState icon="heart" :title="t('lists.emptyListTitle')" :description="t('lists.emptyListDescription')"><UiButton @click="pickerOpen = true">{{ t('lists.fromCollection') }}</UiButton></EmptyState></UiCard>
      <p class="list-muted">{{ t('lists.sharedWishHint') }}</p>
      <WishPickerDialog v-model:open="pickerOpen" :list-id="id" :existing="wishes.map(wish => wish.id)" @updated="updated" />
      <WishImageDialog v-model:open="imageOpen" :wish="imageWish" @updated="imageUpdated" />
      <UiDialog :open="dialog === 'manage'" :title="t('lists.manage')" @update:open="!$event && dialog === 'manage' && (dialog = '')"><div class="list-action-stack"><UiButton variant="secondary" @click="show('edit')">{{ t('lists.edit') }}</UiButton><UiButton variant="secondary" @click="show('copy')">{{ t('lists.copy') }}</UiButton><UiButton variant="secondary" :disabled="!wishes.length" @click="show('clear')">{{ t('lists.clear') }}</UiButton><UiButton variant="destructive" @click="show('delete')">{{ t('lists.delete') }}</UiButton></div></UiDialog>
      <UiDialog :open="dialog === 'edit'" :busy="busy" :title="t('lists.edit')" @update:open="!$event && dialog === 'edit' && (dialog = '')"><ListForm :initial="list" :busy="busy" :error="operationError" @submit="save" @cancel="dialog = ''" /><UiButton v-if="operationError === 'list_changed'" variant="secondary" @click="reload">{{ t('lists.reload') }}</UiButton></UiDialog>
      <UiDialog :open="dialog === 'share'" :title="t('lists.share')" @update:open="!$event && dialog === 'share' && (dialog = '')"><p class="list-muted">{{ t(`lists.visibilityHint.${list.visibility}`) }}</p><template v-if="shareURL"><label class="list-muted" for="list-share-url">{{ t('lists.shareLink') }}</label><div class="share-input-row"><input id="list-share-url" ref="shareInput" class="list-share-path" :value="shareURL" readonly /><UiButton variant="secondary" class="share-icon" :aria-label="t('lists.copyLink')" :title="t('lists.copyLink')" @click="copyLink"><AppIcon name="copy" /></UiButton><UiButton v-if="list.visibility === 'link'" variant="secondary" class="share-icon" :aria-label="t('lists.rotate')" :title="t('lists.rotate')" @click="show('rotate')"><AppIcon name="refresh" /></UiButton></div><div class="list-form-actions"><UiButton variant="secondary" :href="shareURL" target="_blank" rel="noopener noreferrer">{{ t('lists.preview') }}</UiButton></div></template><UiButton v-else variant="secondary" @click="show('edit')">{{ t('lists.edit') }}</UiButton></UiDialog>
      <UiDialog :open="['delete', 'clear', 'remove', 'rotate', 'copy'].includes(dialog)" :busy="busy" :title="t(`lists.${dialog || 'manage'}`)" @update:open="!$event && (dialog = '')"><p class="list-selected-title">{{ selected?.title || list.title }}</p><p class="list-muted">{{ t(`lists.confirm.${dialog}`) }}</p><TextField v-if="dialog === 'copy'" id="list-copy-title" v-model="copyTitle" :label="t('lists.titleLabel')" :disabled="busy" maxlength="240" /><p v-if="operationError" class="list-error" role="alert">{{ t(`lists.error.${operationError}`) }}</p><div class="list-form-actions"><UiButton :variant="['delete', 'clear'].includes(dialog) ? 'destructive' : 'primary'" :loading="busy" @click="perform">{{ t(`lists.${dialog || 'manage'}`) }}</UiButton><UiButton variant="secondary" :disabled="busy" @click="dialog = ''">{{ t('wishes.cancel') }}</UiButton></div><UiButton v-if="operationError === 'list_changed'" variant="secondary" @click="reload">{{ t('lists.reload') }}</UiButton></UiDialog>
    </template>
  </div>
</template>
<style scoped>
.wish-grid { display: grid; gap: var(--wx-space-4); align-items: start; }.share-input-row { display: flex; gap: var(--wx-space-2); align-items: center; }.share-input-row .list-share-path { flex: 1; width: 0; }.share-icon { flex: 0 0 48px; width: 48px; padding: 0; }@media(min-width:48rem) { .wish-grid { grid-template-columns: repeat(2,minmax(0,1fr)); } }
</style>
