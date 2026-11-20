<script setup>
import { computed, nextTick, onMounted, ref, watch, watchEffect } from 'vue'
import { locale, t } from '../i18n.js'
import { trackEvent } from '../analytics.js'
import { register, usernameSuggestions, verifyEmail, resendCode, googleStatus, startGoogle, facebookStatus, startFacebook } from '../api/auth.js'
import { useAuthForm } from '../composables/useAuthForm.js'
import { consumeReservationReturn } from '../reservationReturn.js'
import AuthLayout from '../layouts/AuthLayout.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import UiButton from '../components/ui/UiButton.vue'
import TextField from '../components/ui/TextField.vue'
import PasswordField from '../components/auth/PasswordField.vue'
import CodeVerification from '../components/auth/CodeVerification.vue'

const username = ref('')
const email = ref('')
const password = ref('')
const challenge = ref('')
const submitted = ref(false)
const form = ref(null)
const usernameField = ref(null)
const emailField = ref(null)
const passwordField = ref(null)
const { busy, error, remaining, cooldown, run } = useAuthForm()
const googleEnabled = ref(false)
const facebookEnabled = ref(false)
const suggestions = ref([])
const usernameTouched = ref(false)
let suggestionRequest = 0
watch([username, email, password], () => { error.value = '' })
watch(email, (value, _, onCleanup) => {
  const request = ++suggestionRequest
  suggestions.value = []
  if (!usernameTouched.value) username.value = ''
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim())) return
  let base = value.trim().split('@')[0].toLowerCase().replace(/[^a-z0-9_]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 30)
  if (base.length < 3) base = (base + 'user').slice(0, 30)
  const timer = setTimeout(async () => {
    try {
      const result = await usernameSuggestions(base)
      if (request !== suggestionRequest) return
      suggestions.value = result.suggestions
      if (!usernameTouched.value && result.suggestions.length) username.value = result.suggestions[0]
    } catch { /* Manual entry remains available if suggestions cannot be loaded. */ }
  }, 600)
  onCleanup(() => clearTimeout(timer))
})
function chooseUsername(value) { usernameTouched.value = true; username.value = value }
onMounted(() => { googleStatus().then(s => { googleEnabled.value = s.enabled }).catch(() => {}); facebookStatus().then(s => { facebookEnabled.value = s.enabled }).catch(() => {}) })
function socialSignIn(provider) { return run(async () => { const result = await (provider === 'google' ? startGoogle('login') : startFacebook('login')); window.location.assign(result.url) }) }

const usernameError = computed(() => {
  if (!submitted.value) return ''
  if (!username.value.trim()) return t('register.usernameRequired')
  return /^[a-z0-9_]{3,30}$/i.test(username.value.trim()) ? '' : t('auth.error.username_invalid')
})
const emailError = computed(() => {
  if (!submitted.value) return ''
  if (!email.value.trim()) return t('login.emailRequired')
  return form.value?.elements.email.validity.typeMismatch ? t('login.emailInvalid') : ''
})
const passwordError = computed(() => submitted.value && (Array.from(password.value).length < 4 || Array.from(password.value).length > 128) ? t('auth.error.password_invalid') : '')

async function submit() {
  if (busy.value) return
  submitted.value = true
  await nextTick()
  if (emailError.value) return emailField.value.focus()
  if (usernameError.value) return usernameField.value.focus()
  if (passwordError.value) return passwordField.value.focus()
  await run(async () => {
    const result = await register({ username: username.value, email: email.value, password: password.value, locale: locale.value })
    trackEvent('sign_up_started')
    challenge.value = result.challenge
    password.value = ''
    cooldown(result.resend_after)
    await nextTick()
    document.getElementById('code')?.focus()
  })
}

function verify(code) {
  return run(async () => { await verifyEmail(challenge.value, code); trackEvent('sign_up_completed'); window.location.assign(consumeReservationReturn()) })
}
function resend() {
  return run(async () => { const result = await resendCode(challenge.value); cooldown(result.resend_after) })
}
watchEffect(() => { document.title = `${t(challenge.value ? 'auth.verifyTitle' : 'register.title')} · ${t('appName')}` })
</script>

<template>
  <AuthLayout :title="t(challenge ? 'auth.verifyTitle' : 'register.title')" :description="t(challenge ? 'auth.verifyDescription' : 'register.description')" title-id="register-title" :divider-label="t('register.orEmail')" :show-social="(googleEnabled || facebookEnabled) && !challenge" :google="googleEnabled" :facebook="facebookEnabled" :social-busy="busy" @social="socialSignIn">
    <CodeVerification v-if="challenge" :busy="busy" :remaining="remaining" @verify="verify" @resend="resend" />
    <form v-else ref="form" class="register-form" novalidate :aria-busy="busy" @submit.prevent="submit">
      <TextField id="email" ref="emailField" v-model="email" name="email" type="email" autocomplete="email" inputmode="email" autocapitalize="none" :spellcheck="false" required maxlength="254" :label="t('login.email')" :placeholder="t('login.emailPlaceholder')" :error="emailError" />
      <div class="username-section">
        <TextField id="username" ref="usernameField" v-model="username" name="username" autocomplete="username" autocapitalize="none" :spellcheck="false" required maxlength="30" :label="t('register.username')" :placeholder="t('register.usernamePlaceholder')" :hint="t('register.usernameHint')" :error="usernameError" @input="usernameTouched = true" />
        <p class="profile-url">{{ t('register.profileUrl') }} <bdi>/u/{{ username.trim().toLowerCase() || t('register.usernamePlaceholder') }}</bdi></p>
        <div v-if="suggestions.length" class="suggestions">
          <p>{{ t('register.suggestions') }}</p>
          <div class="suggestion-list"><button v-for="suggestion in suggestions" :key="suggestion" type="button" :aria-pressed="username === suggestion" @click="chooseUsername(suggestion)">{{ suggestion }}</button></div>
        </div>
      </div>
      <PasswordField ref="passwordField" v-model="password" autocomplete="new-password" :placeholder="t('register.passwordPlaceholder')" :hint="t('auth.passwordHint')" :error="passwordError" />
      <p class="verification-note"><AppIcon name="mail" /><span>{{ t('register.verificationNote') }}</span></p>
      <UiButton type="submit" :loading="busy">{{ t('login.createAccount') }}<AppIcon name="arrow" /></UiButton>
    </form>
    <p v-if="error" class="auth-error" role="alert">{{ error }}</p>
    <template #switch>
      <span>{{ t('register.hasAccount') }}</span>
      <a href="/login">{{ t('login.submit') }}</a>
    </template>
  </AuthLayout>
</template>

<style scoped>
.register-form { display: grid; gap: var(--wx-space-5); }
.username-section { display: grid; gap: var(--wx-space-2); min-width: 0; }
.profile-url, .suggestions p { margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.5; overflow-wrap: anywhere; }
.suggestion-list { display: flex; flex-wrap: wrap; gap: var(--wx-space-2); }
.suggestion-list button { min-width: 44px; min-height: 44px; max-width: 100%; padding: var(--wx-space-2) var(--wx-space-3); overflow-wrap: anywhere; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); background: var(--wx-surface); color: var(--wx-primary); font: inherit; cursor: pointer; }
.suggestion-list button:hover, .suggestion-list button[aria-pressed="true"] { border-color: var(--wx-primary); }
.suggestion-list button:focus-visible { outline: none; box-shadow: var(--wx-focus-ring); }
.verification-note { display: flex; align-items: flex-start; gap: var(--wx-space-3); margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.6; }
.verification-note .icon { color: var(--wx-primary); }
</style>
