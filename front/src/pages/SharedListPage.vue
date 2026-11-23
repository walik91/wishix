<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watchEffect } from 'vue'
import { locale, t } from '../i18n.js'
import { viewGuestList } from '../api/lists.js'
import { exampleWishlist } from '../exampleWishlist.js'
import SharedWishCard from '../components/lists/SharedWishCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiButton from '../components/ui/UiButton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
const props = defineProps({ example: Boolean })
const sharedData = ref(null)
const data = computed(() => props.example ? exampleWishlist() : sharedData.value)
const list = computed(() => data.value?.list)
const wishes = computed(() => data.value?.wishes ?? [])
const listId = ref(''), token = ref(''), loading = ref(!props.example), error = ref('')
const resumeWishID = ref('')
function clearResume() { resumeWishID.value = '' }
let version = 0
const robots = props.example ? null : document.createElement('meta')
if (robots) { robots.name = 'robots'; document.head.append(robots) }
watchEffect(() => { document.title = `${list.value?.title || t('lists.sharedTitle')} · ${t('appName')}` })
async function load() {
  if (props.example) return
  const current = ++version
  const match = /^\/u\/[A-Za-z0-9_]{3,30}\/wishlist\/([a-fA-F0-9-]{36})(?:\/([A-Za-z0-9_-]{43}))?$/.exec(window.location.pathname.replace(/\/+$/, ''))
  listId.value = match?.[1] || ''; token.value = match?.[2] || ''
  sharedData.value = null; error.value = ''; loading.value = true
  // The server sends noindex for link-only/invalid URLs before rendering.
  robots.content = 'noindex, nofollow, noarchive'
  try {
    if (!match) { error.value = 'list_not_found'; return }
    const result = await viewGuestList(listId.value, token.value)
    if (current !== version) return
    sharedData.value = result
    robots.content = result.list.visibility === 'public' ? 'index, follow' : 'noindex, nofollow, noarchive'
  }
  catch (failure) { if (current === version) error.value = failure.status === 404 ? 'list_not_found' : 'unavailable' }
  finally { if (current === version) loading.value = false }
}
function date(value) { return new Intl.DateTimeFormat(locale.value, { dateStyle: 'long', timeZone: 'UTC' }).format(new Date(`${value}T00:00:00Z`)) }
onMounted(() => {
  if (props.example) return
  const resume = /^#reserve-([a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12})$/.exec(window.location.hash)
  if (resume) {
    resumeWishID.value = resume[1]
    window.history.replaceState(window.history.state, '', window.location.pathname + window.location.search)
  }
  load()
})
onBeforeUnmount(() => { version++; robots?.remove() })
</script>
<template>
  <div class="lists-page shared-list-page" :aria-busy="loading">
    <UiSkeleton v-if="loading" class="list-form-skeleton" />
    <UiCard v-else-if="error" class="list-empty">
      <EmptyState icon="lock" :title="t('lists.sharedUnavailable')" :description="t(`lists.error.${error}`)">
        <UiButton v-if="error !== 'list_not_found'" @click="load">{{ t('checkAgain') }}</UiButton>
      </EmptyState>
    </UiCard>
    <template v-else-if="list">
      <header class="list-page-heading">
        <div>
          <p v-if="!example" class="list-eyebrow">{{ t('lists.sharedBy') }} <a :href="`/u/${encodeURIComponent(data.username)}`">{{ data.username }}</a></p>
          <h1>{{ list.title }}</h1>
          <p v-if="list.event_date" class="list-muted"><time :datetime="list.event_date">{{ date(list.event_date) }}</time></p>
          <p v-if="list.description" class="list-description list-muted">{{ list.description }}</p>
          <p v-if="!list.expired" class="list-muted">{{ t('lists.wishesCount') }} {{ wishes.length }}</p>
        </div>
      </header>
      <UiCard v-if="list.expired" class="list-empty">
        <EmptyState icon="calendar" :title="t('lists.expired')" :description="t('lists.expiredDescription')" />
      </UiCard>
      <div v-else-if="wishes.length" class="shared-wish-grid">
        <SharedWishCard v-for="wish in wishes" :key="wish.id" :wish="wish" :list-id="listId" :token="token"
          :is-owner="data.is_owner === true" :example="example" :resume="resumeWishID === wish.id"
          @resumed="clearResume" @updated="load" />
      </div>
      <UiCard v-else class="list-empty">
        <EmptyState icon="heart" :title="t('lists.emptyListTitle')" :description="t('lists.sharedEmpty')" />
      </UiCard>
    </template>
  </div>
</template>
<style scoped>.shared-list-page { padding-block: var(--wx-space-8); }.list-eyebrow a { display: inline-flex; align-items: center; min-height: 44px; color: var(--wx-primary-pressed); }.shared-wish-grid { display: grid; grid-auto-rows: 1fr; gap: var(--wx-space-4); }@media(min-width:48rem) { .shared-wish-grid { grid-template-columns: repeat(2,minmax(0,1fr)); } }</style>
