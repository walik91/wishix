<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { locale, t } from '../i18n.js'
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
const username = ref(''), addressEditing = ref(false), confirmRename = ref(false)
const addressToggle = ref(null), addressField = ref(null), addressCooldown = ref(null)
const avatarVersion = ref(Date.now()), picker = ref(null), choosePhoto = ref(null)
const content = ref(null), nameField = ref(null), biographyField = ref(null), saveStatus = ref(null)
const busy = computed(() => !!operation.value)
const avatarURL = computed(() => settings.value?.has_avatar ? `/api/profile/avatar?v=${avatarVersion.value}` : '')
const profileOrigin = window.location.origin
const profilePath = computed(() => `/u/${encodeURIComponent(settings.value.username)}`)
const newUsername = computed(() => username.value.trim().toLowerCase())
const nextAddressChange = computed(() => Date.parse(settings.value?.username_changed_at) + 30 * 24 * 60 * 60 * 1000)
const now = ref(Date.now())
const addressLocked = computed(() => now.value < nextAddressChange.value)
const addressAvailableAt = computed(() => new Intl.DateTimeFormat(locale.value, { dateStyle: 'long', timeStyle: 'short' }).format(nextAddressChange.value))
const addressFieldError = computed(() => ['profile.usernameInvalid', 'profile.usernameTaken'].includes(addressError.value) ? addressError.value : '')
const nameChanged = computed(() => savedName.value !== null && displayName.value !== savedName.value)
const detailsChanged = computed(() => {
  if (!settings.value) return false
  const fields = editableFields(settings.value)
  return Object.keys(fields).some(key => fields[key] !== savedFields.value[key])
})
const hasProfileChanges = computed(() => nameChanged.value || detailsChanged.value)
const hasUnsavedChanges = computed(() => hasProfileChanges.value || (addressEditing.value && username.value !== settings.value?.username))
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
  if (!addressEditing.value || username.value === settings.value?.username) username.value = details.username
  savedFields.value = editableFields(details)
  settings.value = { ...details, birth_date: details.birth_date || '' }
}

