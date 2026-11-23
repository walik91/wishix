<script setup>
import { computed, onMounted, onUnmounted, ref, watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { rememberProfileDestination, usePublicProfileFocus } from '../composables/usePublicProfileFocus.js'
import { socialPeople, publicAvatarURL, publicDetails } from '../api/social.js'
import PublicProfileNavigation from '../components/navigation/PublicProfileNavigation.vue'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import UserAvatar from '../components/ui/UserAvatar.vue'
import AppIcon from '../components/ui/AppIcon.vue'
const props = defineProps({ own: Boolean, username: { type: String, default: '' }, initialKind: { type: String, default: 'following' } })
const { user, checking, unavailable, refresh } = useSession()
const name = computed(() => props.own ? user.value?.username || '' : props.username)
const kind = ref(props.initialKind)
const loading = ref(false), people = ref([]), cursor = ref(''), error = ref('')
const sessionChecking = computed(() => props.own && checking.value)
const sessionError = computed(() => props.own && unavailable.value)
const canLoad = computed(() => !sessionChecking.value && !sessionError.value)
const unavailableKind = computed(() => error.value === 'unavailableList' ? kind.value : '')
const details = ref(null)
const detailsError = ref(false)
const profileUsername = computed(() => details.value?.username || props.username)
const heading = props.own ? ref(null) : usePublicProfileFocus()
const peopleList = ref(null)
let requestId = 0

function rememberPerson(event) {
  if (props.own || !rememberProfileDestination(event)) return
  history.replaceState({ ...history.state, profileReturnPath: event.currentTarget.pathname }, '')
}

function restorePersonFocus(fromCache = false) {
  const { profileReturnPath, ...state } = history.state ?? {}
  if (!profileReturnPath) return
  history.replaceState(state, '')
  if (!fromCache && document.activeElement !== document.body) return
  const links = peopleList.value?.querySelectorAll('a[href]') ?? []
  const row = Array.from(links).find(link => link.pathname === profileReturnPath)
  const target = row ?? heading.value
  target?.focus()
}

function restoreCachedFocus(event) {
  if (event.persisted) restorePersonFocus(true)
}

if (!props.own) {
  onMounted(() => {
    const navigation = performance.getEntriesByType('navigation')[0]
    if (history.state?.profileReturnPath && (!navigation || navigation.type === 'back_forward')) {
      if (loading.value) watch(loading, () => restorePersonFocus(), { once: true, flush: 'post' })
      else restorePersonFocus()
    }
    window.addEventListener('pageshow', restoreCachedFocus)
  })
  onUnmounted(() => window.removeEventListener('pageshow', restoreCachedFocus))
}

async function loadDetails() {
  detailsError.value = false
  try { details.value = await publicDetails(props.username) }
  catch (failure) { detailsError.value = failure.status !== 404 }
}
if (!props.own) onMounted(loadDetails)

watchEffect(() => { document.title = `${t(`social.${kind.value}`)} · ${t('appName')}` })
async function load(after = '') {
  const id = ++requestId
  loading.value = true
  error.value = ''
  try {
    const data = await socialPeople(name.value, kind.value, after)
    if (id !== requestId) return
    people.value = after ? [...people.value, ...data.people] : data.people
    cursor.value = data.next_cursor
  } catch (failure) {
    if (id !== requestId) return
    if (failure.status === 404) {
      people.value = []
      cursor.value = ''
      error.value = 'unavailableList'
    } else {
      error.value = 'loadError'
    }
  } finally {
    if (id === requestId) loading.value = false
  }
}

async function retry() {
  if (sessionError.value) {
    await refresh()
    if (!user.value && !unavailable.value) window.location.replace('/login')
  } else {
    await load(cursor.value)
  }
}

watch([name, kind, canLoad], () => {
  requestId++
  people.value = []
  cursor.value = ''
  error.value = ''
  loading.value = false
  if (name.value && canLoad.value) load()
}, { immediate: true })
</script>
<template>
  <div class="people-page" :class="{ 'public-people': !own }">
    <template v-if="!own">
      <UiButton class="people-back" variant="ghost" :href="`/u/${encodeURIComponent(profileUsername)}`" @click="rememberProfileDestination">
        <AppIcon name="arrow" />
        <span>{{ t('social.backToProfile').replace('{username}', `@${profileUsername}`) }}</span>
      </UiButton>
      <PublicProfileNavigation
        :username="profileUsername"
        :current="kind"
        :show-followers="details?.show_followers && unavailableKind !== 'followers'"
        :show-following="details?.show_following && unavailableKind !== 'following'"
      />
    </template>
    <h1 ref="heading" :tabindex="own ? undefined : -1">{{ own ? t('social.title') : t(`social.${kind}`) }}</h1>
    <div v-if="own" class="people-tabs" role="group" :aria-label="t('social.title')"><UiButton v-for="tab in ['following', 'followers', 'blocked']" :key="tab" :variant="kind === tab ? 'primary' : 'secondary'" :aria-pressed="kind === tab" @click="kind = tab">{{ t(`social.${tab}`) }}</UiButton></div>
    <p class="sr-only" role="status">{{ sessionChecking ? t('auth.checkingSession') : loading ? t('social.loading') : '' }}</p>
    <UiSkeleton v-if="sessionChecking || (loading && !people.length)" class="people-skeleton" />
    <ul v-else-if="people.length" ref="peopleList" class="people-list">
      <li v-for="person in people" :key="person.username">
        <a class="person-row" :href="`/u/${encodeURIComponent(person.username)}`" @click="rememberPerson">
          <UserAvatar :username="person.display_name || person.username" :src="person.has_avatar ? publicAvatarURL(person.username) : ''" />
          <span class="person-details">
            <strong>{{ person.display_name || person.username }}</strong>
            <span class="person-address">@{{ person.username }}</span>
          </span>
          <AppIcon name="arrow" />
        </a>
      </li>
    </ul>
    <UiCard v-else-if="!error && !sessionError && (!own || user)" class="people-empty"><EmptyState icon="users" :title="t(`social.empty.${kind}`)" /></UiCard>
    <UiCard v-if="error || sessionError" class="people-empty" :class="{ 'people-more': people.length }">
      <EmptyState :icon="error === 'unavailableList' ? 'lock' : 'globe'" :title="t(`social.${error || 'loadError'}`)">
        <UiButton v-if="error !== 'unavailableList'" @click="retry">{{ t('checkAgain') }}</UiButton>
      </EmptyState>
    </UiCard>
    <UiButton v-else-if="cursor" class="people-more" variant="secondary" :loading="loading" @click="load(cursor)">{{ t('search.more') }}</UiButton>
    <div v-if="detailsError" class="people-details-error">
      <span role="alert">{{ t('social.detailsError') }}</span>
      <UiButton variant="secondary" @click="loadDetails">{{ t('checkAgain') }}</UiButton>
    </div>
  </div>
</template>
<style scoped>
.people-page { max-width: 760px; min-width: 0; }.public-people { padding-block: var(--wx-space-8); }.people-page h1 { margin: 0 0 var(--wx-space-5); font-size: var(--wx-text-2xl); }.people-tabs { display: flex; flex-wrap: wrap; gap: var(--wx-space-2); margin-bottom: var(--wx-space-5); }.people-list { display: grid; gap: var(--wx-space-3); margin: 0; padding: 0; list-style: none; }.people-empty { padding: var(--wx-space-6); }.people-skeleton { height: 90px; }.people-more { margin-top: var(--wx-space-4); }
.person-row {
  display: flex;
  align-items: center;
  gap: var(--wx-space-3);
  min-height: 64px;
  padding: var(--wx-space-3);
  border: 1px solid var(--wx-border);
  border-radius: var(--wx-radius-card);
  background: var(--wx-surface);
  color: var(--wx-text);
  text-decoration: none;
  overflow-wrap: anywhere;
}
.person-row:hover { border-color: var(--wx-secondary); background: var(--wx-background); }
.person-row:active { background: var(--wx-lavender); }
.person-details { display: grid; flex: 1; min-width: 0; }
.person-address { color: var(--wx-text-muted); font-size: var(--wx-text-sm); }
.person-row > .icon { color: var(--wx-primary); }
.public-people h1 { margin-top: var(--wx-space-4); }
.public-people h1:focus-visible { outline: none; }
.public-people .people-back { max-width: 100%; margin-bottom: var(--wx-space-2); padding-inline: var(--wx-space-2); text-align: start; }
.people-back span { min-width: 0; overflow-wrap: anywhere; }
.people-back .icon { transform: rotate(180deg); }
.people-details-error { display: flex; flex-wrap: wrap; align-items: center; gap: var(--wx-space-2); margin-top: var(--wx-space-4); color: var(--wx-error); }
</style>
