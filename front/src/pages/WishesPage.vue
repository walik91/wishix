<script setup>
import { nextTick, ref, watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { useWishes } from '../composables/useWishes.js'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import UiDialog from '../components/ui/UiDialog.vue'
import UiToast from '../components/ui/UiToast.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import WishCard from '../components/wishes/WishCard.vue'
import WishImageDialog from '../components/wishes/WishImageDialog.vue'
import WishListsDialog from '../components/lists/WishListsDialog.vue'
import gift from '../assets/brand/gift-hero.svg'

const { user, checking, unavailable, refresh } = useSession()
const { wishes, loading, error, load, remove } = useWishes()
const deleteOpen = ref(false)
const selected = ref(null)
const deleting = ref(false)
const deleteError = ref(false)
const addButton = ref(null)
const imageOpen = ref(false)
const imageWish = ref(null)
const listsOpen = ref(false), listsWish = ref(null)
const noticeParam = new URLSearchParams(window.location.search).get('notice')
const notice = ref(noticeParam === 'wish-added' ? 'added' : noticeParam === 'wish-updated' ? 'updated' : '')
const noticeSequence = ref(0)
function showNotice(value) { noticeSequence.value++; notice.value = value }
if (notice.value) window.history.replaceState(null, '', `${window.location.pathname}${window.location.hash}`)

watchEffect(() => { document.title = `${t('my.allWishes')} · ${t('appName')}` })
watch([user, checking, unavailable], ([account, pending, failed]) => {
  if (account && !pending && !failed) load()
}, { immediate: true })

async function retry() {
  await refresh()
  if (!user.value && !unavailable.value) window.location.replace('/login')
}
function confirmDelete(wish) {
  selected.value = wish
  deleteError.value = false
  deleteOpen.value = true
}
function editImage(wish) { imageWish.value = wish; imageOpen.value = true }
function imageUpdated(id, url) {
  const wish = wishes.value.find(wish => wish.id === id)
  if (wish) wish.image_url = url
  showNotice('imageUpdated')
}
async function removeSelected() {
  if (deleting.value) return
  deleting.value = true
  deleteError.value = false
  try {
    await remove(selected.value.id)
    deleteOpen.value = false
    showNotice('deleted')
    await nextTick()
    // The deleted card's trigger no longer exists, so restore focus explicitly.
    addButton.value?.$el.focus()
  } catch { deleteError.value = true }
  finally { deleting.value = false }
}
</script>

<template>
  <div class="my-page" :aria-busy="checking || loading">
    <div v-if="checking" class="page-loading">
      <p class="sr-only" role="status">{{ t('auth.checkingSession') }}</p>
      <UiSkeleton class="loading-title" /><UiSkeleton class="loading-subtitle" />
      <div class="wish-grid"><UiSkeleton v-for="index in 4" :key="index" class="loading-card" /></div>
    </div>
    <UiCard v-else-if="unavailable" class="connection-error">
      <EmptyState icon="globe" :title="t('my.connectionTitle')" :description="t('auth.error.unavailable')"><UiButton @click="retry">{{ t('checkAgain') }}</UiButton></EmptyState>
    </UiCard>
    <template v-else-if="user">
      <UiToast v-if="notice" :key="noticeSequence" @dismiss="notice = ''">{{ t(`wishes.${notice}`) }}</UiToast>
      <header class="collection-heading">
        <div><p class="eyebrow">{{ t('auth.my') }}</p><h1>{{ t('my.allWishes') }}<span v-if="!loading && !error" class="wish-count">{{ wishes.length }}</span></h1><p>{{ t('my.wishesDescription') }}</p></div>
        <UiButton ref="addButton" href="/my/wishes/new"><AppIcon name="plus" />{{ t('my.addWish') }}</UiButton>
      </header>
      <div v-if="loading" class="wish-grid" role="status" :aria-label="t('wishes.loading')"><UiSkeleton v-for="index in 4" :key="index" class="loading-card" /></div>
      <UiCard v-else-if="error" class="connection-error">
        <EmptyState icon="globe" :title="t('wishes.loadErrorTitle')" :description="t('wishes.loadError')"><UiButton @click="load">{{ t('checkAgain') }}</UiButton></EmptyState>
      </UiCard>
      <UiCard v-else-if="!wishes.length" class="wishes-empty">
        <img :src="gift" alt="" width="220" height="180" aria-hidden="true" />
        <EmptyState :title="t('my.emptyWishesTitle')" :description="t('wishes.emptyDescription')"><UiButton href="/my/wishes/new"><AppIcon name="plus" />{{ t('wishes.firstWish') }}</UiButton></EmptyState>
      </UiCard>
      <div v-else class="wish-grid"><WishCard v-for="wish in wishes" :key="wish.id" :wish="wish" @delete="confirmDelete" @image="editImage" @copied="wishes.unshift($event); showNotice('copied')"><template #actions><UiButton variant="secondary" @click="listsWish = wish; listsOpen = true"><AppIcon name="list" />{{ t('lists.wishLists') }}</UiButton></template></WishCard></div>
      <p class="privacy-note"><AppIcon name="lock" /><span>{{ t('wishes.privateNote') }}</span></p>
    </template>
    <WishImageDialog v-model:open="imageOpen" :wish="imageWish" @updated="imageUpdated" />
    <WishListsDialog v-model:open="listsOpen" :wish="listsWish" @updated="showNotice('listsUpdated')" />
    <UiDialog v-model:open="deleteOpen" :busy="deleting" :title="t('wishes.deleteTitle')">
      <p class="delete-title">{{ selected?.title }}</p>
      <p class="delete-description">{{ t('wishes.deleteDescription') }}</p>
      <p v-if="deleteError" class="operation-error" role="alert">{{ t('wishes.deleteError') }}</p>
      <div class="dialog-actions"><UiButton variant="secondary" :disabled="deleting" @click="deleteOpen = false">{{ t('wishes.cancel') }}</UiButton><UiButton variant="destructive" :loading="deleting" @click="removeSelected">{{ t('wishes.delete') }}</UiButton></div>
    </UiDialog>
  </div>
</template>

<style scoped>
.my-page { min-width: 0; overflow-wrap: anywhere; }
.collection-heading { display: flex; flex-direction: column; align-items: flex-start; gap: var(--wx-space-5); margin-bottom: var(--wx-space-6); }
.collection-heading > div { min-width: 0; }
.eyebrow { margin: 0 0 var(--wx-space-2); color: var(--wx-primary-pressed); font-size: var(--wx-text-sm); font-weight: 600; }
h1 { display: flex; flex-wrap: wrap; align-items: center; gap: var(--wx-space-3); margin: 0; font-size: 26px; line-height: 1.3; font-weight: 750; letter-spacing: -.03em; }
.wish-count { display: inline-flex; align-items: center; justify-content: center; min-width: 32px; min-height: 32px; padding: var(--wx-space-1) var(--wx-space-2); border-radius: var(--wx-radius-pill); color: var(--wx-primary-pressed); background: var(--wx-lavender); font-size: var(--wx-text-sm); letter-spacing: 0; }
.collection-heading > div > p:last-child { margin: var(--wx-space-2) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.65; }
.collection-heading > .button { width: 100%; }
.wish-grid { display: grid; grid-auto-rows: 1fr; gap: var(--wx-space-4); }
.wishes-empty { display: flex; flex-direction: column; align-items: center; padding: var(--wx-space-8) var(--wx-space-5) var(--wx-space-10); }
.wishes-empty > img { display: block; width: 180px; max-width: 100%; height: auto; margin-bottom: var(--wx-space-4); }
.wishes-empty :deep(.empty-state > .empty-icon) { display: none; }
.privacy-note { display: flex; align-items: flex-start; justify-content: center; gap: var(--wx-space-2); margin: var(--wx-space-6) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.6; }
.privacy-note .icon { width: 16px; height: 18px; }
.page-loading { display: grid; gap: var(--wx-space-4); }
.loading-title { width: 60%; height: 32px; }.loading-subtitle { width: 80%; margin-bottom: var(--wx-space-5); }.loading-card { height: 180px; border-radius: var(--wx-radius-card); }
.connection-error { padding: var(--wx-space-10) var(--wx-space-5); }
.delete-title { margin: 0 0 var(--wx-space-3); font-weight: 650; line-height: 1.5; }
.delete-description { margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.65; }
.dialog-actions { display: flex; flex-direction: column-reverse; gap: var(--wx-space-3); margin-top: var(--wx-space-6); }
.operation-error { color: var(--wx-error); font-size: var(--wx-text-sm); line-height: 1.5; }
@media (min-width: 48rem) {
  .collection-heading { flex-direction: row; align-items: center; justify-content: space-between; gap: var(--wx-space-6); margin-bottom: var(--wx-space-8); }
  h1 { font-size: var(--wx-text-2xl); }.collection-heading > .button { flex-shrink: 0; width: auto; }
  .wish-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .wishes-empty { min-height: 400px; padding: var(--wx-space-10); }.wishes-empty > img { width: 220px; }
  .dialog-actions { flex-direction: row; justify-content: flex-end; }
}
</style>
