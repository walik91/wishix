<script setup>
import { onMounted, ref, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { publicCollection } from '../api/lists.js'
import SharedWishCard from '../components/lists/SharedWishCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiButton from '../components/ui/UiButton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
const props = defineProps({ username: { type: String, required: true } })
const username = ref(''), wishes = ref([]), loading = ref(true), error = ref('')
watchEffect(() => { document.title = `${t('profile.allWishes')} · ${t('appName')}` })
async function load() {
  loading.value = true; error.value = ''
  try { const data = await publicCollection(props.username); username.value = data.username; wishes.value = data.wishes }
  catch (failure) { error.value = failure.status === 404 ? 'list_not_found' : 'unavailable' }
  finally { loading.value = false }
}
onMounted(load)
</script>
<template>
  <div class="lists-page public-collection" :aria-busy="loading">
    <UiSkeleton v-if="loading" class="list-form-skeleton" />
    <UiCard v-else-if="error" class="list-empty"><EmptyState icon="lock" :title="t('lists.sharedUnavailable')" :description="t(`lists.error.${error}`)"><UiButton v-if="error !== 'list_not_found'" @click="load">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <template v-else>
      <header class="list-page-heading"><div><p class="list-eyebrow"><a :href="`/u/${encodeURIComponent(username)}`">{{ username }}</a></p><h1>{{ t('profile.allWishes') }}</h1><p class="list-muted">{{ t('lists.wishesCount') }} {{ wishes.length }}</p></div></header>
      <div v-if="wishes.length" class="shared-wish-grid"><SharedWishCard v-for="wish in wishes" :key="wish.id" :wish="wish" :username="username" @updated="load" /></div>
      <UiCard v-else class="list-empty"><EmptyState icon="heart" :title="t('lists.emptyListTitle')" :description="t('lists.sharedEmpty')" /></UiCard>
    </template>
  </div>
</template>
<style scoped>
.public-collection { padding-block: var(--wx-space-8); }.list-eyebrow a { display: inline-flex; align-items: center; min-height: 44px; color: var(--wx-primary-pressed); }.shared-wish-grid { display: grid; grid-auto-rows: 1fr; gap: var(--wx-space-4); }@media(min-width:48rem) { .shared-wish-grid { grid-template-columns: repeat(2,minmax(0,1fr)); } }
</style>
