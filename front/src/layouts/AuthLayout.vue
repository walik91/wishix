<script setup>
import { t } from '../i18n.js'
import AppIcon from '../components/ui/AppIcon.vue'
import SocialAuthButtons from '../components/auth/SocialAuthButtons.vue'
import gift from '../assets/brand/gift-hero.svg'

defineProps({
  title: { type: String, required: true },
  description: { type: String, required: true },
  titleId: { type: String, required: true },
  dividerLabel: { type: String, required: true },
  notice: { type: String, default: '' },
  showSocial: { type: Boolean, default: true },
  socialBusy: Boolean,
})
defineEmits(['social'])
</script>

<template>
  <section class="auth-card" :aria-labelledby="titleId">
    <aside class="brand-panel" aria-labelledby="brand-title">
      <div class="brand-copy">
        <p class="brand-eyebrow">{{ t('login.brandEyebrow') }}</p>
        <h2 id="brand-title">{{ t('login.brandTitle') }}</h2>
        <p class="brand-description">{{ t('login.brandDescription') }}</p>
      </div>
      <img class="gift-illustration" :src="gift" alt="" width="370" height="300" />
      <div class="brand-note">
        <span class="note-icon"><AppIcon name="heart" /></span>
        <p>{{ t('login.brandNote') }}</p>
      </div>
    </aside>
    <section class="form-panel" :aria-labelledby="titleId">
      <div class="form-content">
        <div class="form-heading">
          <h1 :id="titleId" tabindex="-1">{{ title }}</h1>
          <p>{{ description }}</p>
        </div>
        <SocialAuthButtons v-if="showSocial" :busy="socialBusy" @select="$emit('social', $event)" />
        <div v-if="showSocial" class="divider"><span>{{ dividerLabel }}</span></div>
        <slot />
        <p class="auth-switch"><slot name="switch" /></p>
        <p v-if="notice" class="preview-notice" role="status">{{ notice }}</p>
      </div>
    </section>
  </section>
</template>

<style scoped>
.auth-card { display: grid; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-hero); background: var(--wx-surface); box-shadow: var(--wx-shadow-panel); }
.brand-panel { display: none; }
.form-panel { min-width: 0; padding: var(--wx-space-8) var(--wx-space-5); }
.form-content { max-width: 384px; margin-inline: auto; }
.form-heading h1 { margin: 0; font-size: var(--wx-text-xl); font-weight: 750; line-height: 1.25; letter-spacing: -.03em; overflow-wrap: anywhere; }
.form-heading p { margin: var(--wx-space-3) 0 var(--wx-space-6); color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.6; }
.divider { display: flex; align-items: center; gap: var(--wx-space-4); margin-block: var(--wx-space-6); color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.5; }
.divider::before, .divider::after { content: ''; flex: 1; height: 1px; background: var(--wx-border); }
.divider span { max-width: 75%; text-align: center; }
.auth-switch { display: flex; flex-wrap: wrap; justify-content: center; align-items: center; gap: 0 var(--wx-space-2); margin: var(--wx-space-5) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.5; }
.auth-switch :deep(a) { display: inline-flex; align-items: center; justify-content: center; min-width: 44px; min-height: 44px; color: var(--wx-primary); font-weight: 600; text-decoration: none; border-radius: var(--wx-space-1); }
.auth-switch :deep(a:hover) { color: var(--wx-primary-hover); text-decoration: underline; text-underline-offset: 3px; }
.preview-notice { margin: var(--wx-space-4) 0 0; padding: var(--wx-space-3) var(--wx-space-4); border-radius: var(--wx-radius-sm); color: var(--wx-text-muted); background: var(--wx-lavender); font-size: var(--wx-text-sm); line-height: 1.5; overflow-wrap: anywhere; }
@media (min-width: 48rem) {
  .form-panel { padding: var(--wx-space-10); }
  .form-heading h1 { font-size: var(--wx-text-2xl); }
  .auth-card { margin-block: auto; }
}
@media (min-width: 64rem) {
  .auth-card { grid-template-columns: .95fr 1.05fr; }
  .brand-panel { display: flex; flex-direction: column; align-items: center; justify-content: center; min-width: 0; padding: var(--wx-space-10); border-radius: var(--wx-radius-hero) 0 0 var(--wx-radius-hero); background: linear-gradient(145deg, var(--wx-cream), var(--wx-lavender)); }
  .brand-copy { align-self: stretch; }
  .brand-eyebrow { margin: 0 0 var(--wx-space-4); color: var(--wx-primary-pressed); font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; }
  .brand-copy h2 { max-width: 340px; margin: 0; font-size: var(--wx-text-2xl); font-weight: 750; line-height: 1.25; letter-spacing: -.035em; text-wrap: balance; }
  .brand-description { max-width: 350px; margin: var(--wx-space-4) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.7; }
  .gift-illustration { display: block; width: min(100%, 340px); height: auto; margin-block: var(--wx-space-5); }
  .brand-note { display: flex; align-items: center; gap: var(--wx-space-3); max-width: 330px; }
  .note-icon { display: grid; place-items: center; flex: 0 0 40px; height: 40px; border-radius: var(--wx-radius-md); color: var(--wx-primary); background: var(--wx-surface); }
  .brand-note p { margin: 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.6; }
}
</style>
