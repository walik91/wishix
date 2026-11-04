<script setup>
import { nextTick, ref, watchEffect } from 'vue'
import { locale, t } from '../i18n.js'
import { forgotPassword, resendCode, resetPassword } from '../api/auth.js'
import { useAuthForm } from '../composables/useAuthForm.js'
import AuthLayout from '../layouts/AuthLayout.vue'
import TextField from '../components/ui/TextField.vue'
import UiButton from '../components/ui/UiButton.vue'
import PasswordField from '../components/auth/PasswordField.vue'
import CodeVerification from '../components/auth/CodeVerification.vue'

const email = ref('')
const password = ref('')
const challenge = ref('')
const emailError = ref('')
const passwordError = ref('')
const form = ref(null)
const emailField = ref(null)
const passwordField = ref(null)
const { busy, error, remaining, cooldown, run } = useAuthForm()

async function requestCode() {
  if (busy.value) return
  emailError.value = !email.value.trim() ? t('login.emailRequired') : form.value.elements.email.validity.typeMismatch ? t('login.emailInvalid') : ''
  if (emailError.value) { await nextTick(); return emailField.value.focus() }
  await run(async () => {
    const result = await forgotPassword({ email: email.value, locale: locale.value })
    challenge.value = result.challenge
    cooldown(result.resend_after)
    await nextTick()
    document.getElementById('code')?.focus()
  })
}

async function reset(code) {
  passwordError.value = Array.from(password.value).length < 4 || Array.from(password.value).length > 128 ? t('auth.error.password_invalid') : ''
  if (passwordError.value) { await nextTick(); return passwordField.value.focus() }
  return run(async () => {
    await resetPassword({ challenge: challenge.value, code, password: password.value })
    password.value = ''
    window.location.assign('/login?reset=1')
  })
}

function resend() {
  return run(async () => { const result = await resendCode(challenge.value); cooldown(result.resend_after) })
}
watchEffect(() => { document.title = `${t('auth.recoveryTitle')} · ${t('appName')}` })
</script>

<template>
  <AuthLayout :title="t('auth.recoveryTitle')" :description="t(challenge ? 'auth.recoverySent' : 'auth.recoveryDescription')" title-id="recovery-title" :divider-label="t('login.orEmail')" :show-social="false">
    <CodeVerification v-if="challenge" :busy="busy" :remaining="remaining" @verify="reset" @resend="resend">
      <PasswordField ref="passwordField" v-model="password" autocomplete="new-password" :placeholder="t('register.passwordPlaceholder')" :hint="t('auth.passwordHint')" :error="passwordError" />
    </CodeVerification>
    <form v-else ref="form" class="recovery-form" novalidate :aria-busy="busy" @submit.prevent="requestCode">
      <TextField id="email" ref="emailField" v-model="email" name="email" type="email" autocomplete="email" inputmode="email" autocapitalize="none" :spellcheck="false" required maxlength="254" :label="t('login.email')" :placeholder="t('login.emailPlaceholder')" :error="emailError" />
      <UiButton type="submit" :loading="busy">{{ t('auth.sendCode') }}</UiButton>
    </form>
    <p v-if="error" class="auth-error" role="alert">{{ error }}</p>
    <template #switch><a href="/login">{{ t('auth.backToLogin') }}</a></template>
  </AuthLayout>
</template>

<style scoped>
.recovery-form { display: grid; gap: var(--wx-space-5); }
</style>
