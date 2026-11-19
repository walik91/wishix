<script setup>
import { computed, ref, watch } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { getProfileDetails, saveProfileDetails, changeUsername, uploadAvatar, deleteAvatar } from '../api/social.js'
import TextField from '../components/ui/TextField.vue'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiDialog from '../components/ui/UiDialog.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import UserAvatar from '../components/ui/UserAvatar.vue'
const { user, checking, unavailable, refresh } = useSession()
const loading = ref(true), busy = ref(false), error = ref(''), notice = ref(''), settings = ref(null)
const username = ref(''), redirectOld = ref(true), confirmRename = ref(false), avatarVersion = ref(Date.now()), picker = ref(null)
const avatarURL = computed(() => settings.value?.has_avatar ? `/api/profile/avatar?v=${avatarVersion.value}` : '')
watch([user, checking, unavailable], async ([account, pending, failed]) => { if (account && !pending && !failed && !settings.value) await load() }, { immediate: true })
watch(() => settings.value?.birth_date, value => { if (!value && settings.value) settings.value.show_age = false })
async function load() {
  loading.value = true; error.value = ''
  try { const details = await getProfileDetails(); details.birth_date ||= ''; settings.value = details; username.value = details.username }
  catch { error.value = t('profile.loadError') }
  finally { loading.value = false }
}
async function saveDetails() {
  if (busy.value) return
  busy.value = true; error.value = ''; notice.value = ''
  const value = settings.value
  try { settings.value = await saveProfileDetails({ biography: value.biography, birth_date: value.birth_date || null, show_age: !!value.birth_date && value.show_age, show_followers: value.show_followers, show_following: value.show_following }); settings.value.birth_date ||= ''; notice.value = t('profile.saved') }
  catch (failure) { error.value = failure.code === 'invalid_input' ? t('profile.invalidDetails') : t('profile.saveError') } finally { busy.value = false }
}
async function rename() {
  if (busy.value) return
  busy.value = true; error.value = ''; notice.value = ''
  try { const updated = await changeUsername(username.value, redirectOld.value); settings.value.username = updated.username; settings.value.username_changed_at = updated.username_changed_at; username.value = updated.username; confirmRename.value = false; await refresh(); notice.value = t('profile.usernameSaved') }
  catch (failure) { error.value = t({ username_taken: 'profile.usernameTaken', username_cooldown: 'profile.usernameCooldown', username_invalid: 'profile.usernameInvalid' }[failure.code] || 'profile.saveError') }
  finally { busy.value = false }
}
async function selectAvatar(event) {
  const file = event.target.files?.[0]; event.target.value = ''
  if (!file || busy.value) return
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || file.size > 5 * 1024 * 1024) { error.value = t('profile.avatarInvalid'); return }
  busy.value = true; error.value = ''; notice.value = ''
  try { await uploadAvatar(file); settings.value.has_avatar = true; avatarVersion.value = Date.now(); notice.value = t('profile.saved') }
  catch { error.value = t('profile.avatarInvalid') } finally { busy.value = false }
}
async function removeAvatar() {
  if (busy.value) return
  busy.value = true; error.value = ''; notice.value = ''
  try { await deleteAvatar(); settings.value.has_avatar = false; notice.value = t('profile.saved') }
  catch { error.value = t('profile.saveError') } finally { busy.value = false }
}
</script>

