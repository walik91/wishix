<script setup>
import { computed, nextTick, ref, watch } from 'vue'
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
const loading = ref(true), operation = ref(''), settings = ref(null)
const savedFields = ref(null)
const displayName = ref(''), savedName = ref(null), nameLoadFailed = ref(false)
const saveError = ref(''), saveNotice = ref(false)
const avatarError = ref(''), avatarNotice = ref(''), addressError = ref(''), addressNotice = ref(false)
const username = ref(''), confirmRename = ref(false), avatarVersion = ref(Date.now()), picker = ref(null)
const content = ref(null), nameField = ref(null), biographyField = ref(null), saveStatus = ref(null)
const busy = computed(() => !!operation.value)
const avatarURL = computed(() => settings.value?.has_avatar ? `/api/profile/avatar?v=${avatarVersion.value}` : '')
const nameChanged = computed(() => savedName.value !== null && displayName.value !== savedName.value)
const detailsChanged = computed(() => {
  if (!settings.value) return false
  const fields = editableFields(settings.value)
  return Object.keys(fields).some(key => fields[key] !== savedFields.value[key])
})
const hasProfileChanges = computed(() => nameChanged.value || detailsChanged.value)
const hasUnsavedChanges = computed(() => hasProfileChanges.value || (!!settings.value && username.value !== settings.value.username))
defineExpose({ hasUnsavedChanges, busy })

function editableFields(value) {
  return {
    biography: value.biography,
    birth_date: value.birth_date || null,
    show_age: !!value.birth_date && value.show_age,
    show_followers: value.show_followers,
    show_following: value.show_following,
  }
}
function acceptSavedDetails(details) {
  savedFields.value = editableFields(details)
  settings.value = { ...details, birth_date: details.birth_date || '' }
}

watch([user, checking, unavailable], ([account, pending, failed]) => {
  if (!account || pending || failed) return
  if (!settings.value) load()
  if (savedName.value === null) loadName()
}, { immediate: true })
watch(() => settings.value?.birth_date, value => { if (!value && settings.value) settings.value.show_age = false })
watch(hasProfileChanges, changed => {
  if (!changed) return
  saveNotice.value = false
  const focused = document.activeElement
  if (content.value?.contains(focused)) focused.scrollIntoView({ block: 'nearest' })
}, { flush: 'post' })

async function load() {
  loading.value = true
  try {
    const details = await getProfileDetails()
    acceptSavedDetails(details)
    username.value = details.username
  } catch { settings.value = null }
  finally { loading.value = false }
}
async function loadName() {
  nameLoadFailed.value = false
  try {
    const data = await getProfileName()
    displayName.value = savedName.value = data.display_name
  } catch { nameLoadFailed.value = true }
}
async function saveProfile(event) {
  if (busy.value || !hasProfileChanges.value) return
  saveError.value = ''
  const name = displayName.value.trim()
  if (nameChanged.value && (!name || [...name].length > 100)) {
    saveError.value = name ? 'profile.nameTooLong' : 'profile.nameRequired'
    nameField.value.focus()
    return
  }

  operation.value = 'saveProfile'
  try {
    if (nameChanged.value) {
      const data = await setProfileName(name)
      displayName.value = savedName.value = data.display_name
    }
    if (detailsChanged.value) acceptSavedDetails(await saveProfileDetails(editableFields(settings.value)))
    saveNotice.value = true
  } catch (failure) {
    saveError.value = nameChanged.value ? 'profile.nameSaveError'
      : failure.code === 'invalid_input' ? 'profile.invalidDetails' : 'profile.saveError'
  } finally {
    operation.value = ''
    await nextTick()
    if (document.activeElement === document.body) {
      const target = saveNotice.value ? saveStatus.value : event.submitter
      target?.focus({ preventScroll: true })
    }
  }
}
async function cancelProfileChanges() {
  if (busy.value) return
  const field = nameChanged.value ? nameField.value : biographyField.value
  if (nameChanged.value) displayName.value = savedName.value
  if (detailsChanged.value) Object.assign(settings.value, savedFields.value, { birth_date: savedFields.value.birth_date || '' })
  saveError.value = ''
  await nextTick()
  field?.focus()
}
function openRename() {
  addressError.value = ''
  addressNotice.value = false
  confirmRename.value = true
}
async function rename() {
  if (busy.value) return
  operation.value = 'rename'
  addressError.value = ''
  try {
    const updated = await changeUsername(username.value)
    settings.value.username = updated.username
    settings.value.username_changed_at = updated.username_changed_at
    username.value = updated.username
    confirmRename.value = false
    await refresh()
    addressNotice.value = true
  } catch (failure) {
    addressError.value = { username_taken: 'profile.usernameTaken', username_cooldown: 'profile.usernameCooldown', username_invalid: 'profile.usernameInvalid' }[failure.code] || 'profile.saveError'
  } finally { operation.value = '' }
}
async function selectAvatar(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file || busy.value) return
  avatarError.value = ''
  avatarNotice.value = ''
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || file.size > 5 * 1024 * 1024) {
    avatarError.value = 'profile.avatarInvalid'
    return
  }
  operation.value = 'uploadAvatar'
  try {
    await uploadAvatar(file)
    settings.value.has_avatar = true
    avatarVersion.value = Date.now()
    avatarNotice.value = 'profile.avatarSaved'
  } catch { avatarError.value = 'profile.avatarInvalid' }
  finally { operation.value = '' }
}
async function removeAvatar() {
  if (busy.value) return
  operation.value = 'removeAvatar'
  avatarError.value = ''
  avatarNotice.value = ''
  try {
    await deleteAvatar()
    settings.value.has_avatar = false
    avatarNotice.value = 'profile.avatarRemoved'
  } catch { avatarError.value = 'profile.saveError' }
  finally { operation.value = '' }
}
</script>

