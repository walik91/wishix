<script setup>
import { onMounted, ref, watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { publicProfile } from '../api/lists.js'
import { publicDetails, publicAvatarURL, socialState, setFollow, setBlock, sendReport } from '../api/social.js'
import { useSession } from '../composables/useSession.js'
import { sharedListPath } from '../sharedListPath.js'
import PublicProfileNavigation from '../components/navigation/PublicProfileNavigation.vue'
import ListCard from '../components/wishes/ListCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiButton from '../components/ui/UiButton.vue'
import UiDialog from '../components/ui/UiDialog.vue'
import TextField from '../components/ui/TextField.vue'
import UserAvatar from '../components/ui/UserAvatar.vue'
import EmptyState from '../components/ui/EmptyState.vue'
const props = defineProps({ username: { type: String, required: true } })
const { user, checking } = useSession()
const username = ref(''), displayName = ref(''), showAllWishes = ref(false), lists = ref([]), details = ref(null), state = ref(null), loading = ref(true), error = ref(''), actionError = ref(''), notice = ref(''), busy = ref(false), dialog = ref(''), report = ref('')
const detailsError = ref(false), stateError = ref(false)
watchEffect(() => { document.title = `${displayName.value || username.value || t('lists.publicProfile')} · ${t('appName')}` })
async function load() {
  loading.value = true; error.value = ''
  try {
    const data = await publicProfile(props.username)
    if (data.username !== props.username.toLowerCase()) { window.location.replace(`/u/${encodeURIComponent(data.username)}`); return }
    username.value = data.username; displayName.value = data.display_name || ''; showAllWishes.value = data.show_all_wishes; lists.value = data.lists
    try { details.value = await publicDetails(data.username); detailsError.value = false } catch { details.value = null; detailsError.value = true }
  } catch (failure) { error.value = failure.status === 404 ? 'list_not_found' : 'unavailable' }
  finally { loading.value = false }
}
async function loadState() {
  state.value = null; stateError.value = false
  if (!username.value || !user.value || checking.value) return
  try { state.value = await socialState(username.value) } catch { stateError.value = true }
}
watch([username, user, checking], loadState, { immediate: true })
async function follow() {
  if (busy.value) return; busy.value = true; actionError.value = ''; notice.value = ''
  try { state.value = await setFollow(username.value, !state.value.following) } catch { actionError.value = t('social.actionError') } finally { busy.value = false }
}
async function block() {
  if (busy.value) return; busy.value = true; actionError.value = ''; notice.value = ''
  try { state.value = await setBlock(username.value, !state.value.blocked); dialog.value = ''; notice.value = t(state.value.blocked ? 'social.blockedNotice' : 'social.unblockedNotice') } catch { actionError.value = t('social.actionError') } finally { busy.value = false }
}
async function submitReport() {
  if (busy.value) return; busy.value = true; actionError.value = ''; notice.value = ''
  try { await sendReport(username.value, report.value); report.value = ''; dialog.value = ''; notice.value = t('social.reportSaved') } catch { actionError.value = t('social.actionError') } finally { busy.value = false }
}
function openDialog(kind) { actionError.value = ''; notice.value = ''; dialog.value = kind }
onMounted(load)
</script>
<template>
  <div class="lists-page public-profile-page">
    <UiSkeleton v-if="loading" class="list-form-skeleton" />
    <UiCard v-else-if="error" class="list-empty"><EmptyState icon="user" :title="t('lists.profileUnavailable')"><UiButton v-if="error !== 'list_not_found'" @click="load">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <template v-else>
      <header class="list-page-heading"><div class="profile-heading"><UserAvatar :username="displayName || username" :src="details?.has_avatar ? publicAvatarURL(username) : ''" /><div><h1>{{ displayName || username }}</h1><p v-if="details?.age !== undefined && details?.age !== null" class="list-muted">{{ t('profile.age') }}: {{ details.age }}</p><p v-if="details?.biography" class="profile-biography">{{ details.biography }}</p></div></div></header>
      <p v-if="notice" role="status">{{ notice }}</p><p v-if="actionError && !dialog" class="profile-error" role="alert">{{ actionError }}</p>
      <div v-if="detailsError || stateError" class="profile-actions" role="alert"><span class="profile-error">{{ t(detailsError ? 'social.detailsError' : 'social.stateError') }}</span><UiButton variant="secondary" @click="detailsError ? load() : loadState()">{{ t('checkAgain') }}</UiButton></div>
      <div class="profile-actions">
        <template v-if="user && state && !state.self"><UiButton v-if="state.can_follow || state.following" :disabled="busy" :variant="state.following ? 'secondary' : 'primary'" @click="follow">{{ t(state.following ? 'social.unfollow' : 'social.follow') }}</UiButton><UiButton variant="secondary" :disabled="busy" @click="state.blocked ? block() : openDialog('block')">{{ t(state.blocked ? 'social.unblock' : 'social.block') }}</UiButton><UiButton variant="secondary" :disabled="busy" @click="openDialog('report')">{{ t('social.report') }}</UiButton><span v-if="!state.can_follow && !state.blocked" class="list-muted">{{ t('social.cannotFollow') }}</span></template>
        <UiButton v-else-if="!user && !checking" href="/login" variant="secondary">{{ t('social.signIn') }}</UiButton>
        <UiButton v-if="showAllWishes" variant="secondary" :href="`/u/${encodeURIComponent(username)}/wishes`">{{ t('profile.allWishes') }}</UiButton>
      </div>
      <PublicProfileNavigation
        :username="username"
        current="lists"
        :show-followers="details?.show_followers"
        :show-following="details?.show_following"
      />
      <h2 class="public-lists-title">{{ t('lists.publicLists') }}</h2>
      <div v-if="lists.length" class="list-grid"><ListCard v-for="(list, index) in lists" :key="index" :list="list" :href="sharedListPath(username, list.share_path)" :manage="false" /></div>
      <UiCard v-else class="list-empty"><EmptyState icon="list" :title="t('lists.noPublicLists')" /></UiCard>
      <UiDialog :open="dialog === 'block'" :busy="busy" :title="t('social.block')" @update:open="!$event && (dialog = '')"><p>{{ t('social.blockConfirm') }}</p><p v-if="actionError" class="profile-error" role="alert">{{ actionError }}</p><div class="profile-actions"><UiButton variant="destructive" :loading="busy" @click="block">{{ t('social.block') }}</UiButton><UiButton variant="secondary" :disabled="busy" @click="dialog = ''">{{ t('wishes.cancel') }}</UiButton></div></UiDialog>
      <UiDialog :open="dialog === 'report'" :busy="busy" :title="t('social.report')" @update:open="!$event && (dialog = '')"><form class="report-form" @submit.prevent="submitReport"><TextField id="social-report" v-model="report" multiline rows="4" maxlength="2000" required :label="t('social.reportReason')" :hint="t('social.reportHint')" /><p v-if="actionError" class="profile-error" role="alert">{{ actionError }}</p><UiButton type="submit" :loading="busy">{{ t('social.report') }}</UiButton></form></UiDialog>
    </template>
  </div>
</template>
<style scoped>
.public-profile-page { padding-block: var(--wx-space-8); }.public-profile-page h1 { color: var(--wx-primary-pressed); overflow-wrap: anywhere; }.profile-heading { display: flex; align-items: flex-start; gap: var(--wx-space-4); min-width: 0; }.profile-heading > div { min-width: 0; }.profile-biography { white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.6; }.profile-actions { display: flex; flex-wrap: wrap; align-items: center; gap: var(--wx-space-3); margin-block: var(--wx-space-4) var(--wx-space-6); }.public-lists-title { font-size: var(--wx-text-lg); margin-top: var(--wx-space-6); }.profile-error { color: var(--wx-error); }.report-form { display: grid; gap: var(--wx-space-4); }
</style>
