<script setup>
import { ref, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { trackEvent } from '../analytics.js'
import { searchPeople } from '../api/search.js'
import { publicAvatarURL } from '../api/social.js'
import AppIcon from '../components/ui/AppIcon.vue'
import UserAvatar from '../components/ui/UserAvatar.vue'
import TextField from '../components/ui/TextField.vue'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import EmptyState from '../components/ui/EmptyState.vue'

const query = ref(''), submitted = ref(''), people = ref([]), cursor = ref(''), loading = ref(false), searched = ref(false), error = ref('')
watchEffect(() => { document.title = `${t('my.search')} · ${t('appName')}` })
async function find(more = false) {
  if (loading.value) return
  const text = more ? submitted.value : query.value.trim()
  if (!text) return
  if (!more) { submitted.value = text; people.value = []; cursor.value = ''; searched.value = false }
  loading.value = true; error.value = ''
  try {
    const result = await searchPeople(text, more ? cursor.value : '')
    if (!more) trackEvent('people_search_completed')
    people.value = more ? [...people.value, ...result.people] : result.people
    cursor.value = result.next_cursor
    searched.value = true
  } catch (failure) {
    if (failure.status === 401) window.location.replace('/login')
    else error.value = failure.code === 'invalid_input' ? t('search.invalid') : t('search.error')
  } finally { loading.value = false }
}
</script>

<template>
  <div class="search-page" :aria-busy="loading">
    <header><h1>{{ t('search.title') }}</h1><p>{{ t('search.description') }}</p></header>
    <form class="search-form" @submit.prevent="find(false)">
      <TextField id="people-search" v-model="query" type="search" :label="t('search.label')" :placeholder="t('search.placeholder')" maxlength="200" />
      <UiButton type="submit" :loading="loading" :disabled="!query.trim()"><AppIcon name="search" />{{ t('my.search') }}</UiButton>
    </form>
    <p v-if="error" class="search-error" role="alert">{{ error }}</p>
    <ul v-if="people.length" class="search-results" :aria-label="t('search.results')">
      <li v-for="person in people" :key="person.username"><a class="person-card" :href="`/u/${encodeURIComponent(person.username)}`">
        <UserAvatar :username="person.display_name || person.username" :src="person.has_avatar ? publicAvatarURL(person.username) : ''" />
        <span class="person-details"><strong>{{ person.display_name || person.username }}</strong><span v-if="person.display_name">@{{ person.username }}</span></span>
        <AppIcon name="arrow" />
      </a></li>
    </ul>
    <UiCard v-else-if="searched && !loading && !error" class="search-empty"><EmptyState icon="search" :title="t('search.empty')" :description="t('search.emptyHint')" /></UiCard>
    <p v-else-if="loading && !people.length" class="search-status" role="status">{{ t('search.loading') }}</p>
    <UiButton v-if="cursor" class="more-results" variant="secondary" :loading="loading" @click="find(true)">{{ t('search.more') }}</UiButton>
  </div>
</template>

<style scoped>
.search-page { max-width: 760px; min-width: 0; }.search-page h1 { margin: 0; font-size: var(--wx-text-2xl); line-height: 1.2; }.search-page header p, .search-status { color: var(--wx-text-muted); line-height: 1.6; }.search-form { display: grid; gap: var(--wx-space-3); align-items: end; margin-block: var(--wx-space-6); }.search-error { color: var(--wx-error); }.search-results { display: grid; gap: var(--wx-space-3); margin: 0; padding: 0; list-style: none; }.person-card { display: flex; align-items: center; gap: var(--wx-space-3); min-height: 72px; min-width: 0; padding: var(--wx-space-4); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); color: var(--wx-text); background: var(--wx-surface); text-decoration: none; overflow-wrap: anywhere; }.person-card:hover { border-color: var(--wx-secondary); background: var(--wx-background); }.person-card:active { background: var(--wx-lavender); }.person-card > .icon { color: var(--wx-primary); }.person-details { display: grid; flex: 1; gap: var(--wx-space-1); min-width: 0; }.person-details span { color: var(--wx-text-muted); font-size: var(--wx-text-sm); }.search-empty { padding: var(--wx-space-6); }.more-results { margin-top: var(--wx-space-4); }@media(min-width: 48rem) { .search-form { grid-template-columns: minmax(0, 1fr) auto; } }
</style>
