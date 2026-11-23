<script setup>
import { computed, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import AppIcon from '../components/ui/AppIcon.vue'
import UiButton from '../components/ui/UiButton.vue'
import WishCard from '../components/wishes/WishCard.vue'
import gift from '../assets/brand/gift-hero.svg'
import { exampleWishlist, exampleWishlistPath } from '../exampleWishlist.js'

const { user, checking } = useSession()
const startHref = computed(() => user.value ? '/my/wishes' : '/register')
const startLabel = computed(() => t(checking.value ? 'auth.checkingSession' : user.value ? 'home.openMyWishes' : 'home.start'))
const exampleWish = computed(() => exampleWishlist().wishes[0])

const steps = [
  { key: 'collect', icon: 'list' },
  { key: 'share', icon: 'share' },
  { key: 'give', icon: 'heart' },
]
const questions = ['account', 'links', 'privacy']

watchEffect(() => {
  document.title = `${t('appName')} · ${t('home.pageTitle')}`
})
</script>

<template>
  <div class="guest-content">
    <section class="hero" aria-labelledby="hero-title">
      <div class="hero-copy">
        <h1 id="hero-title">{{ t('home.title') }} <span>{{ t('home.titleAccent') }}</span></h1>
        <p class="hero-description">{{ t('home.description') }}</p>
        <div class="hero-actions">
          <UiButton :href="startHref" :loading="checking">{{ startLabel }}<AppIcon name="arrow" /></UiButton>
          <UiButton :href="exampleWishlistPath" variant="secondary">{{ t('home.viewExample') }}</UiButton>
        </div>
        <p class="hero-guest-hint">{{ t('home.guestHint') }}</p>
      </div>
      <div class="hero-visual">
        <img class="gift" :src="gift" alt="" width="370" height="300" fetchpriority="high" />
        <a class="hero-example" :href="exampleWishlistPath">
          <WishCard class="hero-wish" :wish="exampleWish" preview />
          <AppIcon name="arrow" />
          <span class="sr-only"> · {{ t('home.viewExample') }}</span>
        </a>
      </div>
    </section>

    <section id="how-it-works" class="steps-section" aria-labelledby="steps-title">
      <div class="section-heading">
        <p class="eyebrow">{{ t('home.stepsEyebrow') }}</p>
        <h2 id="steps-title">{{ t('home.stepsTitle') }}</h2>
        <p>{{ t('home.stepsDescription') }}</p>
      </div>
      <ol class="steps">
        <li v-for="(step, index) in steps" :key="step.key" class="step-card">
          <div class="step-top"><span class="step-icon"><AppIcon :name="step.icon" /></span><span class="step-number" aria-hidden="true">0{{ index + 1 }}</span></div>
          <h3>{{ t(`home.${step.key}Title`) }}</h3>
          <p>{{ t(`home.${step.key}Description`) }}</p>
        </li>
      </ol>
    </section>

    <section class="guest-giving" aria-labelledby="giving-title">
      <div class="giving-icon"><AppIcon name="share" /></div>
      <div class="giving-copy"><h2 id="giving-title">{{ t('home.givingTitle') }}</h2><p>{{ t('home.givingDescription') }}</p></div>
      <span class="guest-badge"><AppIcon name="check" />{{ t('home.noAccount') }}</span>
    </section>

    <section id="questions" class="faq-section" aria-labelledby="questions-title">
      <div class="section-heading"><p class="eyebrow">{{ t('home.questionsEyebrow') }}</p><h2 id="questions-title">{{ t('home.questionsTitle') }}</h2></div>
      <div class="questions">
        <details v-for="question in questions" :key="question">
          <summary>{{ t(`home.faq.${question}Question`) }}<AppIcon name="chevron" /></summary>
          <p>{{ t(`home.faq.${question}Answer`) }}</p>
        </details>
      </div>
    </section>

    <section class="closing" aria-labelledby="closing-title">
      <span class="closing-heart"><AppIcon name="heart" /></span>
      <h2 id="closing-title">{{ t('home.closingTitle') }}</h2>
      <p>{{ t('home.closingDescription') }}</p>
      <UiButton :href="startHref" :loading="checking">{{ startLabel }}<AppIcon name="arrow" /></UiButton>
    </section>
  </div>
</template>

<style scoped>
.hero { display: grid; gap: var(--wx-space-4); padding: var(--wx-space-5) var(--wx-space-5) var(--wx-space-4); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-hero); background: linear-gradient(125deg, var(--wx-cream), var(--wx-lavender)); }
.hero-copy { min-width: 0; }
.eyebrow { display: flex; align-items: center; gap: var(--wx-space-2); margin: 0 0 var(--wx-space-4); color: var(--wx-primary-pressed); font-size: var(--wx-text-xs); font-weight: 600; line-height: 1.6; }
h1 { max-width: 470px; margin: 0; font-size: var(--wx-text-xl); font-weight: 800; line-height: 1.25; letter-spacing: -.035em; text-wrap: balance; }
h1 span { color: var(--wx-primary); }
.hero-description { max-width: 440px; margin: var(--wx-space-3) 0 var(--wx-space-4); color: var(--wx-text-muted); font-size: var(--wx-text-md); line-height: 1.5; }
.hero-actions { display: flex; flex-direction: column; align-items: stretch; gap: var(--wx-space-2); }
.hero-guest-hint { max-width: 440px; margin: var(--wx-space-3) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.5; }
.hero-visual { display: grid; justify-items: center; align-content: center; min-width: 0; }
.gift { display: block; width: min(100%, 300px); height: auto; }
.hero-example { position: relative; width: min(100%, 320px); margin-top: calc(-1 * var(--wx-space-5)); border-radius: var(--wx-radius-card); color: inherit; text-decoration: none; }
.hero-example .hero-wish { padding: var(--wx-space-3); padding-inline-end: var(--wx-space-10); }
.hero-example > .icon { position: absolute; inset-inline-end: var(--wx-space-3); top: 50%; width: 20px; height: 20px; transform: translateY(-50%); color: var(--wx-primary); }
.hero-example:hover .hero-wish { border-color: var(--wx-secondary); }
.hero-example:active .hero-wish { background: var(--wx-background); }
.step-icon, .giving-icon, .closing-heart { display: grid; place-items: center; flex: 0 0 auto; width: 44px; height: 44px; border-radius: var(--wx-radius-md); color: var(--wx-primary); background: var(--wx-lavender); }
.steps-section, .faq-section { padding-top: calc(var(--wx-space-10) + var(--wx-space-6)); scroll-margin-top: var(--wx-space-6); }
.section-heading { max-width: 560px; margin-bottom: var(--wx-space-6); }
.section-heading .eyebrow { margin-bottom: var(--wx-space-2); }
h2 { margin: 0; font-size: var(--wx-text-lg); font-weight: 750; line-height: 1.35; letter-spacing: -.025em; text-wrap: balance; }
.section-heading > p:last-child:not(.eyebrow) { margin: var(--wx-space-3) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.7; }
.steps { display: grid; gap: var(--wx-space-4); margin: 0; padding: 0; list-style: none; }
.step-card { min-width: 0; padding: var(--wx-space-6); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); background: var(--wx-surface); box-shadow: var(--wx-shadow-card); }
.step-top { display: flex; justify-content: space-between; align-items: center; }
.step-number { color: var(--wx-text-muted); font-size: var(--wx-text-xs); font-variant-numeric: tabular-nums; }
.step-card h3 { margin: var(--wx-space-5) 0 var(--wx-space-2); font-size: var(--wx-text-md); line-height: 1.4; }
.step-card p { margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.7; }
.guest-giving { display: flex; flex-wrap: wrap; align-items: flex-start; gap: var(--wx-space-4); margin-top: var(--wx-space-6); padding: var(--wx-space-6); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); background: var(--wx-surface); }
.giving-icon { background: var(--wx-cream); }
.giving-copy { flex: 1 1 180px; }
.giving-copy h2 { font-size: var(--wx-text-md); }
.giving-copy p { max-width: 620px; margin: var(--wx-space-2) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.7; }
.guest-badge { display: inline-flex; align-items: flex-start; gap: var(--wx-space-2); padding: var(--wx-space-2) var(--wx-space-3); border-radius: var(--wx-radius-md); background: var(--wx-lavender); color: var(--wx-primary-pressed); font-size: var(--wx-text-xs); line-height: 1.6; }
.guest-badge .icon { width: 18px; height: 18px; }
.questions { border-top: 1px solid var(--wx-border); }
details { border-bottom: 1px solid var(--wx-border); }
summary { display: flex; align-items: center; justify-content: space-between; gap: var(--wx-space-4); min-height: 60px; padding-block: var(--wx-space-4); border-radius: var(--wx-space-1); font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.6; list-style: none; cursor: pointer; }
summary::-webkit-details-marker { display: none; }
summary:hover { color: var(--wx-primary); }
summary .icon { color: var(--wx-primary); }
details[open] summary .icon { transform: rotate(180deg); }
details > p { max-width: 700px; margin: 0 0 var(--wx-space-5); padding-right: var(--wx-space-8); color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.8; }
.closing { display: flex; flex-direction: column; align-items: center; margin-top: calc(var(--wx-space-10) + var(--wx-space-6)); padding: var(--wx-space-8) var(--wx-space-5); border-radius: var(--wx-radius-hero); background: var(--wx-lavender); text-align: center; }
.closing-heart { margin-bottom: var(--wx-space-4); background: var(--wx-surface); }
.closing > p { max-width: 460px; margin: var(--wx-space-3) 0 var(--wx-space-6); color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.7; }
@media (max-width: 22rem) {
  .hero { padding-inline: var(--wx-space-4); }
  .hero-description { display: none; }
  .hero-actions { margin-top: var(--wx-space-4); }
}
@media (min-width: 36rem) {
  .hero { padding: var(--wx-space-10); }
  .hero-actions { flex-direction: row; align-items: center; gap: var(--wx-space-5); }
}
@media (min-width: 48rem) {
  .hero { grid-template-columns: 1.15fr .85fr; align-items: center; gap: var(--wx-space-6); min-height: 440px; padding: var(--wx-space-10) var(--wx-space-8); }
  h1 { font-size: var(--wx-text-2xl); }
  .hero-description { margin: var(--wx-space-5) 0 var(--wx-space-6); line-height: 1.7; }
  .hero-actions { flex-wrap: wrap; gap: var(--wx-space-2) var(--wx-space-5); }
  .gift { width: min(100%, 370px); }
  .hero-example { margin-top: calc(-1 * var(--wx-space-3)); }
  .section-heading { margin-bottom: var(--wx-space-8); }
  .steps { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .step-card { padding: var(--wx-space-5); }
  h2 { font-size: var(--wx-text-xl); }
  .faq-section { display: grid; grid-template-columns: .8fr 1.2fr; gap: var(--wx-space-10); }
  .closing { padding: var(--wx-space-10); }
}
@media (min-width: 64rem) {
  .hero { grid-template-columns: 1fr 1fr; min-height: 450px; padding-inline: calc(var(--wx-space-10) + var(--wx-space-4)); }
  .eyebrow { font-size: var(--wx-text-sm); }
  .steps-section, .faq-section { padding-top: calc(var(--wx-space-10) * 2); }
  .step-card { padding: var(--wx-space-6); }
  .guest-giving { align-items: center; padding: var(--wx-space-6) var(--wx-space-8); }
  .closing { margin-top: calc(var(--wx-space-10) * 2); }
}
</style>
