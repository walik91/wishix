<script setup>
import { computed, nextTick, ref, watchEffect } from 'vue'
import { t } from '../i18n.js'
import AuthLayout from '../layouts/AuthLayout.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import UiButton from '../components/ui/UiButton.vue'
import TextField from '../components/ui/TextField.vue'
import PasswordField from '../components/auth/PasswordField.vue'

const email = ref('')
const password = ref('')
const submitted = ref(false)
const notice = ref('')
const emailField = ref(null)
const passwordField = ref(null)
const form = ref(null)

const emailError = computed(() => {
  if (!submitted.value) return ''
  if (!email.value.trim()) return t('login.emailRequired')
  return form.value?.elements.email.validity.typeMismatch ? t('login.emailInvalid') : ''
})
const passwordError = computed(() => submitted.value && !password.value ? t('login.passwordRequired') : '')

async function submit() {
  notice.value = ''
  submitted.value = true
  await nextTick()
  if (emailError.value) emailField.value.focus()
  else if (passwordError.value) passwordField.value.focus()
  else notice.value = 'login.previewNotice'
}

watchEffect(() => {
  document.title = `${t('login.submit')} · ${t('appName')}`
})
</script>

<template>
  <AuthLayout :title="t('login.title')" :description="t('login.description')" title-id="login-title" :divider-label="t('login.orEmail')" :notice="notice ? t(notice) : ''" @social="notice = `login.${$event}Notice`">
    <form ref="form" class="login-form" novalidate @submit.prevent="submit" @input="notice = ''">
      <TextField id="email" ref="emailField" v-model="email" name="email" type="email" autocomplete="username" inputmode="email" autocapitalize="none" :spellcheck="false" required :label="t('login.email')" :placeholder="t('login.emailPlaceholder')" :error="emailError" />
      <div class="password-field">
        <PasswordField ref="passwordField" v-model="password" :error="passwordError" />
        <button type="button" class="forgot-password" @click="notice = 'login.recoveryNotice'">{{ t('login.forgotPassword') }}</button>
      </div>
      <UiButton type="submit" class="submit-button">{{ t('login.submit') }}<AppIcon name="arrow" /></UiButton>
    </form>
    <template #switch>
      <span>{{ t('login.noAccount') }}</span>
      <a href="/register">{{ t('login.createAccount') }}</a>
    </template>
  </AuthLayout>
</template>

<style scoped>
.login-form { display: grid; gap: var(--wx-space-5); }
.password-field { display: grid; }
.forgot-password { justify-self: end; min-height: 44px; margin-top: var(--wx-space-1); padding: var(--wx-space-2) 0; border: 0; border-radius: var(--wx-space-1); color: var(--wx-primary); background: transparent; font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; text-align: end; cursor: pointer; }
.forgot-password:hover { color: var(--wx-primary-hover); text-decoration: underline; text-underline-offset: 3px; }
.forgot-password:active { color: var(--wx-primary-pressed); }
.submit-button { width: 100%; margin-top: calc(-1 * var(--wx-space-2)); }
</style>