watch([user, checking, unavailable], ([account, pending, failed]) => {
  if (!account || pending || failed) return
  if (!settings.value) load()
  if (savedName.value === null) loadName()
}, { immediate: true })
watch(() => settings.value?.birth_date, value => { if (!value && settings.value) settings.value.show_age = false })
watch([addressEditing, nextAddressChange], ([editing, availableAt], _, onCleanup) => {
  if (!editing || !Number.isFinite(availableAt)) return
  let timer
  function updateAvailability() {
    now.value = Date.now()
    const remaining = availableAt - now.value
    if (remaining > 0) timer = setTimeout(updateAvailability, Math.min(remaining, 2 ** 31 - 1))
  }
  updateAvailability()
  onCleanup(() => clearTimeout(timer))
})
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
async function toggleAddressEditor() {
  addressEditing.value = !addressEditing.value
  username.value = settings.value.username
  addressError.value = ''
  addressNotice.value = false
  if (addressEditing.value) {
    await nextTick()
    if (addressLocked.value) addressCooldown.value?.focus()
    else addressField.value?.focus()
  }
}
function openRename() {
  if (busy.value || addressLocked.value || newUsername.value === settings.value.username) return
  addressError.value = ''
  if (!/^[a-z0-9_]{3,30}$/.test(newUsername.value)) {
    addressError.value = 'profile.usernameInvalid'
    addressField.value?.focus()
    return
  }
  confirmRename.value = true
}
async function rename() {
  if (busy.value) return
  operation.value = 'rename'
  addressError.value = ''
  try {
    const updated = await changeUsername(newUsername.value)
    settings.value.username = updated.username
    settings.value.username_changed_at = updated.username_changed_at
    username.value = updated.username
    confirmRename.value = false
    addressEditing.value = false
    await refresh()
    addressNotice.value = true
  } catch (failure) {
    addressError.value = { username_taken: 'profile.usernameTaken', username_cooldown: 'profile.usernameCooldown', username_invalid: 'profile.usernameInvalid' }[failure.code] || 'profile.saveError'
    confirmRename.value = false
  } finally {
    operation.value = ''
    await nextTick()
    if (addressEditing.value) addressField.value?.focus()
    else addressToggle.value?.$el.focus({ preventScroll: true })
  }
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
  const choice = choosePhoto.value?.$el
  const restoreFocus = document.activeElement === choice
  operation.value = 'uploadAvatar'
  try {
    await uploadAvatar(file)
    settings.value.has_avatar = true
    avatarVersion.value = Date.now()
    avatarNotice.value = 'profile.avatarSaved'
  } catch { avatarError.value = 'profile.avatarInvalid' }
  finally {
    operation.value = ''
    await nextTick()
    if (restoreFocus && document.activeElement === document.body) choice.focus({ preventScroll: true })
  }
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
          <input ref="picker" type="file" accept="image/jpeg,image/png,image/webp" hidden :disabled="busy" @change="selectAvatar" />
          <UiButton ref="choosePhoto" variant="secondary" :loading="operation === 'uploadAvatar'" :disabled="busy" aria-describedby="profile-avatar-hint" @click="picker.click()">{{ t('profile.chooseAvatar') }}</UiButton>
          <UiButton v-if="settings.has_avatar" variant="ghost" :loading="operation === 'removeAvatar'" :disabled="busy" @click="removeAvatar">{{ t('profile.removeAvatar') }}</UiButton>
        </div>
        <p id="profile-avatar-hint" class="profile-hint">{{ t('profile.avatarHint') }}</p>
        <p v-if="avatarError" class="profile-error" role="alert">{{ t(avatarError) }}</p>
        <p v-if="avatarNotice" class="profile-success" role="status">{{ t(avatarNotice) }}</p>
      </div>
      <form id="profile-details-form" class="profile-panel" @submit.prevent="saveProfile" @input="saveError = ''">
        <TextField id="profile-display-name" ref="nameField" v-model="displayName" :label="t('profile.displayName')" :hint="t('profile.nameHint')" aria-required="true" maxlength="200" :disabled="busy || savedName === null" />
        <p v-if="nameLoadFailed" class="profile-error" role="alert">{{ t('profile.nameError') }}</p>
        <template v-if="settings">
          <TextField id="profile-biography" ref="biographyField" v-model="settings.biography" multiline rows="3" maxlength="4000" :label="t('profile.biography')" :hint="t('profile.biographyHint')" :disabled="busy" />
          <TextField id="profile-birth-date" v-model="settings.birth_date" type="date" :label="t('profile.birthDate')" :hint="t('profile.birthDateHint')" :disabled="busy" />
        </template>
        <section class="profile-visibility" aria-labelledby="profile-visibility-title">
          <h3 id="profile-visibility-title">{{ t('profile.visibility') }}</h3>
          <template v-if="settings">
            <label class="visibility-row">
              <input v-model="settings.show_age" type="checkbox" :disabled="busy || !settings.birth_date" :aria-describedby="settings.birth_date ? undefined : 'profile-age-hint'" />
              {{ t('profile.showAge') }}
            </label>
            <p v-if="!settings.birth_date" id="profile-age-hint" class="profile-hint">{{ t('profile.ageHint') }}</p>
            <label class="visibility-row"><input v-model="settings.show_followers" type="checkbox" :disabled="busy" />{{ t('profile.showFollowers') }}</label>
            <label class="visibility-row"><input v-model="settings.show_following" type="checkbox" :disabled="busy" />{{ t('profile.showFollowing') }}</label>
          </template>
          <slot name="collection" />
        </section>
      </form>
      <section v-if="settings" class="profile-address" aria-labelledby="profile-address-title">
        <div class="profile-address-row">
          <div class="profile-address-current">
            <h3 id="profile-address-title">{{ t('profile.address') }}</h3>
            <p class="profile-hint">{{ profileOrigin }}{{ profilePath }}</p>
          </div>
          <div class="profile-actions">
            <UiButton variant="ghost" :href="profilePath" :disabled="busy">{{ t('profile.openPublicProfile') }}</UiButton>
            <UiButton ref="addressToggle" variant="secondary" :disabled="busy" :aria-expanded="addressEditing" aria-controls="profile-address-editor" @click="toggleAddressEditor">
              {{ t(addressEditing ? 'profile.cancelAddressEdit' : 'profile.editAddress') }}
            </UiButton>
          </div>
        </div>
        <form v-show="addressEditing" id="profile-address-editor" class="profile-address-editor" @submit.prevent="openRename">
          <TextField id="profile-username" ref="addressField" v-model="username" :label="t('profile.newUsername')" maxlength="30" autocapitalize="none" spellcheck="false" :hint="addressFieldError ? '' : t('profile.usernameInvalid')" :error="addressFieldError ? t(addressFieldError) : ''" :disabled="busy || addressLocked" @update:model-value="addressError = ''" />
          <p class="profile-hint">{{ t('profile.usernameHint') }} {{ t('profile.redirectHint') }}</p>
          <p v-if="addressLocked" ref="addressCooldown" class="profile-hint" tabindex="-1">{{ t('profile.usernameAvailableAt') }} <time :datetime="new Date(nextAddressChange).toISOString()">{{ addressAvailableAt }}</time>.</p>
          <p v-if="addressError && !addressFieldError" class="profile-error" role="alert">{{ t(addressError) }}</p>
          <UiButton type="submit" variant="secondary" :disabled="busy || addressLocked || newUsername === settings.username">{{ t('profile.changeUsername') }}</UiButton>
        </form>
        <p v-if="addressNotice" class="profile-success" role="status">{{ t('profile.usernameSaved') }}</p>
      </section>
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
      <p>{{ t('profile.renameConfirm') }} <strong>/u/{{ newUsername }}</strong></p>
      <p>{{ t('profile.usernameHint') }}</p>
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
.profile-photo, .profile-address, .profile-address-editor, .profile-panel { display: grid; gap: var(--wx-space-3); min-width: 0; }
.profile-panel, .profile-address { padding-bottom: var(--wx-space-4); border-bottom: 1px solid var(--wx-border); }
.profile-address-row { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: var(--wx-space-2); }
.profile-address-current { flex: 1 1 14rem; min-width: 0; overflow-wrap: anywhere; }
.profile-address-editor > .button { justify-self: start; }
.profile-address-row .button { min-height: 44px; padding: var(--wx-space-2) var(--wx-space-3); }
.profile-panel :deep(textarea) { min-height: 0; }
.profile-hint, .profile-error, .profile-success { margin: 0; font-size: var(--wx-text-sm); line-height: 1.5; }
.profile-hint { color: var(--wx-text-muted); }
.profile-visibility h3, .profile-address h3 { margin: 0 0 var(--wx-space-1); font-size: var(--wx-text-md); }
.visibility-row { display: flex; align-items: center; gap: var(--wx-space-3); min-height: 44px; font-size: var(--wx-text-sm); cursor: pointer; }
.visibility-row input { flex: 0 0 auto; width: 20px; height: 20px; margin: 0; accent-color: var(--wx-primary); }
.visibility-row:has(input:disabled) { color: var(--wx-text-muted); cursor: default; }
.profile-visibility > .profile-hint { margin: 0 0 var(--wx-space-2) var(--wx-space-8); }
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
