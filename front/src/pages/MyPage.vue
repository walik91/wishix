<script setup>
import { watchEffect } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiBadge from '../components/ui/UiBadge.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import ListCard from '../components/wishes/ListCard.vue'
import WishCard from '../components/wishes/WishCard.vue'
import gift from '../assets/brand/gift-hero.svg'

const { user, checking, unavailable, refresh } = useSession()
watchEffect(() => { document.title = `${t('auth.my')} · ${t('appName')}` })

async function retry() {
  await refresh()
  if (!user.value && !unavailable.value) window.location.replace('/login')
}
</script>

<template>
  <div class="my-page" :aria-busy="checking">
    <div v-if="checking" class="page-loading">
      <p class="sr-only" role="status">{{ t('auth.checkingSession') }}</p>
      <UiSkeleton class="loading-title" />
      <UiSkeleton class="loading-subtitle" />
      <UiSkeleton class="loading-hero" />
      <div class="loading-cards">
        <UiSkeleton v-for="index in 3" :key="index" />
      </div>
    </div>
    <UiCard v-else-if="unavailable" class="connection-error">
      <EmptyState icon="globe" :title="t('my.connectionTitle')" :description="t('auth.error.unavailable')">
        <UiButton @click="retry">{{ t('checkAgain') }}</UiButton>
      </EmptyState>
      <p class="sr-only" role="alert">{{ t('auth.error.unavailable') }}</p>
    </UiCard>
    <template v-else-if="user">
      <div class="greeting">
        <h1>{{ t('my.hello') }}<span class="greeting-name">{{ user.username }}!</span></h1>
        <p>{{ t('my.description') }}</p>
      </div>

      <section class="welcome-hero" aria-labelledby="hero-title">
        <div class="hero-copy">
          <h2 id="hero-title">{{ t('my.heroTitle') }}</h2>
          <p>{{ t('my.heroDescription') }}</p>
          <div class="hero-action">
            <UiButton disabled :title="t('my.listsSoon')"><AppIcon name="plus" />{{ t('my.createList') }}</UiButton>
            <span>{{ t('my.soon') }}</span>
          </div>
        </div>
        <img class="hero-gift" :src="gift" alt="" width="370" height="300" aria-hidden="true" />
      </section>

      <section id="my-lists" class="collection-section" aria-labelledby="lists-title">
        <div class="section-heading">
          <div>
            <h2 id="lists-title">{{ t('my.lists') }}</h2>
            <p>{{ t('my.listsDescription') }}</p>
          </div>
          <UiBadge>{{ t('my.template') }}</UiBadge>
        </div>
        <div class="list-grid">
          <ListCard />
          <ListCard icon="calendar" warm />
          <button class="create-list-template" type="button" disabled :title="t('my.listsSoon')">
            <span class="add-circle"><AppIcon name="plus" /></span>
            <span>{{ t('my.newList') }}</span>
            <small>{{ t('my.soon') }}</small>
          </button>
        </div>
      </section>

      <section id="all-wishes" class="collection-section" aria-labelledby="wishes-title">
        <div class="section-heading">
          <div>
            <h2 id="wishes-title">{{ t('my.allWishes') }}</h2>
            <p>{{ t('my.wishesDescription') }}</p>
          </div>
          <UiBadge>{{ t('my.soon') }}</UiBadge>
        </div>
        <UiCard class="wishes-empty">
          <EmptyState :title="t('my.emptyWishesTitle')" :description="t('my.emptyWishesDescription')">
            <UiButton variant="secondary" disabled :title="t('my.wishesSoon')"><AppIcon name="plus" />{{ t('my.addWish') }}</UiButton>
          </EmptyState>
          <div class="wish-preview" aria-hidden="true">
            <WishCard />
            <WishCard />
          </div>
        </UiCard>
      </section>
      <p class="privacy-note">
        <AppIcon name="lock" /><span>{{ t('my.privacyNote') }}</span>
      </p>
    </template>
  </div>
</template>

