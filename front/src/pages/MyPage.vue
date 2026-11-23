<script setup>
import { computed, watch, watchEffect } from 'vue'
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
const emptyDashboard = computed(() =>
  !listsLoading.value && !listsError.value && !lists.value.length &&
  !wishesLoading.value && !wishesError.value && !wishes.value.length,
)
const sections = computed(() => lists.value.length ? ['lists', 'wishes'] : ['wishes', 'lists'])
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
    <div v-if="checking" class="dashboard-loading" role="status" :aria-label="t('auth.checkingSession')"><UiSkeleton class="loading-title" /><UiSkeleton class="wish-skeleton" /><UiSkeleton class="wish-skeleton" /></div>
    <UiCard v-else-if="unavailable" class="dashboard-error"><EmptyState icon="globe" :title="t('my.connectionTitle')" :description="t('auth.error.unavailable')"><UiButton @click="retry">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <template v-else-if="user">
      <header class="greeting">
        <div>
          <h1>{{ t('my.hello') }} {{ user.username }}!</h1>
          <p>{{ t('my.description') }}</p>
        </div>
        <UiButton href="/my/wishes/new"><AppIcon name="plus" />{{ t('my.addWish') }}</UiButton>
      </header>
      <section v-if="emptyDashboard" class="hero" aria-labelledby="dashboard-hero-title">
        <div class="hero-copy"><h2 id="dashboard-hero-title">{{ t('my.heroTitle') }}</h2><p>{{ t('my.heroDescription') }}</p></div>
        <img :src="gift" alt="" width="300" height="240" aria-hidden="true" />
      </section>
      <template v-for="section in sections" :key="section">
        <section v-if="section === 'lists'" class="dashboard-section" aria-labelledby="dashboard-lists-title">
          <div class="section-heading">
            <h2 id="dashboard-lists-title">{{ t('my.lists') }}</h2>
            <a v-if="lists.length" href="/my/lists">{{ t('dashboard.allLists') }} <AppIcon name="arrow" /></a>
          </div>
          <div v-if="listsLoading" class="list-grid" role="status" :aria-label="t('lists.loading')">
            <UiSkeleton v-for="i in 3" :key="i" class="list-skeleton" />
          </div>
          <UiCard v-else-if="listsError" class="dashboard-error">
            <EmptyState icon="globe" :title="t('lists.loadError')" :description="t('lists.error.unavailable')">
              <UiButton @click="loadLists">{{ t('checkAgain') }}</UiButton>
            </EmptyState>
          </UiCard>
          <template v-else>
            <div v-if="lists.length" class="list-grid">
              <ListCard v-for="list in lists.slice(0, 3)" :key="list.id" :list="list" preview />
            </div>
            <UiButton class="create-list" variant="secondary" href="/my/lists/new">
              <AppIcon name="plus" />{{ t('lists.create') }}
            </UiButton>
          </template>
        </section>
        <section v-else class="dashboard-section" aria-labelledby="dashboard-wishes-title">
          <div class="section-heading">
            <h2 id="dashboard-wishes-title">{{ t('dashboard.recentWishes') }}</h2>
            <a v-if="wishes.length" href="/my/wishes">{{ t('dashboard.allWishes') }} <AppIcon name="arrow" /></a>
          </div>
          <div v-if="wishesLoading" class="wish-preview-grid" role="status" :aria-label="t('wishes.loading')">
            <UiSkeleton v-for="i in 3" :key="i" class="wish-skeleton" />
          </div>
          <UiCard v-else-if="wishesError" class="dashboard-error">
            <EmptyState icon="globe" :title="t('wishes.loadErrorTitle')" :description="t('wishes.loadError')">
              <UiButton @click="loadWishes">{{ t('checkAgain') }}</UiButton>
            </EmptyState>
          </UiCard>
          <UiCard v-else-if="!wishes.length" class="wishes-empty"><p>{{ t('my.emptyWishesHint') }}</p></UiCard>
          <div v-else class="wish-preview-grid">
            <a v-for="wish in wishes.slice(0, 3)" :key="wish.id" class="wish-preview" href="/my/wishes">
              <span class="wish-image"><img v-if="wish.image_url" :src="wish.image_url" alt="" loading="lazy" /><AppIcon v-else name="heart" /></span>
              <span class="wish-title">{{ wish.title }}</span>
              <AppIcon name="arrow" />
            </a>
          </div>
        </section>
      </template>
    </template>
  </div>