<template>
  <div class="profile-page" :aria-busy="checking || loading || undefined">
    <div ref="content" class="settings-scroll profile-scroll">
      <UiSkeleton v-if="checking || loading" class="profile-skeleton" />
      <UiCard v-else-if="!settings" class="profile-panel"><p>{{ t('profile.loadError') }}</p><UiButton @click="load">{{ t('checkAgain') }}</UiButton></UiCard>
      <div v-else class="profile-photo">
        <div class="avatar-row">
          <UserAvatar :username="settings.username" :src="avatarURL" />
          <input ref="picker" class="sr-only" type="file" accept="image/jpeg,image/png,image/webp" tabindex="-1" :aria-label="t('profile.chooseAvatar')" :disabled="busy" @change="selectAvatar" />
          <UiButton variant="secondary" :loading="operation === 'uploadAvatar'" :disabled="busy" @click="picker.click()">{{ t('profile.chooseAvatar') }}</UiButton>
          <UiButton v-if="settings.has_avatar" variant="ghost" :loading="operation === 'removeAvatar'" :disabled="busy" @click="removeAvatar">{{ t('profile.removeAvatar') }}</UiButton>
        </div>
        <p class="profile-hint">{{ t('profile.avatarHint') }}</p>
        <p v-if="avatarError" class="profile-error" role="alert">{{ t(avatarError) }}</p>
        <p v-if="avatarNotice" class="profile-success" role="status">{{ t(avatarNotice) }}</p>
      </div>
      <form id="profile-details-form" class="profile-panel" @submit.prevent="saveProfile" @input="saveError = ''">
        <TextField id="profile-display-name" ref="nameField" v-model="displayName" :label="t('profile.displayName')" :hint="t('profile.nameHint')" aria-required="true" maxlength="200" :disabled="busy || savedName === null" />
        <p v-if="nameLoadFailed" class="profile-error" role="alert">{{ t('profile.nameError') }}</p>
        <template v-if="settings">
          <TextField id="profile-biography" ref="biographyField" v-model="settings.biography" multiline rows="3" maxlength="4000" :label="t('profile.biography')" :hint="t('profile.biographyHint')" :disabled="busy" />
          <TextField id="profile-birth-date" v-model="settings.birth_date" type="date" :label="t('profile.birthDate')" :hint="t('profile.birthDateHint')" :disabled="busy" />
          <div class="profile-privacy">
            <label class="privacy"><input v-model="settings.show_age" type="checkbox" :disabled="busy || !settings.birth_date" />{{ t('profile.showAge') }}</label>
            <label class="privacy"><input v-model="settings.show_followers" type="checkbox" :disabled="busy" />{{ t('profile.showFollowers') }}</label>
            <label class="privacy"><input v-model="settings.show_following" type="checkbox" :disabled="busy" />{{ t('profile.showFollowing') }}</label>
          </div>
        </template>
      </form>
      <slot name="collection" />
      <form v-if="settings" class="profile-address" @submit.prevent="openRename">
        <TextField id="profile-username" v-model="username" :label="t('profile.address')" maxlength="30" :hint="t('profile.addressHint')" :disabled="busy" @update:model-value="addressError = ''; addressNotice = false" />
        <p class="profile-hint">{{ t('profile.usernameHint') }} {{ t('profile.redirectHint') }}</p>
        <UiButton type="submit" variant="secondary" :disabled="busy || username.trim().toLowerCase() === settings.username">{{ t('profile.changeUsername') }}</UiButton>
        <p v-if="addressError && !confirmRename" class="profile-error" role="alert">{{ t(addressError) }}</p>
        <p v-if="addressNotice" class="profile-success" role="status">{{ t('profile.usernameSaved') }}</p>
      </form>
      <div class="profile-links"><slot name="links" /></div>
    </div>
    <div v-if="hasProfileChanges || saveNotice" class="profile-footer">
      <p v-if="saveError" class="profile-error" role="alert">{{ t(saveError) }}</p>
      <p v-if="saveNotice" ref="saveStatus" class="profile-success" role="status" tabindex="-1">{{ t('profile.saved') }}</p>
      <div v-if="hasProfileChanges" class="profile-actions">
        <UiButton variant="secondary" :disabled="busy" @click="cancelProfileChanges">{{ t('profile.cancelChanges') }}</UiButton>
        <UiButton type="submit" form="profile-details-form" :loading="operation === 'saveProfile'" :disabled="busy">{{ t('profile.saveChanges') }}</UiButton>
      </div>
    </div>
    <UiDialog v-if="settings" v-model:open="confirmRename" :title="t('profile.changeUsername')" :busy="busy">
      <p>{{ t('profile.renameConfirm') }} <strong>{{ username }}</strong></p>
      <p>{{ t('profile.usernameHint') }}</p>
      <p v-if="addressError" class="profile-error" role="alert">{{ t(addressError) }}</p>
      <div class="profile-actions">
        <UiButton :loading="operation === 'rename'" @click="rename">{{ t('profile.changeUsername') }}</UiButton>
        <UiButton variant="secondary" :disabled="busy" @click="confirmRename = false">{{ t('wishes.cancel') }}</UiButton>
      </div>
    </UiDialog>
  </div>
