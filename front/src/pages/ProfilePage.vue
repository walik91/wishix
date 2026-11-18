<script setup>
import { computed, ref, watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { getProfileName, setProfileName } from '../api/search.js'
import { getProfileDetails, saveProfileDetails, changeUsername, uploadAvatar, deleteAvatar } from '../api/social.js'
import TextField from '../components/ui/TextField.vue'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiDialog from '../components/ui/UiDialog.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import UserAvatar from '../components/ui/UserAvatar.vue'
const { user, checking, unavailable, refresh } = useSession()
const loading = ref(true), busy = ref(false), error = ref(''), notice = ref(''), settings = ref(null)
const displayName = ref(''), showName = ref(false), username = ref(''), redirectOld = ref(true), confirmRename = ref(false), avatarVersion = ref(Date.now()), picker = ref(null)
const avatarURL = computed(() => settings.value?.has_avatar ? `/api/profile/avatar?v=${avatarVersion.value}` : '')
watchEffect(() => { document.title = `${t('profile.editProfile')} · ${t('appName')}` })
watch([user, checking, unavailable], async ([account, pending, failed]) => { if (account && !pending && !failed && !settings.value) await load() }, { immediate: true })
watch(displayName, value => { if (!value.trim()) showName.value = false })
watch(() => settings.value?.birth_date, value => { if (!value && settings.value) settings.value.show_age = false })
async function load() {
  loading.value = true; error.value = ''
  try { const [details, name] = await Promise.all([getProfileDetails(), getProfileName()]); details.birth_date ||= ''; settings.value = details; username.value = details.username; displayName.value = name.display_name; showName.value = name.show_display_name }
  catch { error.value = t('profile.loadError') }
  finally { loading.value = false }
}
async function saveName() {
  if (busy.value) return
  if ([...displayName.value.trim()].length > 100) { error.value = t('profile.nameTooLong'); return }
  busy.value = true; error.value = ''; notice.value = ''
  try { const data = await setProfileName(displayName.value, !!displayName.value.trim() && showName.value); displayName.value = data.display_name; showName.value = data.show_display_name; notice.value = t('profile.saved') }
  catch { error.value = t('profile.nameError') } finally { busy.value = false }
}
async function saveDetails() {
  if (busy.value) return
  busy.value = true; error.value = ''; notice.value = ''
  const value = settings.value
  try { settings.value = await saveProfileDetails({ biography: value.biography, birth_date: value.birth_date || null, show_age: !!value.birth_date && value.show_age, show_biography: value.show_biography, show_avatar: value.show_avatar, show_followers: value.show_followers, show_following: value.show_following }); settings.value.birth_date ||= ''; notice.value = t('profile.saved') }
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
    <h1>{{ t('profile.editProfile') }}</h1>
    <p v-if="error" class="profile-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="profile-success" role="status">{{ notice }}</p>
    <UiSkeleton v-if="checking || loading" class="profile-skeleton" />
    <UiCard v-else-if="!settings" class="profile-panel"><p>{{ t('profile.loadError') }}</p><UiButton @click="load">{{ t('checkAgain') }}</UiButton></UiCard>
    <template v-else>
      <UiCard class="profile-panel">
        <h2>{{ t('profile.avatar') }}</h2><p>{{ t('profile.avatarHint') }}</p>
        <div class="avatar-row"><UserAvatar :username="username" :src="avatarURL" /><input ref="picker" class="sr-only" type="file" accept="image/jpeg,image/png,image/webp" tabindex="-1" :aria-label="t('profile.chooseAvatar')" :disabled="busy" @change="selectAvatar" /><UiButton variant="secondary" :disabled="busy" @click="picker.click()">{{ t('profile.chooseAvatar') }}</UiButton><UiButton v-if="settings.has_avatar" variant="secondary" :disabled="busy" @click="removeAvatar">{{ t('profile.removeAvatar') }}</UiButton></div>
      </UiCard>
      <UiCard class="profile-panel"><form @submit.prevent="saveName"><h2>{{ t('profile.displayName') }}</h2><TextField id="profile-name-page" v-model="displayName" :label="t('profile.displayName')" :hint="t('profile.nameHint')" maxlength="200" :disabled="busy" /><label class="privacy"><input v-model="showName" type="checkbox" :disabled="busy || !displayName.trim()" />{{ t('profile.showName') }}</label><p>{{ t('profile.showNameHint') }}</p><UiButton type="submit" :loading="busy">{{ t('profile.saveName') }}</UiButton></form></UiCard>
      <UiCard class="profile-panel"><form @submit.prevent="saveDetails"><h2>{{ t('profile.about') }}</h2>
        <TextField id="profile-birth-date" v-model="settings.birth_date" type="date" :label="t('profile.birthDate')" :hint="t('profile.birthDateHint')" :disabled="busy" />
        <label class="privacy"><input v-model="settings.show_age" type="checkbox" :disabled="busy || !settings.birth_date" />{{ t('profile.showAge') }}</label>
        <TextField id="profile-biography" v-model="settings.biography" multiline rows="4" maxlength="4000" :label="t('profile.biography')" :hint="t('profile.biographyHint')" :disabled="busy" />
        <label class="privacy"><input v-model="settings.show_biography" type="checkbox" :disabled="busy" />{{ t('profile.showBiography') }}</label>
        <label class="privacy"><input v-model="settings.show_avatar" type="checkbox" :disabled="busy" />{{ t('profile.showAvatar') }}</label>
        <label class="privacy"><input v-model="settings.show_followers" type="checkbox" :disabled="busy" />{{ t('profile.showFollowers') }}</label>
        <label class="privacy"><input v-model="settings.show_following" type="checkbox" :disabled="busy" />{{ t('profile.showFollowing') }}</label>
        <UiButton type="submit" :loading="busy">{{ t('profile.saveSettings') }}</UiButton>
      </form></UiCard>
      <UiCard class="profile-panel"><form @submit.prevent="confirmRename = true"><h2>{{ t('profile.changeUsername') }}</h2><TextField id="profile-username" v-model="username" :label="t('register.username')" maxlength="30" :hint="t('profile.usernameHint')" :disabled="busy" /><label class="privacy"><input v-model="redirectOld" type="checkbox" :disabled="busy" />{{ t('profile.redirectOld') }}</label><p>{{ t(redirectOld ? 'profile.redirectHint' : 'profile.noRedirectHint') }}</p><UiButton type="submit" variant="secondary" :disabled="busy || username.trim().toLowerCase() === settings.username">{{ t('profile.changeUsername') }}</UiButton></form></UiCard>
      <UiDialog v-model:open="confirmRename" :title="t('profile.changeUsername')" :busy="busy"><p>{{ t('profile.renameConfirm') }} <strong>{{ username }}</strong></p><p>{{ t('profile.usernameHint') }}</p><p v-if="error" class="profile-error" role="alert">{{ error }}</p><div class="profile-actions"><UiButton :loading="busy" @click="rename">{{ t('profile.changeUsername') }}</UiButton><UiButton variant="secondary" :disabled="busy" @click="confirmRename = false">{{ t('wishes.cancel') }}</UiButton></div></UiDialog>
    </template>
  </div>
</template>
<style scoped>
.profile-page { max-width: 760px; display: grid; gap: var(--wx-space-4); }.profile-page h1 { margin: 0 0 var(--wx-space-2); font-size: var(--wx-text-2xl); }.profile-panel { padding: var(--wx-space-5); }.profile-panel h2 { margin: 0; font-size: var(--wx-text-lg); }.profile-panel form { display: grid; gap: var(--wx-space-3); }.profile-panel p { color: var(--wx-text-muted); line-height: 1.5; }.privacy { display: flex; align-items: center; gap: var(--wx-space-3); min-height: 44px; }.privacy input { width: 20px; height: 20px; accent-color: var(--wx-primary); }.avatar-row,.profile-actions { display: flex; align-items: center; gap: var(--wx-space-3); flex-wrap: wrap; }.profile-error { color: var(--wx-error); }.profile-success { color: var(--wx-success); }.profile-skeleton { height: 220px; }
</style>
