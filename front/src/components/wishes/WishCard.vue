<script setup>
import { computed, ref } from 'vue'
import { locale, t } from '../../i18n.js'
import { copyWish } from '../../api/wishes.js'
import { trackEvent } from '../../analytics.js'
import AppIcon from '../ui/AppIcon.vue'
import UiCard from '../ui/UiCard.vue'
import UiBadge from '../ui/UiBadge.vue'
import UiButton from '../ui/UiButton.vue'
import UiDialog from '../ui/UiDialog.vue'

const props = defineProps({ wish: { type: Object, required: true }, readOnly: Boolean, removeFromList: Boolean })
const windowListID = window.location.pathname.split('/').pop()
const emit = defineEmits(['delete', 'image', 'copied'])
const copying = ref(false), copyError = ref(false)
const descriptionOpen = ref(false)
async function copy() {
  if (copying.value) return
  copying.value = true; copyError.value = false
  try { const { wish } = await copyWish(props.wish.id); emit('copied', wish) }
  catch (failure) {
    if (failure.status === 401) window.location.replace('/login')
    else copyError.value = true
  } finally { copying.value = false }
}
const price = computed(() => {
  if (props.wish.price === null) return ''
  try {
    return new Intl.NumberFormat(locale.value, { style: 'currency', currency: props.wish.currency }).format(Number(props.wish.price))
  } catch { return `${props.wish.price} ${props.wish.currency}` }
})
function shopHost(link) { try { return new URL(link).hostname } catch { return link } }
</script>

<template>
  <UiCard class="wish-card" :data-wish-id="wish.id">
    <div class="wish-heading">
      <component :is="readOnly ? 'div' : 'button'" class="wish-image" :type="readOnly ? undefined : 'button'" :aria-label="readOnly ? undefined : `${t(wish.image_url ? 'wishes.replaceImage' : 'wishes.chooseImage')}: ${wish.title}`" @click="!readOnly && $emit('image', wish)"><img v-if="wish.image_url" :src="wish.image_url" :alt="wish.title" loading="lazy" decoding="async" /><AppIcon v-else name="image" /></component>
      <div class="wish-summary">
        <h2>{{ wish.title }}</h2>
        <p v-if="price" class="wish-price"><span aria-hidden="true">≈ </span><span class="sr-only">{{ t('wishes.approximate') }} </span>{{ price }}</p>
      </div>
      <button v-if="!readOnly" class="delete-wish" type="button" :aria-label="`${t(removeFromList ? 'lists.remove' : 'wishes.delete')}: ${wish.title}`" @click="$emit('delete', wish)"><AppIcon :name="removeFromList ? 'close' : 'trash'" /></button>
    </div>
    <UiBadge class="priority" :class="wish.priority">{{ t(`wishes.priority.${wish.priority}`) }}</UiBadge>
    <UiBadge v-if="wish.received" class="priority">{{ t('lists.received') }}</UiBadge>
    <p v-if="wish.description" class="description collapsed">{{ wish.description }}</p>
    <button v-if="wish.description" class="description-toggle" type="button" aria-haspopup="dialog" @click="descriptionOpen = true">{{ t('wishes.showMore') }}</button>
    <ul v-if="wish.shop_links.length" class="shop-links" :aria-label="t('wishes.shopLinks')">
      <li v-for="(link, index) in wish.shop_links" :key="index"><a :href="link" target="_blank" rel="noopener noreferrer" @click="trackEvent('shop_link_opened')"><AppIcon name="link" /><span>{{ shopHost(link) }}</span><span class="sr-only"> · {{ t('wishes.newTab') }}</span></a></li>
    </ul>
    <div v-if="!readOnly || $slots.actions" class="wish-actions"><a v-if="!readOnly" class="edit-wish" :href="`/my/wishes/${encodeURIComponent(wish.id)}/edit${removeFromList ? `?list=${encodeURIComponent(windowListID)}` : ''}`">{{ t('wishes.edit') }}</a><UiButton v-if="!readOnly" variant="secondary" :loading="copying" @click="copy">{{ t('wishes.copy') }}</UiButton><slot name="actions" /></div>
    <p v-if="copyError" class="copy-error" role="alert">{{ t('wishes.copyError') }}</p>
    <Teleport to="body"><UiDialog v-model:open="descriptionOpen" :title="wish.title"><p class="full-description">{{ wish.description }}</p></UiDialog></Teleport>
  </UiCard>
