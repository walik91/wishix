<script setup>
import { computed, nextTick, onMounted, ref, watch, watchEffect } from 'vue'
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
const submitted = ref(false)
const invalid = computed(() => submitted.value && !/^[a-z0-9_]{3,30}$/i.test(username.value.trim()))
const { busy, error, run } = useAuthForm()
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
      <TextField id="username" ref="field" v-model="username" name="username" autocomplete="username" autocapitalize="none" :spellcheck="false" required maxlength="30" :label="t('register.username')" :placeholder="t('register.usernamePlaceholder')" :hint="t('register.usernameHint')" :error="invalid ? t('auth.error.username_invalid') : ''" />
      <p class="profile-url">{{ t('register.profileUrl') }} <bdi>/u/{{ username.trim().toLowerCase() || t('register.usernamePlaceholder') }}</bdi></p>
      <UiButton type="submit" :loading="busy">{{ t('google.finishButton') }}</UiButton>
    </form>
    <p v-if="error" class="auth-error" role="alert">{{ error }}</p>
    <template #switch><a href="/login">{{ t('auth.backToLogin') }}</a></template>
  </AuthLayout>
</template>

<style scoped>.finish-form { display: grid; gap: var(--wx-space-5); }.profile-url { margin: calc(-1 * var(--wx-space-3)) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); overflow-wrap: anywhere; }</style>