</template>
<style scoped>
.profile-page { display: flex; flex-direction: column; min-height: 0; }
.profile-scroll {
  display: grid;
  align-content: start;
  gap: var(--wx-space-4);
  min-height: 0;
  padding: var(--wx-space-4) var(--settings-padding-inline) var(--settings-padding-bottom);
  overflow: auto;
  overscroll-behavior: contain;
  scroll-padding-block: var(--wx-space-2);
}
.profile-photo, .profile-address, .profile-panel { display: grid; gap: var(--wx-space-3); min-width: 0; }
.profile-panel, .profile-address { padding-bottom: var(--wx-space-4); border-bottom: 1px solid var(--wx-border); }
.profile-address > .button { justify-self: start; }
.profile-panel :deep(textarea) { min-height: 0; }
.profile-hint, .profile-error, .profile-success { margin: 0; font-size: var(--wx-text-sm); line-height: 1.5; }
.profile-hint { color: var(--wx-text-muted); }
.privacy { display: flex; align-items: center; gap: var(--wx-space-3); min-height: 44px; font-size: var(--wx-text-sm); cursor: pointer; }
.privacy input { flex: 0 0 auto; width: 20px; height: 20px; accent-color: var(--wx-primary); }
.avatar-row, .profile-actions, .profile-links { display: flex; align-items: center; gap: var(--wx-space-2); flex-wrap: wrap; }
.profile-footer {
  flex: none;
  display: grid;
  gap: var(--wx-space-2);
  padding: var(--wx-space-2) var(--settings-padding-inline) var(--settings-padding-bottom);
  border-top: 1px solid var(--wx-border);
}
.profile-footer .profile-actions { justify-content: flex-end; }
.profile-footer .button, .avatar-row .button { min-height: 44px; padding: var(--wx-space-2) var(--wx-space-3); }
.profile-error { color: var(--wx-error); }
.profile-success { color: var(--wx-success); }
.profile-skeleton { height: 220px; }
</style>
