<script setup>
import { nextTick, onMounted, ref, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { completeGoogle, completeFacebook, facebookStatus } from '../api/auth.js'
import { useAuthForm } from '../composables/useAuthForm.js'
import { consumeReservationReturn } from '../reservationReturn.js'
import AuthLayout from '../layouts/AuthLayout.vue'
import UiButton from '../components/ui/UiButton.vue'
import TextField from '../components/ui/TextField.vue'

const username = ref('')
const facebook = window.location.pathname.startsWith('/facebook/')
const ready = ref(!facebook)
onMounted(async () => {
  if (!facebook) return
  try { if ((await facebookStatus()).enabled) { ready.value = true; return } } catch { /* Return to sign-in if availability cannot be checked. */ }
  window.location.replace('/login')
})
const field = ref(null)
const invalid = ref(false)
const { busy, error, run } = useAuthForm()
async function submit() {
  invalid.value = !/^[a-z0-9_]{3,30}$/i.test(username.value.trim())
  if (invalid.value) { await nextTick(); field.value.focus(); return }
  await run(async () => { await (facebook ? completeFacebook(username.value) : completeGoogle(username.value)); window.location.assign(consumeReservationReturn()) })
}
watchEffect(() => { document.title = `${t('google.finishTitle')} · ${t('appName')}` })
</script>

<template>
  <AuthLayout v-if="ready" :title="t('google.finishTitle')" :description="t('google.finishDescription')" title-id="google-finish-title" :divider-label="''" :show-social="false">
    <form class="finish-form" novalidate :aria-busy="busy" @submit.prevent="submit">
      <TextField id="username" ref="field" v-model="username" name="username" autocomplete="username" autocapitalize="none" :spellcheck="false" required maxlength="30" :label="t('register.username')" :placeholder="t('register.usernamePlaceholder')" :hint="t('register.usernameHint')" :error="invalid ? t('auth.error.username_invalid') : ''" />
      <UiButton type="submit" :loading="busy">{{ t('google.finishButton') }}</UiButton>
    </form>
    <p v-if="error" class="auth-error" role="alert">{{ error }}</p>
    <template #switch><a href="/login">{{ t('auth.backToLogin') }}</a></template>
  </AuthLayout>
</template>

<style scoped>.finish-form { display: grid; gap: var(--wx-space-5); }</style>
