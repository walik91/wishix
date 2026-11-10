<script setup>
import { computed, nextTick, ref, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { login } from '../api/auth.js'
import { useAuthForm } from '../composables/useAuthForm.js'
import { consumeReservationReturn } from '../reservationReturn.js'
import AuthLayout from '../layouts/AuthLayout.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import UiButton from '../components/ui/UiButton.vue'
import TextField from '../components/ui/TextField.vue'
import PasswordField from '../components/auth/PasswordField.vue'

const email = ref('')
const password = ref('')
const submitted = ref(false)
const emailField = ref(null)
const passwordField = ref(null)
const form = ref(null)
const passwordReset = new URLSearchParams(window.location.search).has('reset')
const { busy, error, run } = useAuthForm()

const emailError = computed(() => {
  if (!submitted.value) return ''
  if (!email.value.trim()) return t('login.emailRequired')
  return form.value?.elements.email.validity.typeMismatch ? t('login.emailInvalid') : ''
})
const passwordError = computed(() => submitted.value && !password.value ? t('login.passwordRequired') : '')

async function submit() {
  if (busy.value) return
  submitted.value = true
  await nextTick()
  if (emailError.value) return emailField.value.focus()
  if (passwordError.value) return passwordField.value.focus()
  await run(async () => {
    await login({ email: email.value, password: password.value })
    submitted.value = false
    password.value = ''
    window.location.assign(consumeReservationReturn())
  })
}
watchEffect(() => { document.title = `${t('login.submit')} · ${t('appName')}` })
</script>

<template>
  <AuthLayout :title="t('login.title')" :description="t('login.description')" title-id="login-title" :divider-label="t('login.orEmail')" :show-social="false">
    <p v-if="passwordReset" class="auth-notice" role="status">{{ t('auth.resetSuccess') }}</p>
    <form ref="form" class="login-form" novalidate :aria-busy="busy" @submit.prevent="submit">
      <TextField id="email" ref="emailField" v-model="email" name="email" type="email" autocomplete="username" inputmode="email" autocapitalize="none" :spellcheck="false" required maxlength="254" :label="t('login.email')" :placeholder="t('login.emailPlaceholder')" :error="emailError" />
      <div class="password-field">
        <PasswordField ref="passwordField" v-model="password" :error="passwordError" />
        <a class="forgot-password" href="/forgot-password">{{ t('login.forgotPassword') }}</a>
      </div>
      <UiButton type="submit" class="submit-button" :loading="busy">{{ t('login.submit') }}<AppIcon name="arrow" /></UiButton>
    </form>
    <p v-if="error" class="auth-error" role="alert">{{ error }}</p>
    <template #switch>
      <span>{{ t('login.noAccount') }}</span>
      <a href="/register">{{ t('login.createAccount') }}</a>
    </template>
  </AuthLayout>
</template>

<style scoped>
.login-form { display: grid; gap: var(--wx-space-5); }
.password-field { display: grid; }
.forgot-password { display: inline-flex; align-items: center; justify-self: end; min-height: 44px; margin-top: var(--wx-space-1); padding: var(--wx-space-2) 0; border-radius: var(--wx-space-1); color: var(--wx-primary); font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; text-align: end; text-decoration: none; }
.forgot-password:hover { color: var(--wx-primary-hover); text-decoration: underline; text-underline-offset: 3px; }
.forgot-password:active { color: var(--wx-primary-pressed); }
.submit-button { width: 100%; margin-top: calc(-1 * var(--wx-space-2)); }
</style>
