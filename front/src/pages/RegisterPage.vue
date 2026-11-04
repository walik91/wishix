<script setup>
import { computed, nextTick, ref, watchEffect } from 'vue'
import { t } from '../i18n.js'
import AuthLayout from '../layouts/AuthLayout.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import UiButton from '../components/ui/UiButton.vue'
import TextField from '../components/ui/TextField.vue'
import PasswordField from '../components/auth/PasswordField.vue'

const username = ref('')
const email = ref('')
const password = ref('')
const submitted = ref(false)
const notice = ref('')
const form = ref(null)
const usernameField = ref(null)
const emailField = ref(null)
const passwordField = ref(null)

const usernameError = computed(() => submitted.value && !username.value.trim() ? t('register.usernameRequired') : '')
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
  if (usernameError.value) usernameField.value.focus()
  else if (emailError.value) emailField.value.focus()
  else if (passwordError.value) passwordField.value.focus()
  else notice.value = 'register.previewNotice'
}

watchEffect(() => {
  document.title = `${t('register.title')} · ${t('appName')}`
})
</script>

<template>
  <AuthLayout :title="t('register.title')" :description="t('register.description')" title-id="register-title" :divider-label="t('register.orEmail')" :notice="notice ? t(notice) : ''" @social="notice = `login.${$event}Notice`">
    <form ref="form" class="register-form" novalidate @submit.prevent="submit" @input="notice = ''">
      <TextField id="username" ref="usernameField" v-model="username" name="username" autocomplete="username" autocapitalize="none" :spellcheck="false" required :label="t('register.username')" :placeholder="t('register.usernamePlaceholder')" :hint="t('register.usernameHint')" :error="usernameError" />
      <TextField id="email" ref="emailField" v-model="email" name="email" type="email" autocomplete="email" inputmode="email" autocapitalize="none" :spellcheck="false" required :label="t('login.email')" :placeholder="t('login.emailPlaceholder')" :error="emailError" />
      <PasswordField ref="passwordField" v-model="password" autocomplete="new-password" :placeholder="t('register.passwordPlaceholder')" :error="passwordError" />
      <p class="verification-note"><AppIcon name="mail" /><span>{{ t('register.verificationNote') }}</span></p>
      <UiButton type="submit">{{ t('login.createAccount') }}<AppIcon name="arrow" /></UiButton>
    </form>
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
