<script setup>
import { computed } from 'vue'
import { locale, t } from '../../i18n.js'
import AppIcon from '../ui/AppIcon.vue'
import UiCard from '../ui/UiCard.vue'
const props = defineProps({
  list: { type: Object, required: true },
  publicHref: { type: String, default: null },
  preview: Boolean,
})
const isPublicCard = computed(() => props.publicHref !== null)
const path = computed(() => `/my/lists/${props.list.id}`)
const eventDate = computed(() => props.list.event_date ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeZone: 'UTC' }).format(new Date(`${props.list.event_date}T00:00:00Z`)) : '')
</script>
<template>
  <UiCard
    class="list-card"
    :class="{ 'list-card-preview': preview, 'list-card-public': isPublicCard }"
    :href="publicHref || undefined"
    :data-list-id="list.id"
  >
    <component :is="isPublicCard ? 'div' : 'a'" class="list-card-main" :href="isPublicCard ? undefined : path">
      <div class="list-cover" aria-hidden="true"><AppIcon :name="list.event_date ? 'calendar' : 'list'" /></div>
      <div class="list-card-content">
        <component :is="preview || isPublicCard ? 'h3' : 'h2'" class="list-card-title">{{ list.title }}</component>
        <p :class="{ 'list-card-meta': isPublicCard }">
          <span>{{ t('lists.wishesCount') }} {{ list.wish_count }}</span>
          <time v-if="isPublicCard && eventDate" :datetime="list.event_date">{{ eventDate }}</time>
        </p>
        <p v-if="!isPublicCard && eventDate">{{ eventDate }}</p>
        <p v-if="!isPublicCard && list.expired">{{ t('lists.expired') }}</p>
      </div>
      <AppIcon v-if="publicHref" class="list-card-arrow" name="arrow" />
    </component>
    <div v-if="!isPublicCard" class="list-card-footer"><span class="list-visibility"><AppIcon :name="list.visibility === 'private' ? 'lock' : list.visibility === 'public' ? 'globe' : 'link'" />{{ t(`lists.visibility.${list.visibility}`) }}</span><a class="list-card-menu" :href="`${path}?manage=1`" :aria-label="`${t('lists.manage')}: ${list.title}`"><AppIcon name="more" /></a></div>
  </UiCard>
</template>
<style scoped>
.list-card { min-width: 0; overflow: hidden; }.list-card-main { display: flex; gap: var(--wx-space-4); align-items: center; min-height: 88px; padding: var(--wx-space-4); color: var(--wx-text); text-decoration: none; }.list-card-main:hover { background: var(--wx-background); }.list-cover { display: flex; align-items: center; justify-content: center; flex: 0 0 56px; height: 64px; border-radius: var(--wx-radius-sm); color: var(--wx-primary); background: linear-gradient(130deg, var(--wx-background), var(--wx-lavender)); }.list-cover .icon { width: 28px; height: 28px; }.list-card-content { min-width: 0; overflow-wrap: anywhere; }.list-card-title { margin: 0; font-size: var(--wx-text-md); line-height: 1.5; font-weight: 650; }p { margin: var(--wx-space-1) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.5; }.list-card-footer { display: flex; align-items: center; justify-content: space-between; min-height: 52px; gap: var(--wx-space-2); padding: var(--wx-space-1) var(--wx-space-3) var(--wx-space-2) var(--wx-space-4); border-top: 1px solid var(--wx-border); }.list-visibility { display: inline-flex; align-items: center; gap: var(--wx-space-2); color: var(--wx-text-muted); font-size: var(--wx-text-xs); }.list-visibility .icon { width: 16px; height: 16px; }.list-card-menu { display: inline-flex; align-items: center; justify-content: center; width: 44px; height: 44px; flex-shrink: 0; border-radius: var(--wx-radius-sm); color: var(--wx-text-muted); }.list-card-menu:hover { color: var(--wx-primary); background: var(--wx-lavender); }
.list-card-preview .list-card-main:focus-visible { outline-offset: -3px; }
.list-card-public { display: block; color: var(--wx-text); text-decoration: none; }
a.list-card-public:hover { border-color: var(--wx-secondary); background: var(--wx-background); }
a.list-card-public:active { background: var(--wx-lavender); }
.list-card-public .list-card-main {
  flex-direction: row;
  align-items: center;
  gap: var(--wx-space-3);
  min-height: 68px;
  padding: var(--wx-space-3);
  background: transparent;
}
.list-card-public .list-cover { flex: 0 0 44px; width: 44px; height: 44px; background: var(--wx-lavender); }
.list-card-public .list-cover .icon { width: 20px; height: 20px; }
.list-card-public .list-card-content { flex: 1; min-height: 0; }
.list-card-meta { display: flex; flex-wrap: wrap; column-gap: var(--wx-space-3); }
.list-card-arrow { color: var(--wx-primary); }
@media (width < 48rem) {
  .list-card-preview { display: grid; grid-template-columns: minmax(0, 1fr) auto; }
  .list-card-preview .list-card-main { min-height: 72px; gap: var(--wx-space-3); padding: var(--wx-space-2); }
  .list-card-preview .list-cover { height: 56px; }
  .list-card-preview .list-card-title { font-size: var(--wx-text-sm); }
  .list-card-preview .list-card-footer { flex-direction: column-reverse; justify-content: center; gap: 0; padding: var(--wx-space-1) var(--wx-space-2); border: 0; }
  .list-card-preview .list-visibility { gap: var(--wx-space-1); }
}
@media(min-width: 48rem) { .list-card-main { flex-direction: column; align-items: stretch; }.list-cover { flex-basis: auto; width: 100%; height: 104px; }.list-card-content { min-height: 72px; } }
@media (min-width: 48rem) {
  .list-card-preview { display: flex; flex-direction: column; }
  .list-card-preview .list-card-main { flex: 1; gap: var(--wx-space-2); padding: var(--wx-space-3); }
  .list-card-preview .list-cover { height: 72px; }
  .list-card-preview .list-card-content { min-height: 0; }
  .list-card-preview .list-card-footer { min-height: 44px; padding-block: 0; }
}
</style>
