<script setup>
import { onMounted } from 'vue'
import LoginPage from './pages/LoginPage.vue'
import RegisterPage from './pages/RegisterPage.vue'
import GuestHomePage from './pages/GuestHomePage.vue'
import StartPage from './pages/StartPage.vue'
import UiButton from './components/ui/UiButton.vue'
import { t } from './i18n.js'
import PublicLayout from './layouts/PublicLayout.vue'
import RecoveryPage from './pages/RecoveryPage.vue'
import MyPage from './pages/MyPage.vue'
import { useSession } from './composables/useSession.js'

// Full document navigation keeps the small auth flow independent of a router.
const path = window.location.pathname.replace(/\/+$/, '') || '/'
const aliases = { '/sign-in': '/login', '/sign-up': '/register' }
const page = aliases[path] || path

if (aliases[path]) {
  window.history.replaceState(null, '', `${page}${window.location.search}${window.location.hash}`)
}
const { user, unavailable, initialize } = useSession()
onMounted(async () => {
  await initialize()
  if (user.value && (page === '/login' || page === '/register' || page === '/')) window.location.replace('/my')
  else if (!user.value && !unavailable.value && page === '/my') window.location.replace('/login')
})
</script>

<template>
  <PublicLayout>
    <GuestHomePage v-if="page === '/'" />
    <LoginPage v-else-if="page === '/login'" />
    <RegisterPage v-else-if="page === '/register'" />
    <RecoveryPage v-else-if="page === '/forgot-password'" />
    <MyPage v-else-if="page === '/my'" />
    <StartPage v-else-if="page === '/status'" />
    <section v-else class="not-found">
      <h1>{{ t('notFound.title') }}</h1>
      <p>{{ t('notFound.description') }}</p>
      <UiButton href="/">{{ t('notFound.home') }}</UiButton>
    </section>
  </PublicLayout>
</template>

<style scoped>
.not-found { width: min(100%, 600px); margin: 15svh auto; }
.not-found h1 { font-size: var(--wx-text-2xl); }
.not-found p { color: var(--wx-text-muted); line-height: 1.6; }
</style>

