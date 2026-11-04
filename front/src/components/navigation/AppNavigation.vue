<script setup>
import { t } from '../../i18n.js'
import AppIcon from '../ui/AppIcon.vue'
defineProps({ profileAvailable: Boolean })
defineEmits(['profile'])
const inLists = window.location.pathname.startsWith('/my/lists')
</script>

<template>
  <nav class="app-navigation" :aria-label="t('my.navigation')">
    <a class="nav-item" :class="{ active: !inLists }" href="/my" :aria-current="!inLists ? 'page' : undefined"><AppIcon name="home" /><span>{{ t('auth.my') }}</span></a>
    <a class="nav-item" :class="{ active: inLists }" href="/my/lists" :aria-current="inLists ? 'page' : undefined"><AppIcon name="list" /><span>{{ t('lists.nav') }}</span></a>
    <button class="nav-item" type="button" disabled :title="t('my.sectionSoon')"><AppIcon name="users" /><span>{{ t('my.following') }}</span><small>{{ t('my.soon') }}</small></button>
    <button class="nav-item" type="button" :disabled="!profileAvailable" @click="$emit('profile')"><AppIcon name="user" /><span>{{ t('my.profile') }}</span></button>
  </nav>
</template>

<style scoped>
.app-navigation { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--wx-space-1); }
.nav-item { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: var(--wx-space-1); min-width: 0; min-height: 56px; padding: var(--wx-space-2) var(--wx-space-1); border: 0; border-radius: var(--wx-radius-md); color: var(--wx-text-muted); background: transparent; font-size: var(--wx-text-xs); font-weight: 500; line-height: 1.4; text-decoration: none; overflow-wrap: anywhere; cursor: pointer; }
.nav-item .icon { width: 22px; height: 22px; }
.active { color: var(--wx-primary); background: var(--wx-lavender); font-weight: 600; }
.nav-item:hover:not(:disabled, .active) { color: var(--wx-primary); background: var(--wx-background); }
.nav-item:active:not(:disabled) { background: var(--wx-lavender); }
.nav-item:disabled { opacity: .55; cursor: not-allowed; }
small { display: none; }
@media (min-width: 64rem) {
  .app-navigation { grid-template-columns: 1fr; gap: var(--wx-space-2); }
  .nav-item { flex-direction: row; justify-content: flex-start; gap: var(--wx-space-3); min-height: 48px; padding: var(--wx-space-3); font-size: var(--wx-text-sm); }
  small { display: inline; margin-left: auto; font-size: var(--wx-text-xs); font-weight: 400; }
}
</style>
