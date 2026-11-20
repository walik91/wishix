<script setup>
import { onMounted, ref } from 'vue'
import LoginPage from './pages/LoginPage.vue'
import GoogleFinishPage from './pages/GoogleFinishPage.vue'
import RegisterPage from './pages/RegisterPage.vue'
import GuestHomePage from './pages/GuestHomePage.vue'
import StartPage from './pages/StartPage.vue'
import UiButton from './components/ui/UiButton.vue'
import CookieConsent from './components/ui/CookieConsent.vue'
import { startAnalytics } from './analytics.js'
import { t } from './i18n.js'
import PublicLayout from './layouts/PublicLayout.vue'
import AppShell from './layouts/AppShell.vue'
import RecoveryPage from './pages/RecoveryPage.vue'
import MyPage from './pages/MyPage.vue'
import WishesPage from './pages/WishesPage.vue'
import NewWishPage from './pages/NewWishPage.vue'
import EditWishPage from './pages/EditWishPage.vue'
import ListsPage from './pages/ListsPage.vue'
import NewListPage from './pages/NewListPage.vue'
import ListPage from './pages/ListPage.vue'
import SharedListPage from './pages/SharedListPage.vue'
import PublicProfilePage from './pages/PublicProfilePage.vue'
import PublicCollectionPage from './pages/PublicCollectionPage.vue'
import ReservationPage from './pages/ReservationPage.vue'
import SearchPage from './pages/SearchPage.vue'
import SocialPeoplePage from './pages/SocialPeoplePage.vue'
import { useSession } from './composables/useSession.js'
import { isNamedSharedListPath } from './sharedListPath.js'
import { consumeReservationReturn } from './reservationReturn.js'

// Full document navigation keeps the small auth flow independent of a router.
const path = window.location.pathname.replace(/\/+$/, '') || '/'
const aliases = { '/sign-in': '/login', '/sign-up': '/register' }
const page = aliases[path] || path
const listMatch = page.match(/^\/my\/lists\/([^/]+)$/)
const editWishMatch = page.match(/^\/my\/wishes\/([^/]+)\/edit$/)
const profileMatch = page.match(/^\/u\/([A-Za-z0-9_]{3,30})$/)
const collectionMatch = page.match(/^\/u\/([A-Za-z0-9_]{3,30})\/wishes$/)
const peopleMatch = page.match(/^\/u\/([A-Za-z0-9_]{3,30})\/(followers|following)$/)
const privatePage = page === '/my' || page === '/my/profile' || page === '/my/following' || page === '/my/search' || page === '/my/wishes' || page === '/my/wishes/new' || page === '/my/lists' || !!listMatch || !!editWishMatch

if (aliases[path]) {
  window.history.replaceState(null, '', `${page}${window.location.search}${window.location.hash}`)
}
const { user, unavailable, initialize } = useSession()
const cookieConsent = ref(null)
onMounted(async () => {
  startAnalytics()
  await initialize()
  if (user.value && page === '/login') window.location.replace(consumeReservationReturn())
  else if (user.value && page === '/register') window.location.replace(consumeReservationReturn())
  else if (!user.value && !unavailable.value && privatePage) window.location.replace('/login')
})
</script>

<template>
  <component :is="privatePage ? AppShell : PublicLayout" @cookie-settings="cookieConsent?.showSettings()">
    <GuestHomePage v-if="page === '/'" />
    <LoginPage v-else-if="page === '/login'" />
    <GoogleFinishPage v-else-if="page === '/google/finish' || page === '/facebook/finish'" />
    <RegisterPage v-else-if="page === '/register'" />
    <RecoveryPage v-else-if="page === '/forgot-password'" />
    <ReservationPage v-else-if="page === '/reservation'" />
    <MyPage v-else-if="page === '/my' || page === '/my/profile'" />
    <SearchPage v-else-if="page === '/my/search'" />
    <SocialPeoplePage v-else-if="page === '/my/following'" own />
    <WishesPage v-else-if="page === '/my/wishes'" />
    <NewWishPage v-else-if="page === '/my/wishes/new'" />
    <EditWishPage v-else-if="editWishMatch" :id="editWishMatch[1]" />
    <ListsPage v-else-if="page === '/my/lists'" />
    <NewListPage v-else-if="page === '/my/lists/new'" />
    <ListPage v-else-if="listMatch" :id="listMatch[1]" />
    <SharedListPage v-else-if="isNamedSharedListPath(page)" />
    <PublicCollectionPage v-else-if="collectionMatch" :username="collectionMatch[1]" />
    <SocialPeoplePage v-else-if="peopleMatch" :username="peopleMatch[1]" :initial-kind="peopleMatch[2]" />
    <PublicProfilePage v-else-if="profileMatch" :username="profileMatch[1]" />
    <StartPage v-else-if="page === '/status'" />
    <section v-else class="not-found">
      <h1>{{ t('notFound.title') }}</h1>
      <p>{{ t('notFound.description') }}</p>
      <UiButton href="/">{{ t('notFound.home') }}</UiButton>
    </section>
  </component>
  <CookieConsent ref="cookieConsent" />
</template>

<style scoped>
.not-found { width: min(100%, 600px); margin: 15svh auto; }
.not-found h1 { font-size: var(--wx-text-2xl); }
.not-found p { color: var(--wx-text-muted); line-height: 1.6; }
</style>

