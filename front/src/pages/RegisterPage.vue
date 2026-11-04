<script setup>
import { computed, nextTick, ref, watchEffect } from 'vue'
import { locale, t } from '../i18n.js'
import { register, verifyEmail, resendCode } from '../api/auth.js'
import { useAuthForm } from '../composables/useAuthForm.js'
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
  if (usernameError.value) return usernameField.value.focus()
  if (emailError.value) return emailField.value.focus()
  if (passwordError.value) return passwordField.value.focus()
  await run(async () => {
    const result = await register({ username: username.value, email: email.value, password: password.value, locale: locale.value })
    challenge.value = result.challenge
    password.value = ''
    cooldown(result.resend_after)
    await nextTick()
    document.getElementById('code')?.focus()
  })
}

function verify(code) {
  return run(async () => { await verifyEmail(challenge.value, code); window.location.assign('/my') })
}
function resend() {
  return run(async () => { const result = await resendCode(challenge.value); cooldown(result.resend_after) })
}
watchEffect(() => { document.title = `${t(challenge.value ? 'auth.verifyTitle' : 'register.title')} · ${t('appName')}` })
</script>

<template>
  <AuthLayout :title="t(challenge ? 'auth.verifyTitle' : 'register.title')" :description="t(challenge ? 'auth.verifyDescription' : 'register.description')" title-id="register-title" :divider-label="t('register.orEmail')" :show-social="false">
    <CodeVerification v-if="challenge" :busy="busy" :remaining="remaining" @verify="verify" @resend="resend" />
    <form v-else ref="form" class="register-form" novalidate :aria-busy="busy" @submit.prevent="submit">
      <TextField id="username" ref="usernameField" v-model="username" name="username" autocomplete="username" autocapitalize="none" :spellcheck="false" required maxlength="30" :label="t('register.username')" :placeholder="t('register.usernamePlaceholder')" :hint="t('register.usernameHint')" :error="usernameError" />
      <TextField id="email" ref="emailField" v-model="email" name="email" type="email" autocomplete="email" inputmode="email" autocapitalize="none" :spellcheck="false" required maxlength="254" :label="t('login.email')" :placeholder="t('login.emailPlaceholder')" :error="emailError" />
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
.verification-note { display: flex; align-items: flex-start; gap: var(--wx-space-3); margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.6; }
.verification-note .icon { color: var(--wx-primary); }
</style>