</template>

<style scoped>
.wish-card { display: flex; flex-direction: column; min-width: 0; padding: var(--wx-space-4); overflow-wrap: anywhere; }
.wish-actions { display: flex; flex-wrap: wrap; gap: var(--wx-space-3); margin-top: auto; padding-top: var(--wx-space-4); }.wish-image:not(button) { cursor: default; }
.edit-wish { display: inline-flex; align-items: center; min-height: 44px; padding: 0 var(--wx-space-4); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); color: var(--wx-primary-pressed); text-decoration: none; font-weight: 600; font-size: var(--wx-text-sm); }.edit-wish:hover { background: var(--wx-lavender); }
.wish-heading { display: flex; align-items: flex-start; gap: var(--wx-space-3); }
.wish-image { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 56px; height: 64px; padding: 0; overflow: hidden; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-sm); color: var(--wx-primary); background: linear-gradient(140deg, var(--wx-background), var(--wx-lavender)); cursor: pointer; }
.wish-image:hover { border-color: var(--wx-secondary); }.wish-image:active { background: var(--wx-lavender); }
.wish-image img { width: 100%; height: 100%; object-fit: contain; background: var(--wx-surface); }
.wish-image .icon { width: 26px; height: 26px; }
.wish-summary { flex: 1; min-width: 0; padding-top: var(--wx-space-1); }
h2 { margin: 0; font-size: var(--wx-text-md); line-height: 1.5; font-weight: 650; }
.wish-price { margin: var(--wx-space-2) 0 0; font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; }
.delete-wish { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; height: 44px; margin: -4px -4px 0 0; border: 0; border-radius: var(--wx-radius-sm); color: var(--wx-text-muted); background: transparent; cursor: pointer; }
.delete-wish:hover { color: var(--wx-error); background: var(--wx-background); }
.delete-wish:active { color: var(--wx-error); background: var(--wx-lavender); }
.priority { align-self: flex-start; margin-top: var(--wx-space-4); color: var(--wx-text-muted); background: var(--wx-background); }
.priority.high { color: var(--wx-primary-pressed); background: var(--wx-lavender); }
.description { margin: var(--wx-space-3) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.65; white-space: pre-wrap; }
.description.collapsed { display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 3; overflow: hidden; }
.description-toggle { align-self: flex-start; min-height: 44px; padding: 0; border: 0; background: transparent; color: var(--wx-primary-pressed); font: inherit; font-size: var(--wx-text-sm); cursor: pointer; }
.full-description { margin: 0; white-space: pre-wrap; line-height: 1.65; }
.copy-error { color: var(--wx-error); font-size: var(--wx-text-sm); line-height: 1.5; }
.shop-links { display: flex; flex-wrap: wrap; gap: var(--wx-space-1) var(--wx-space-3); margin: var(--wx-space-3) 0 0; padding: 0; list-style: none; }
.shop-links li { min-width: 0; max-width: 100%; }
.shop-links a { display: inline-flex; align-items: center; gap: var(--wx-space-2); min-width: 44px; min-height: 44px; max-width: 100%; border-radius: var(--wx-radius-sm); color: var(--wx-primary-pressed); font-size: var(--wx-text-xs); text-decoration-thickness: 1px; text-underline-offset: 3px; }
.shop-links a .icon { width: 16px; height: 16px; }
.shop-links a span { min-width: 0; }
@media (min-width: 48rem) { .wish-card { padding: var(--wx-space-5); } .wish-image { flex-basis: 64px; } }
</style>
