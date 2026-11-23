<script setup>
import { computed, onMounted, ref, watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { socialPeople, publicAvatarURL, publicDetails } from '../api/social.js'
import PublicProfileNavigation from '../components/navigation/PublicProfileNavigation.vue'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import UserAvatar from '../components/ui/UserAvatar.vue'
import AppIcon from '../components/ui/AppIcon.vue'
const props = defineProps({ own: Boolean, username: { type: String, default: '' }, initialKind: { type: String, default: 'following' } })
const { user, checking, unavailable } = useSession()
const name = computed(() => props.own ? user.value?.username || '' : props.username)
const kind = ref(props.initialKind), loading = ref(true), people = ref([]), cursor = ref(''), error = ref('')
const details = ref(null)
const profileUsername = computed(() => details.value?.username || props.username)
if (!props.own) {
  onMounted(async () => {
    try { details.value = await publicDetails(props.username) } catch { details.value = null }
  })
}
watchEffect(() => { document.title = `${t(`social.${kind.value}`)} · ${t('appName')}` })
async function load(more = false) {
  if (!name.value) return
  loading.value = true; error.value = ''
  try { const data = await socialPeople(name.value, kind.value, more ? cursor.value : ''); people.value = more ? [...people.value, ...data.people] : data.people; cursor.value = data.next_cursor }
  catch (failure) { error.value = failure.status === 404 ? 'unavailableList' : 'loadError' }
  finally { loading.value = false }
}
watch([name, kind], ([current]) => { if (current) { people.value = []; cursor.value = ''; load() } }, { immediate: true })
</script>
<template>
  <div class="people-page" :class="{ 'public-people': !own }" :aria-busy="loading || checking">
    <template v-if="!own">
      <UiButton class="people-back" variant="ghost" :href="`/u/${encodeURIComponent(profileUsername)}`">
        <AppIcon name="arrow" />
        <span>{{ t('social.backToProfile').replace('{username}', `@${profileUsername}`) }}</span>
      </UiButton>
      <PublicProfileNavigation
        :username="profileUsername"
        :current="kind"
        :show-followers="details?.show_followers"
        :show-following="details?.show_following"
      />
    </template>
    <h1>{{ own ? t('social.title') : t(`social.${kind}`) }}</h1>
    <div v-if="own" class="people-tabs" role="group" :aria-label="t('social.title')"><UiButton v-for="tab in ['following', 'followers', 'blocked']" :key="tab" :variant="kind === tab ? 'primary' : 'secondary'" :aria-pressed="kind === tab" @click="kind = tab">{{ t(`social.${tab}`) }}</UiButton></div>
    <UiSkeleton v-if="(loading && !people.length) || (own && checking)" class="people-skeleton" />
    <UiCard v-else-if="error || (own && unavailable)" class="people-empty"><EmptyState icon="user" :title="t(`social.${error || 'loadError'}`)"><UiButton v-if="error !== 'unavailableList'" @click="load()">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <ul v-else-if="people.length" class="people-list"><li v-for="person in people" :key="person.username"><a :href="`/u/${encodeURIComponent(person.username)}`"><UserAvatar :username="person.display_name || person.username" :src="person.has_avatar ? publicAvatarURL(person.username) : ''" /><span><strong>{{ person.display_name || person.username }}</strong><small v-if="person.display_name">@{{ person.username }}</small></span><AppIcon name="arrow" /></a></li></ul>
    <UiCard v-else class="people-empty"><EmptyState icon="users" :title="t(`social.empty.${kind}`)" /></UiCard>
    <UiButton v-if="cursor" class="people-more" variant="secondary" :loading="loading" @click="load(true)">{{ t('search.more') }}</UiButton>
  </div>
</template>
<style scoped>
.people-page { max-width: 760px; min-width: 0; }.public-people { padding-block: var(--wx-space-8); }.people-page h1 { margin: 0 0 var(--wx-space-5); font-size: var(--wx-text-2xl); }.people-tabs { display: flex; flex-wrap: wrap; gap: var(--wx-space-2); margin-bottom: var(--wx-space-5); }.people-list { display: grid; gap: var(--wx-space-3); margin: 0; padding: 0; list-style: none; }.people-list a { display: flex; align-items: center; min-height: 72px; gap: var(--wx-space-3); padding: var(--wx-space-4); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); background: var(--wx-surface); color: var(--wx-text); text-decoration: none; overflow-wrap: anywhere; }.people-list a:hover { border-color: var(--wx-secondary); }.people-list a > span:not(.avatar) { display: grid; flex: 1; gap: var(--wx-space-1); min-width: 0; }.people-list small { color: var(--wx-text-muted); }.people-list .icon { color: var(--wx-primary); }.people-empty { padding: var(--wx-space-6); }.people-skeleton { height: 90px; }.people-more { margin-top: var(--wx-space-4); }
.public-people h1 { margin-top: var(--wx-space-4); }
.public-people .people-back { max-width: 100%; margin-bottom: var(--wx-space-2); padding-inline: var(--wx-space-2); text-align: start; }
.people-back span { min-width: 0; overflow-wrap: anywhere; }
.people-back .icon { transform: rotate(180deg); }
</style>