<template>
  <div class="profile-page" :aria-busy="checking || loading || busy">
    <h2>{{ t('profile.editProfile') }}</h2>
    <p v-if="error" class="profile-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="profile-success" role="status">{{ notice }}</p>
    <UiSkeleton v-if="checking || loading" class="profile-skeleton" />
    <UiCard v-else-if="!settings" class="profile-panel"><p>{{ t('profile.loadError') }}</p><UiButton @click="load">{{ t('checkAgain') }}</UiButton></UiCard>
    <template v-else>
      <section class="profile-panel">
        <h3>{{ t('profile.avatar') }}</h3><p>{{ t('profile.avatarHint') }}</p>
        <div class="avatar-row"><UserAvatar :username="username" :src="avatarURL" /><input ref="picker" class="sr-only" type="file" accept="image/jpeg,image/png,image/webp" tabindex="-1" :aria-label="t('profile.chooseAvatar')" :disabled="busy" @change="selectAvatar" /><UiButton variant="secondary" :disabled="busy" @click="picker.click()">{{ t('profile.chooseAvatar') }}</UiButton><UiButton v-if="settings.has_avatar" variant="secondary" :disabled="busy" @click="removeAvatar">{{ t('profile.removeAvatar') }}</UiButton></div>
      </section>
      <section class="profile-panel"><form @submit.prevent="saveDetails"><h3>{{ t('profile.about') }}</h3>
        <TextField id="profile-birth-date" v-model="settings.birth_date" type="date" :label="t('profile.birthDate')" :hint="t('profile.birthDateHint')" :disabled="busy" />
        <label class="privacy"><input v-model="settings.show_age" type="checkbox" :disabled="busy || !settings.birth_date" />{{ t('profile.showAge') }}</label>
        <TextField id="profile-biography" v-model="settings.biography" multiline rows="4" maxlength="4000" :label="t('profile.biography')" :hint="t('profile.biographyHint')" :disabled="busy" />
        <label class="privacy"><input v-model="settings.show_followers" type="checkbox" :disabled="busy" />{{ t('profile.showFollowers') }}</label>
        <label class="privacy"><input v-model="settings.show_following" type="checkbox" :disabled="busy" />{{ t('profile.showFollowing') }}</label>
        <UiButton type="submit" :loading="busy">{{ t('profile.saveSettings') }}</UiButton>
      </form></section>
      <section class="profile-panel"><form @submit.prevent="confirmRename = true"><h3>{{ t('profile.changeUsername') }}</h3><TextField id="profile-username" v-model="username" :label="t('register.username')" maxlength="30" :hint="t('profile.usernameHint')" :disabled="busy" /><label class="privacy"><input v-model="redirectOld" type="checkbox" :disabled="busy" />{{ t('profile.redirectOld') }}</label><p>{{ t(redirectOld ? 'profile.redirectHint' : 'profile.noRedirectHint') }}</p><UiButton type="submit" variant="secondary" :disabled="busy || username.trim().toLowerCase() === settings.username">{{ t('profile.changeUsername') }}</UiButton></form></section>
      <UiDialog v-model:open="confirmRename" :title="t('profile.changeUsername')" :busy="busy"><p>{{ t('profile.renameConfirm') }} <strong>{{ username }}</strong></p><p>{{ t('profile.usernameHint') }}</p><p v-if="error" class="profile-error" role="alert">{{ error }}</p><div class="profile-actions"><UiButton :loading="busy" @click="rename">{{ t('profile.changeUsername') }}</UiButton><UiButton variant="secondary" :disabled="busy" @click="confirmRename = false">{{ t('wishes.cancel') }}</UiButton></div></UiDialog>
    </template>
  </div>
</template>
<style scoped>
.profile-page { min-width: 0; display: grid; gap: var(--wx-space-4); }.profile-page > h2 { margin: 0; font-size: var(--wx-text-lg); }.profile-panel { min-width: 0; padding-bottom: var(--wx-space-4); border-bottom: 1px solid var(--wx-border); }.profile-panel h3 { margin: 0; font-size: var(--wx-text-md); }.profile-panel form { display: grid; gap: var(--wx-space-3); }.profile-panel p { color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.5; }.privacy { display: flex; align-items: center; gap: var(--wx-space-3); min-height: 44px; }.privacy input { flex: 0 0 auto; width: 20px; height: 20px; accent-color: var(--wx-primary); }.avatar-row,.profile-actions { display: flex; align-items: center; gap: var(--wx-space-3); flex-wrap: wrap; }.profile-error { color: var(--wx-error); }.profile-success { color: var(--wx-success); }.profile-skeleton { height: 220px; }
</style>
