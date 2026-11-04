<script setup>
import { onBeforeUnmount, onMounted, ref, watchEffect } from 'vue'
import { locale, t } from '../i18n.js'
import { viewSharedList } from '../api/lists.js'
import SharedWishCard from '../components/lists/SharedWishCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiButton from '../components/ui/UiButton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
const list = ref(null), wishes = ref([]), username = ref(''), token = ref(''), loading = ref(true), error = ref('')
let version = 0
const robots = document.createElement('meta'); robots.name = 'robots'; document.head.append(robots)
watchEffect(() => { document.title = `${list.value?.title || t('lists.sharedTitle')} · ${t('appName')}` })
async function load() {
  const current = ++version
  const queryToken = new URLSearchParams(window.location.search).get('token')
  token.value = queryToken ?? ''
  list.value = null; wishes.value = []; username.value = ''; error.value = ''; loading.value = true
  // The server sends noindex for link-only/invalid URLs before rendering.
  robots.content = queryToken ? 'index, follow' : 'noindex, nofollow, noarchive'
  try {
    if (!/^[A-Za-z0-9_-]{43}$/.test(token.value)) { error.value = 'list_not_found'; return }
    const data = await viewSharedList(token.value)
    if (current !== version) return
    list.value = data.list; wishes.value = data.wishes; username.value = data.username
    robots.content = data.list.visibility === 'public' ? 'index, follow' : 'noindex, nofollow, noarchive'
  }
  catch (failure) { if (current === version) error.value = failure.status === 404 ? 'list_not_found' : 'unavailable' }
  finally { if (current === version) loading.value = false }
}
function date(value) { return new Intl.DateTimeFormat(locale.value, { dateStyle: 'long', timeZone: 'UTC' }).format(new Date(`${value}T00:00:00Z`)) }
onMounted(load)
onBeforeUnmount(() => { version++; robots.remove() })
</script>
<template>
  <div class="lists-page shared-list-page" :aria-busy="loading"><UiSkeleton v-if="loading" class="list-form-skeleton" />
    <UiCard v-else-if="error" class="list-empty"><EmptyState icon="lock" :title="t('lists.sharedUnavailable')" :description="t(`lists.error.${error}`)"><UiButton v-if="error !== 'list_not_found'" @click="load">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <template v-else-if="list"><header class="list-page-heading"><div><p class="list-eyebrow">{{ t('lists.sharedBy') }} <a :href="`/u/${encodeURIComponent(username)}`">{{ username }}</a></p><h1>{{ list.title }}</h1><p v-if="list.event_date" class="list-muted"><time :datetime="list.event_date">{{ date(list.event_date) }}</time></p><p v-if="list.description" class="list-description list-muted">{{ list.description }}</p><p class="list-muted">{{ t('lists.wishesCount') }} {{ wishes.length }}</p></div></header><div v-if="wishes.length" class="shared-wish-grid"><SharedWishCard v-for="wish in wishes" :key="wish.id" :wish="wish" :token="token" /></div><UiCard v-else class="list-empty"><EmptyState icon="heart" :title="t('lists.emptyListTitle')" :description="t('lists.sharedEmpty')" /></UiCard></template>
  </div>
</template>
<style scoped>.shared-list-page { padding-block: var(--wx-space-8); }.list-eyebrow a { display: inline-flex; align-items: center; min-height: 44px; color: var(--wx-primary-pressed); }.shared-wish-grid { display: grid; gap: var(--wx-space-4); align-items: start; }@media(min-width:48rem) { .shared-wish-grid { grid-template-columns: repeat(2,minmax(0,1fr)); } }</style>
