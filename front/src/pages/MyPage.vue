<script setup>
import { watchEffect } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { useAuthForm } from '../composables/useAuthForm.js'
import UiButton from '../components/ui/UiButton.vue'

const { user, checking, unavailable, refresh, signOut } = useSession()
const { busy, error, run } = useAuthForm()
watchEffect(() => { document.title = `${t('auth.my')} · ${t('appName')}` })

async function retry() {
  await refresh()
  if (!user.value && !unavailable.value) window.location.replace('/login')
}
</script>

<template>
  <section class="account-card" :aria-busy="checking">
    <p v-if="checking" role="status">{{ t('auth.checkingSession') }}</p>
    <template v-else-if="unavailable">
      <p class="auth-error" role="alert">{{ t('auth.error.unavailable') }}</p>
      <UiButton @click="retry">{{ t('checkAgain') }}</UiButton>
    </template>
    <template v-else-if="user">
      <h1>{{ t('auth.my') }}</h1>
      <p class="welcome">{{ t('auth.welcome') }}, <strong>{{ user.username }}</strong>!</p>
      <dl><dt>{{ t('login.email') }}</dt><dd>{{ user.email }}</dd><dt>{{ t('register.username') }}</dt><dd>{{ user.username }}</dd></dl>
      <UiButton variant="secondary" :loading="busy" @click="run(signOut)">{{ t('auth.logout') }}</UiButton>
      <p v-if="error" class="auth-error" role="alert">{{ error }}</p>
    </template>
  </section>
</template>

<style scoped>
.account-card { width: min(100%, 640px); margin: var(--wx-space-8) auto; padding: var(--wx-space-6); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); background: var(--wx-surface); box-shadow: var(--wx-shadow-card); overflow-wrap: anywhere; }
h1 { margin: 0; font-size: var(--wx-text-xl); }
.welcome { font-size: var(--wx-text-lg); line-height: 1.5; }
dl { display: grid; gap: var(--wx-space-2); margin-block: var(--wx-space-6); }
dt { color: var(--wx-text-muted); font-size: var(--wx-text-sm); }
dd { margin: 0 0 var(--wx-space-3); }
</style>
