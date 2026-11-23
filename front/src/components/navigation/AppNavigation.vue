<script setup>
import { t } from '../../i18n.js'
import AppIcon from '../ui/AppIcon.vue'
defineProps({ profileAvailable: Boolean, profileActive: Boolean })
defineEmits(['profile'])
const path = window.location.pathname.replace(/\/+$/, '')
const inWishes = path.startsWith('/my/wishes')
const inLists = path.startsWith('/my/lists')
const inSearch = path === '/my/search'
const inFollowing = path === '/my/following'
</script>

<template>
  <nav class="app-navigation" :aria-label="t('my.navigation')">
    <a class="nav-item" :class="{ active: !inLists && !inWishes && !inSearch && !inFollowing && !profileActive }" href="/my" :aria-current="!inLists && !inWishes && !inSearch && !inFollowing && !profileActive ? 'page' : undefined"><AppIcon name="home" /><span>{{ t('auth.my') }}</span></a>
    <a class="nav-item" :class="{ active: inWishes }" href="/my/wishes" :aria-current="inWishes ? 'page' : undefined"><AppIcon name="heart" /><span>{{ t('my.myWishes') }}</span></a>
    <a class="nav-item" :class="{ active: inLists }" href="/my/lists" :aria-current="inLists ? 'page' : undefined"><AppIcon name="list" /><span>{{ t('lists.nav') }}</span></a>
    <a class="nav-item" :class="{ active: inSearch }" href="/my/search" :aria-current="inSearch ? 'page' : undefined"><AppIcon name="search" /><span>{{ t('my.search') }}</span></a>
    <a class="nav-item following" :class="{ active: inFollowing }" href="/my/following" :aria-current="inFollowing ? 'page' : undefined"><AppIcon name="users" /><span>{{ t('my.following') }}</span></a>
    <button class="nav-item" :class="{ active: profileActive }" type="button" aria-haspopup="dialog" :aria-expanded="profileActive" :disabled="!profileAvailable" @click="$emit('profile')"><AppIcon name="user" /><span>{{ t('my.profile') }}</span></button>
  </nav>
</template>

<style scoped>
.app-navigation { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: var(--wx-space-1); }
.nav-item { display: flex; flex-direction: column; align-items: center; justify-content: flex-start; gap: var(--wx-space-1); min-width: 0; min-height: 56px; padding: var(--wx-space-1) 0; border: 0; border-radius: var(--wx-radius-md); color: var(--wx-text-muted); background: transparent; font-size: var(--wx-text-xs); font-weight: 500; line-height: 1.4; text-align: center; text-decoration: none; cursor: pointer; }
.nav-item .icon { width: 22px; height: 22px; }
.nav-item span { max-width: 100%; }
.active { color: var(--wx-primary); background: var(--wx-lavender); font-weight: 600; }
.nav-item:hover:not(:disabled, .active) { color: var(--wx-primary); background: var(--wx-background); }
.nav-item:active:not(:disabled) { background: var(--wx-lavender); }
.nav-item:disabled { opacity: .55; cursor: not-allowed; }
small { display: none; }
.following { display: none; }
@media (width < 64rem) {
  .nav-item:focus-visible { outline-offset: -3px; }
}
@media (min-width: 64rem) {
  .app-navigation { grid-template-columns: 1fr; gap: var(--wx-space-2); }
  .nav-item { flex-direction: row; justify-content: flex-start; gap: var(--wx-space-3); min-height: 48px; padding: var(--wx-space-3); font-size: var(--wx-text-sm); text-align: left; overflow-wrap: anywhere; }
  small { display: inline; margin-left: auto; font-size: var(--wx-text-xs); font-weight: 400; }
  .following { display: flex; }
}
</style>
