<script setup>
import { t } from '../../i18n.js'
import LanguagePicker from '../ui/LanguagePicker.vue'
import UiButton from '../ui/UiButton.vue'
import logo from '../../assets/brand/logo-lockup.svg'
import { useSession } from '../../composables/useSession.js'
const { user, checking } = useSession()
</script>

<template>
  <header class="public-header">
    <a class="brand" href="/" :aria-label="t('appName')"><img :src="logo" :alt="t('appName')" width="147" height="45" /></a>
    <nav class="section-nav" :aria-label="t('home.navigation')">
      <a href="/#how-it-works">{{ t('home.howItWorks') }}</a>
      <a href="/#questions">{{ t('home.questions') }}</a>
    </nav>
    <LanguagePicker class="language" />
    <nav class="account-nav" :aria-label="t('home.accountNavigation')">
      <UiButton v-if="user" href="/my">{{ t('auth.my') }}</UiButton>
      <template v-else-if="!checking">
        <a class="login-link" href="/login">{{ t('login.submit') }}</a>
        <UiButton href="/register">{{ t('login.createAccount') }}</UiButton>
      </template>
    </nav>
  </header>
</template>

<style scoped>
.public-header { display: grid; grid-template-columns: 1fr auto; align-items: center; gap: var(--wx-space-4); padding-block: var(--wx-space-5); }
.brand { display: inline-flex; align-items: center; min-height: 44px; width: fit-content; max-width: 100%; border-radius: var(--wx-radius-sm); }
.brand img { display: block; max-width: 100%; height: auto; }
.language { justify-self: end; }
.section-nav { display: none; }
.account-nav { grid-column: 1 / -1; display: flex; align-items: center; gap: var(--wx-space-4); }
.account-nav .button { flex: 1; }
.login-link, .section-nav a { display: inline-flex; align-items: center; justify-content: center; min-height: 44px; border-radius: var(--wx-space-1); color: var(--wx-primary); font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; text-align: center; text-decoration: none; }
.login-link { padding-inline: var(--wx-space-3); }
.login-link:hover, .section-nav a:hover { color: var(--wx-primary-hover); text-decoration: underline; text-underline-offset: 4px; }
@media (min-width: 36rem) {
  .public-header { display: flex; flex-wrap: wrap; }
  .language { margin-left: auto; }
  .account-nav { margin-left: auto; }
  .account-nav .button { flex: initial; }
}
@media (min-width: 48rem) { .public-header { padding-block: var(--wx-space-6); } }
@media (min-width: 64rem) {
  .section-nav { display: flex; gap: var(--wx-space-6); margin-left: var(--wx-space-8); }
  .section-nav a { color: var(--wx-text-muted); }
  .account-nav { margin-left: 0; }
}
</style>
