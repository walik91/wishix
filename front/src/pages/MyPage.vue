<script setup>
import { watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { useLists } from '../composables/useLists.js'
import { useWishes } from '../composables/useWishes.js'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import ListCard from '../components/wishes/ListCard.vue'
import gift from '../assets/brand/gift-hero.svg'

const { user, checking, unavailable, refresh } = useSession()
const { lists, loading: listsLoading, error: listsError, load: loadLists } = useLists()
const { wishes, loading: wishesLoading, error: wishesError, load: loadWishes } = useWishes()
watch([user, checking, unavailable], ([account, pending, failed]) => {
  if (account && !pending && !failed) { loadLists(); loadWishes() }
}, { immediate: true })
watchEffect(() => { document.title = `${t('auth.my')} · ${t('appName')}` })
async function retry() {
  await refresh()
  if (!user.value && !unavailable.value) window.location.replace('/login')
}
</script>

<template>
  <div class="dashboard" :aria-busy="checking || (user && (listsLoading || wishesLoading))">
    <div v-if="checking" class="dashboard-loading" role="status" :aria-label="t('auth.checkingSession')"><UiSkeleton class="loading-title" /><UiSkeleton class="loading-hero" /><UiSkeleton class="loading-hero" /></div>
    <UiCard v-else-if="unavailable" class="dashboard-error"><EmptyState icon="globe" :title="t('my.connectionTitle')" :description="t('auth.error.unavailable')"><UiButton @click="retry">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <template v-else-if="user">
      <header class="greeting"><h1>{{ t('my.hello') }} {{ user.username }}!</h1><p>{{ t('my.description') }}</p></header>
      <section v-if="!wishesLoading && !wishesError && (!wishes.length || (!listsLoading && !listsError && !lists.length))" class="hero" aria-labelledby="dashboard-hero-title">
        <div class="hero-copy"><h2 id="dashboard-hero-title">{{ t('my.heroTitle') }}</h2><p>{{ t('my.heroDescription') }}</p><UiButton :href="wishes.length ? '/my/lists/new' : '/my/wishes/new'"><AppIcon name="plus" />{{ t(wishes.length ? 'my.createList' : 'my.addWish') }}</UiButton></div>
        <img :src="gift" alt="" width="300" height="240" aria-hidden="true" />
      </section>
      <section class="dashboard-section" aria-labelledby="dashboard-lists-title">
        <div class="section-heading"><div><h2 id="dashboard-lists-title">{{ t('my.lists') }}</h2><p>{{ t('dashboard.listsDescription') }}</p></div><a href="/my/lists">{{ t('dashboard.allLists') }} <AppIcon name="arrow" /></a></div>
        <div v-if="listsLoading" class="list-grid" role="status" :aria-label="t('lists.loading')"><UiSkeleton v-for="i in 3" :key="i" class="list-skeleton" /></div>
        <UiCard v-else-if="listsError" class="dashboard-error"><EmptyState icon="globe" :title="t('lists.loadError')" :description="t('lists.error.unavailable')"><UiButton @click="loadLists">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
        <UiCard v-else-if="!lists.length" class="list-empty"><EmptyState icon="list" :title="t('lists.emptyTitle')" :description="t('lists.emptyDescription')"><UiButton href="/my/lists/new"><AppIcon name="plus" />{{ t('lists.create') }}</UiButton></EmptyState></UiCard>
        <div v-else class="list-grid"><ListCard v-for="list in lists.slice(0, 2)" :key="list.id" :list="list" /><a class="create-list" href="/my/lists/new"><span><AppIcon name="plus" /></span>{{ t('dashboard.createListTile') }}</a></div>
      </section>
      <section class="dashboard-section" aria-labelledby="dashboard-wishes-title">
        <div class="section-heading"><div><h2 id="dashboard-wishes-title">{{ t('my.allWishes') }}</h2><p>{{ t('my.wishesDescription') }}</p></div><a href="/my/wishes">{{ t('dashboard.allWishes') }} <AppIcon name="arrow" /></a></div>
        <div v-if="wishesLoading" class="wish-preview-grid" role="status" :aria-label="t('wishes.loading')"><UiSkeleton v-for="i in 3" :key="i" class="wish-skeleton" /></div>
        <UiCard v-else-if="wishesError" class="dashboard-error"><EmptyState icon="globe" :title="t('wishes.loadErrorTitle')" :description="t('wishes.loadError')"><UiButton @click="loadWishes">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
        <UiCard v-else-if="!wishes.length" class="wishes-empty"><EmptyState :title="t('my.emptyWishesTitle')" :description="t('wishes.emptyDescription')"><UiButton href="/my/wishes/new"><AppIcon name="plus" />{{ t('my.addWish') }}</UiButton></EmptyState></UiCard>
        <div v-else class="wish-preview-grid" :aria-label="t('dashboard.wishesPreview')"><a v-for="wish in wishes.slice(0, 3)" :key="wish.id" class="wish-preview" href="/my/wishes"><span class="wish-image"><img v-if="wish.image_url" :src="wish.image_url" alt="" loading="lazy" /><AppIcon v-else name="heart" /></span><span>{{ wish.title }}</span><AppIcon name="arrow" /></a></div>
      </section>
      <p class="privacy-note"><AppIcon name="lock" />{{ t('my.privacyNote') }}</p>
    </template>
  </div>
</template>

<style scoped>
.dashboard { min-width: 0; overflow-wrap: anywhere; }
.greeting h1 { margin: 0; font-size: 26px; line-height: 1.25; letter-spacing: -.03em; }
.greeting p, .section-heading p { margin: var(--wx-space-2) 0 0; color: var(--wx-text-muted); line-height: 1.6; }
.hero { position: relative; display: flex; align-items: center; min-height: 220px; margin-top: var(--wx-space-6); padding: var(--wx-space-6); overflow: hidden; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); background: linear-gradient(110deg, var(--wx-cream), var(--wx-lavender)); }
.hero-copy { position: relative; z-index: 1; min-width: 0; }
.hero h2 { max-width: 420px; margin: 0; font-size: 24px; line-height: 1.2; letter-spacing: -.03em; }
.hero p { max-width: 490px; margin: var(--wx-space-3) 0 var(--wx-space-5); color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.6; }
.hero img { display: none; }
.dashboard-section { margin-top: var(--wx-space-8); }
.section-heading { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: flex-start; gap: var(--wx-space-2) var(--wx-space-4); margin-bottom: var(--wx-space-4); }
.section-heading h2 { margin: 0; font-size: 22px; line-height: 1.3; letter-spacing: -.02em; }
.section-heading p { font-size: var(--wx-text-sm); }
.section-heading a { display: inline-flex; align-items: center; gap: var(--wx-space-1); min-height: 44px; color: var(--wx-primary-pressed); font-size: var(--wx-text-sm); font-weight: 600; text-decoration: none; }
.section-heading a:hover { text-decoration: underline; }.section-heading a .icon { width: 16px; height: 16px; }
.create-list { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: var(--wx-space-3); min-height: 144px; padding: var(--wx-space-5); border: 1px dashed var(--wx-secondary); border-radius: var(--wx-radius-card); color: var(--wx-primary-pressed); background: var(--wx-background); text-align: center; text-decoration: none; font-size: var(--wx-text-sm); font-weight: 600; }
.create-list:hover { background: var(--wx-lavender); }.create-list span { display: grid; place-items: center; width: 44px; height: 44px; border-radius: 50%; background: var(--wx-lavender); }
.wishes-empty, .dashboard-error { padding: var(--wx-space-8) var(--wx-space-5); }
.wish-preview-grid { display: grid; gap: var(--wx-space-4); }.wish-preview { display: flex; align-items: center; gap: var(--wx-space-3); min-width: 0; min-height: 90px; padding: var(--wx-space-3); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); background: var(--wx-surface); color: var(--wx-text); text-decoration: none; font-size: var(--wx-text-sm); font-weight: 600; box-shadow: var(--wx-shadow-card); }.wish-preview:hover { border-color: var(--wx-secondary); }.wish-preview > span:nth-child(2) { flex: 1; min-width: 0; }.wish-preview > .icon { color: var(--wx-primary); }.wish-image { display: grid; place-items: center; flex: 0 0 58px; height: 58px; overflow: hidden; border-radius: var(--wx-radius-sm); color: var(--wx-primary); background: var(--wx-lavender); }.wish-image img { width: 100%; height: 100%; object-fit: contain; background: var(--wx-surface); }
.privacy-note { display: flex; align-items: center; justify-content: center; gap: var(--wx-space-2); margin: var(--wx-space-8) 0 0; color: var(--wx-text-muted); text-align: center; font-size: var(--wx-text-xs); }.privacy-note .icon { width: 16px; height: 16px; }
.dashboard-loading { display: grid; gap: var(--wx-space-5); }.loading-title { width: 60%; height: 32px; }.loading-hero { height: 220px; border-radius: var(--wx-radius-card); }.wish-skeleton { height: 90px; border-radius: var(--wx-radius-card); }
@media (min-width: 48rem) { .greeting h1 { font-size: var(--wx-text-2xl); }.hero { min-height: 250px; padding: var(--wx-space-8); }.hero-copy { max-width: 60%; }.hero img { display: block; position: absolute; right: var(--wx-space-5); bottom: 0; width: 32%; max-width: 300px; height: auto; }.wish-preview-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (min-width: 75rem) { .wish-preview-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
