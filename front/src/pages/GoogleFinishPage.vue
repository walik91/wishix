<script setup>
import { computed, nextTick, onMounted, ref, watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { completeGoogle, completeFacebook, facebookStatus, googleUsernameSuggestions } from '../api/auth.js'
import { useAuthForm } from '../composables/useAuthForm.js'
import { consumeReservationReturn } from '../reservationReturn.js'
import AuthLayout from '../layouts/AuthLayout.vue'
import UiButton from '../components/ui/UiButton.vue'
import TextField from '../components/ui/TextField.vue'

const username = ref('')
const facebook = window.location.pathname.startsWith('/facebook/')
const ready = ref(!facebook)
const suggestions = ref([])
const usernameTouched = ref(false)
onMounted(async () => {
  if (!facebook) {
    try {
      const result = await googleUsernameSuggestions()
      suggestions.value = result.suggestions
      if (!usernameTouched.value && result.suggestions.length) username.value = result.suggestions[0]
    } catch { /* Manual entry remains available. */ }
    return
  }
  try { if ((await facebookStatus()).enabled) { ready.value = true; return } } catch { /* Return to sign-in if availability cannot be checked. */ }
  window.location.replace('/login')
})
const field = ref(null)
const submitted = ref(false)
const invalid = computed(() => submitted.value && !/^[a-z0-9_]{3,30}$/i.test(username.value.trim()))
const { busy, error, run } = useAuthForm()
function chooseUsername(value) { usernameTouched.value = true; username.value = value }
watch(username, () => { error.value = '' })
async function submit() {
  submitted.value = true
  if (invalid.value) { await nextTick(); field.value.focus(); return }
  await run(async () => { await (facebook ? completeFacebook(username.value) : completeGoogle(username.value)); window.location.assign(consumeReservationReturn()) })
}
watchEffect(() => { document.title = `${t('google.finishTitle')} · ${t('appName')}` })
</script>

<template>
  <AuthLayout v-if="ready" :title="t('google.finishTitle')" :description="t('google.finishDescription')" title-id="google-finish-title" :divider-label="''" :show-social="false">
    <form class="finish-form" novalidate :aria-busy="busy" @submit.prevent="submit">
      <TextField id="username" ref="field" v-model="username" name="username" autocomplete="username" autocapitalize="none" :spellcheck="false" required maxlength="30" :label="t('register.username')" :placeholder="t('register.usernamePlaceholder')" :hint="t('register.usernameHint')" :error="invalid ? t('auth.error.username_invalid') : ''" @input="usernameTouched = true" />
      <p class="profile-url">{{ t('register.profileUrl') }} <bdi>/u/{{ username.trim().toLowerCase() || t('register.usernamePlaceholder') }}</bdi></p>
      <div v-if="suggestions.length" class="suggestions">
        <p>{{ t('register.suggestions') }}</p>
        <div class="suggestion-list"><button v-for="suggestion in suggestions" :key="suggestion" type="button" :aria-pressed="username === suggestion" @click="chooseUsername(suggestion)"><span>{{ suggestion }}</span></button></div>
      </div>
      <UiButton type="submit" :loading="busy">{{ t('google.finishButton') }}</UiButton>
    </form>
    <p v-if="error" class="auth-error" role="alert">{{ error }}</p>
    <template #switch><a href="/login">{{ t('auth.backToLogin') }}</a></template>
  </AuthLayout>
</template>

<style scoped>
.finish-form { display: grid; gap: var(--wx-space-5); min-width: 0; }
.profile-url, .suggestions p { margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.5; overflow-wrap: anywhere; }
.profile-url { margin-top: calc(-1 * var(--wx-space-3)); }
.suggestions p { margin-bottom: var(--wx-space-1); }
.suggestion-list { display: flex; flex-wrap: wrap; gap: var(--wx-space-2); }
.suggestion-list button { display: grid; align-items: center; min-width: 44px; min-height: 44px; max-width: 100%; padding: 0; border: 0; background: transparent; color: var(--wx-primary); font: inherit; font-size: var(--wx-text-sm); cursor: pointer; }
.suggestion-list button span { display: block; min-width: 0; padding: var(--wx-space-1) var(--wx-space-2); overflow-wrap: anywhere; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); background: var(--wx-surface); line-height: 1.5; }
.suggestion-list button:hover span, .suggestion-list button[aria-pressed="true"] span { border-color: var(--wx-primary); }
.suggestion-list button:focus-visible { outline: none; }
.suggestion-list button:focus-visible span { box-shadow: var(--wx-focus-ring); }
</style>