<style scoped>
.my-page { min-width: 0; overflow-wrap: anywhere; }
.greeting { margin-bottom: var(--wx-space-6); }
h1 { display: flex; flex-wrap: wrap; column-gap: .3em; margin: 0; font-size: 26px; font-weight: 750; line-height: 1.25; letter-spacing: -.03em; }
.greeting-name { min-width: 0; }
.greeting > p { margin: var(--wx-space-2) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.6; }
.welcome-hero { position: relative; isolation: isolate; }
.hero-copy { position: relative; z-index: 1; max-width: 420px; }
.hero-copy h2 { display: none; margin: 0; max-width: 350px; font-size: var(--wx-text-xl); line-height: 1.25; letter-spacing: -.025em; }
.hero-copy > p { display: none; margin: var(--wx-space-3) 0 var(--wx-space-5); color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.6; }
.hero-action { display: flex; flex-wrap: wrap; align-items: center; gap: var(--wx-space-2); }
.hero-action > .button { width: 100%; }
.hero-action > span { color: var(--wx-text-muted); font-size: var(--wx-text-xs); }
.hero-gift { display: none; }
.collection-section { scroll-margin-top: var(--wx-space-6); margin-top: var(--wx-space-8); }
.section-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--wx-space-3); margin-bottom: var(--wx-space-4); }
.section-heading h2 { margin: 0; font-size: var(--wx-text-lg); line-height: 1.35; letter-spacing: -.02em; }
.section-heading p { margin: var(--wx-space-1) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.6; }
.section-heading > .badge { flex-shrink: 0; border: 1px solid var(--wx-border); }
.list-grid { display: grid; gap: var(--wx-space-3); }
.create-list-template { display: flex; align-items: center; justify-content: center; gap: var(--wx-space-3); min-width: 0; min-height: 80px; padding: var(--wx-space-4); border: 1px dashed var(--wx-secondary); border-radius: var(--wx-radius-card); color: var(--wx-text-muted); background: var(--wx-background); font-size: var(--wx-text-sm); font-weight: 500; cursor: not-allowed; }
.add-circle { display: inline-flex; align-items: center; justify-content: center; width: 36px; height: 36px; border-radius: 50%; color: var(--wx-primary); background: var(--wx-lavender); }
.create-list-template small { font-size: var(--wx-text-xs); font-weight: 400; }
.wishes-empty { padding: var(--wx-space-8) var(--wx-space-5); }
.wish-preview { display: none; }
.privacy-note { display: flex; align-items: flex-start; justify-content: center; gap: var(--wx-space-2); margin: var(--wx-space-6) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.6; }
.privacy-note .icon { width: 16px; height: 18px; }
.page-loading { display: grid; gap: var(--wx-space-4); }
.loading-title { width: 60%; height: 32px; }
.loading-subtitle { width: 85%; }
.loading-hero { height: 224px; margin-block: var(--wx-space-2); border-radius: var(--wx-radius-hero); }
.loading-cards { display: grid; gap: var(--wx-space-4); }
.loading-cards > .skeleton { height: 96px; border-radius: var(--wx-radius-card); }
.connection-error { margin-block: var(--wx-space-8); padding: var(--wx-space-10) var(--wx-space-6); }
@media (min-width: 48rem) {
  h1 { font-size: var(--wx-text-2xl); }
  .greeting > p { font-size: var(--wx-text-md); }
  .welcome-hero { display: flex; align-items: center; min-height: 240px; overflow: hidden; padding: var(--wx-space-8); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-hero); background: linear-gradient(115deg, var(--wx-cream), var(--wx-lavender)); }
  .hero-copy { max-width: 58%; }
  .hero-copy h2 { display: block; font-size: 28px; }
  .hero-copy > p { display: block; }
  .hero-action { gap: var(--wx-space-3); }
  .hero-action > .button { width: auto; }
  .hero-gift { position: absolute; display: block; right: var(--wx-space-3); bottom: -6px; width: 280px; height: auto; }
  .section-heading h2 { font-size: var(--wx-text-xl); }
  .section-heading p { font-size: var(--wx-text-sm); }
  .list-grid, .loading-cards { grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--wx-space-4); }
  .create-list-template { flex-direction: column; gap: var(--wx-space-2); }
  .add-circle { width: 44px; height: 44px; }
  .wishes-empty { padding: var(--wx-space-8); }
}
@media (min-width: 80rem) {
  .welcome-hero { padding: var(--wx-space-8) var(--wx-space-10); }
  .hero-gift { width: 310px; right: var(--wx-space-8); bottom: -12px; }
  .hero-copy { max-width: 52%; }
  .wishes-empty { display: flex; align-items: center; justify-content: space-around; gap: var(--wx-space-8); }
  .wishes-empty > .empty-state { flex: 1; }
  .wish-preview { display: grid; flex: 0 0 260px; gap: var(--wx-space-3); transform: rotate(-4deg); }
  .wish-preview > :last-child { transform: translateX(-12px); opacity: .6; }
}
</style>
