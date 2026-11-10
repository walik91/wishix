<script setup>
import { computed } from 'vue'
import { locale, t } from '../../i18n.js'
import AppIcon from '../ui/AppIcon.vue'
import UiCard from '../ui/UiCard.vue'
const props = defineProps({ list: { type: Object, required: true }, href: { type: String, default: '' }, manage: { type: Boolean, default: true } })
const path = computed(() => props.href || `/my/lists/${props.list.id}`)
const eventDate = computed(() => props.list.event_date ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeZone: 'UTC' }).format(new Date(`${props.list.event_date}T00:00:00Z`)) : '')
</script>
<template>
  <UiCard class="list-card" :data-list-id="list.id">
    <a class="list-card-main" :href="path"><div class="list-cover" aria-hidden="true"><AppIcon :name="list.event_date ? 'calendar' : 'list'" /></div><div class="list-card-content"><h2>{{ list.title }}</h2><p>{{ t('lists.wishesCount') }} {{ list.wish_count }}</p><p v-if="eventDate">{{ eventDate }}</p><p v-if="list.expired">{{ t('lists.expired') }}</p></div></a>
    <div class="list-card-footer"><span class="list-visibility"><AppIcon :name="list.visibility === 'private' ? 'lock' : list.visibility === 'public' ? 'globe' : 'link'" />{{ t(`lists.visibility.${list.visibility}`) }}</span><a v-if="manage" class="list-card-menu" :href="`${path}?manage=1`" :aria-label="`${t('lists.manage')}: ${list.title}`"><AppIcon name="more" /></a></div>
  </UiCard>
</template>
<style scoped>
.list-card { min-width: 0; overflow: hidden; }.list-card-main { display: flex; gap: var(--wx-space-4); align-items: center; min-height: 88px; padding: var(--wx-space-4); color: var(--wx-text); text-decoration: none; }.list-card-main:hover { background: var(--wx-background); }.list-cover { display: flex; align-items: center; justify-content: center; flex: 0 0 56px; height: 64px; border-radius: var(--wx-radius-sm); color: var(--wx-primary); background: linear-gradient(130deg, var(--wx-background), var(--wx-lavender)); }.list-cover .icon { width: 28px; height: 28px; }.list-card-content { min-width: 0; overflow-wrap: anywhere; }h2 { margin: 0; font-size: var(--wx-text-md); line-height: 1.5; font-weight: 650; }p { margin: var(--wx-space-1) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.5; }.list-card-footer { display: flex; align-items: center; justify-content: space-between; min-height: 52px; gap: var(--wx-space-2); padding: var(--wx-space-1) var(--wx-space-3) var(--wx-space-2) var(--wx-space-4); border-top: 1px solid var(--wx-border); }.list-visibility { display: inline-flex; align-items: center; gap: var(--wx-space-2); color: var(--wx-text-muted); font-size: var(--wx-text-xs); }.list-visibility .icon { width: 16px; height: 16px; }.list-card-menu { display: inline-flex; align-items: center; justify-content: center; width: 44px; height: 44px; flex-shrink: 0; border-radius: var(--wx-radius-sm); color: var(--wx-text-muted); }.list-card-menu:hover { color: var(--wx-primary); background: var(--wx-lavender); }
@media(min-width: 48rem) { .list-card-main { flex-direction: column; align-items: stretch; }.list-cover { flex-basis: auto; width: 100%; height: 104px; }.list-card-content { min-height: 72px; } }
</style>