</template>

<style scoped>
.dashboard { min-width: 0; overflow-wrap: anywhere; }
.greeting { display: grid; gap: var(--wx-space-4); }
.greeting h1 { margin: 0; font-size: 26px; line-height: 1.25; letter-spacing: -.03em; }
.greeting p { margin: var(--wx-space-2) 0 0; color: var(--wx-text-muted); line-height: 1.6; }
.hero { display: none; }
.hero h2 { max-width: 420px; margin: 0; font-size: var(--wx-text-xl); line-height: 1.2; letter-spacing: -.03em; }
.hero p { max-width: 490px; margin: var(--wx-space-3) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.6; }
.dashboard-section { margin-top: var(--wx-space-6); }
.section-heading { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 0 var(--wx-space-4); margin-bottom: var(--wx-space-2); }
.section-heading h2 { margin: 0; font-size: var(--wx-text-lg); line-height: 1.3; letter-spacing: -.02em; }
.section-heading a { display: inline-flex; align-items: center; gap: var(--wx-space-1); min-width: 44px; min-height: 44px; color: var(--wx-primary-pressed); font-size: var(--wx-text-sm); font-weight: 600; text-decoration: none; }
.section-heading a:hover { text-decoration: underline; }
.section-heading a .icon { width: 16px; height: 16px; }
.create-list { min-height: 44px; padding-block: var(--wx-space-2); }
.list-grid + .create-list { margin-top: var(--wx-space-3); }
.wishes-empty { padding: var(--wx-space-4) var(--wx-space-5); }
.wishes-empty p { margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.6; }
.dashboard-error { padding: var(--wx-space-8) var(--wx-space-5); }
.list-grid, .wish-preview-grid { gap: var(--wx-space-3); }
.list-grid { align-items: stretch; }
.wish-preview-grid { display: grid; }
.wish-preview {
  display: flex;
  align-items: center;
  gap: var(--wx-space-3);
  min-width: 0;
  padding: var(--wx-space-2);
  border: 1px solid var(--wx-border);
  border-radius: var(--wx-radius-card);
  background: var(--wx-surface);
  color: var(--wx-text);
  text-decoration: none;
  font-size: var(--wx-text-sm);
  font-weight: 600;
  box-shadow: var(--wx-shadow-card);
}
.wish-preview:hover { border-color: var(--wx-secondary); }
.wish-preview:active { background: var(--wx-background); }
.wish-title { flex: 1; min-width: 0; }
.wish-preview > .icon { flex-shrink: 0; color: var(--wx-primary); }
.wish-image { display: grid; place-items: center; flex: 0 0 60px; height: 60px; overflow: hidden; border-radius: var(--wx-radius-sm); color: var(--wx-primary); background: var(--wx-lavender); }
.wish-image img { display: block; width: 100%; height: 100%; object-fit: contain; background: var(--wx-surface); }
.dashboard-loading { display: grid; gap: var(--wx-space-5); }
.loading-title { width: 60%; height: 32px; }
.wish-skeleton { height: 78px; border-radius: var(--wx-radius-card); }
@media (width < 48rem) { .list-skeleton { height: 74px; } }
@media (min-width: 48rem) {
  .greeting > .button { justify-self: start; }
  .greeting h1 { font-size: var(--wx-text-2xl); }
  .section-heading h2 { font-size: 22px; }
  .list-skeleton { height: 192px; }
  .wish-preview-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (min-width: 64rem) {
  .greeting { grid-template-columns: minmax(0, 1fr) auto; align-items: start; }
  .dashboard-section { margin-top: var(--wx-space-8); }
  .hero {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 32%);
    align-items: center;
    gap: var(--wx-space-6);
    min-height: 220px;
    margin-top: var(--wx-space-6);
    padding: var(--wx-space-6) var(--wx-space-8);
    border: 1px solid var(--wx-border);
    border-radius: var(--wx-radius-hero);
    background: linear-gradient(110deg, var(--wx-cream), var(--wx-lavender));
  }
  .hero img { width: 100%; max-width: 240px; height: auto; justify-self: end; }
}
@media (min-width: 75rem) { .wish-preview-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
