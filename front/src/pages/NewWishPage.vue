<script setup>
import { computed, nextTick, ref, watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { trackEvent } from '../analytics.js'
import { useSession } from '../composables/useSession.js'
import { createWish } from '../api/wishes.js'
import { useWishImport, validImportLink } from '../composables/useWishImport.js'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import UiToast from '../components/ui/UiToast.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import TextField from '../components/ui/TextField.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import WishImageInput from '../components/wishes/WishImageInput.vue'
import ListCheckboxes from '../components/lists/ListCheckboxes.vue'
import { listLists } from '../api/lists.js'

const { user, checking, unavailable, refresh } = useSession()
const title = ref('')
const description = ref('')
const price = ref('')
const currency = ref('')
const priority = ref('medium')
const image = ref(null)
const imageInvalid = ref('')
let nextLinkID = 1
const links = ref([{ id: nextLinkID++, value: '' }])
const errors = ref({})
const saving = ref(false)
const saved = ref(false)
const failure = ref('')
const form = ref(null)
const availableLists = ref([]), selectedLists = ref([]), listsLoading = ref(true), listsError = ref(false)
const contextList = ref(new URLSearchParams(window.location.search).get('list') || '')
const contextMissing = computed(() => !!contextList.value && !listsLoading.value && !listsError.value && !availableLists.value.some(list => list.id === contextList.value))
let listsInitialized = false
async function loadLists() {
  listsLoading.value = true; listsError.value = false
  try {
    availableLists.value = (await listLists()).lists
    if (!listsInitialized && contextList.value && availableLists.value.some(list => list.id === contextList.value)) selectedLists.value = [contextList.value]
    else selectedLists.value = selectedLists.value.filter(id => availableLists.value.some(list => list.id === id))
    listsInitialized = true
  } catch (error) { if (error.status === 401) window.location.replace('/login'); else listsError.value = true }
  finally { listsLoading.value = false }
}
watch([user, checking, unavailable], ([account, pending, failed]) => { if (account && !pending && !failed) loadLists() }, { immediate: true })
const importNotice = ref(''), importNoticeSequence = ref(0)
const { activeId: importingId, markDirty, cancel: cancelImport, run: importLink, firstLinkBlur } = useWishImport(links, { title, description, price, currency, image }, applied => {
  for (const field of applied) delete errors.value[field]
}, result => {
  importNotice.value = result
  importNoticeSequence.value++
})
const currencies = ['EUR', 'USD', 'GBP', 'MDL', 'RON', 'UAH', 'RUB', 'CHF', 'CAD', 'AUD', 'JPY']
watchEffect(() => { document.title = `${t('my.addWish')} · ${t('appName')}` })

async function retry() {
  await refresh()
  if (!user.value && !unavailable.value) window.location.replace('/login')
}
async function addLink() {
  if (links.value.length >= 10) return
  const id = nextLinkID++
  links.value.push({ id, value: '' })
  await nextTick()
  document.getElementById(`wish-link-${id}`)?.focus()
}
async function removeLink(id) {
  const index = links.value.findIndex(link => link.id === id)
  links.value = links.value.filter(link => link.id !== id)
  await nextTick()
  document.getElementById(`wish-link-${links.value[Math.min(index, links.value.length - 1)].id}`)?.focus()
}
function validLink(raw) {
  return validImportLink(raw)
}
async function submit() {
  if (saving.value || saved.value) return
  if (contextList.value && (listsLoading.value || listsError.value || contextMissing.value)) return
  const invalid = {}
  const normalizedPrice = price.value.trim().replace(',', '.')
  if (!title.value.trim() || [...title.value.trim()].length > 200) invalid.title = 'wish_title_invalid'
  if ([...description.value.trim()].length > 5000) invalid.description = 'wish_description_invalid'
  if (normalizedPrice && !/^(0|[1-9][0-9]{0,9})(\.[0-9]{1,2})?$/.test(normalizedPrice)) invalid.price = 'wish_price_invalid'
  if (normalizedPrice && !/^[A-Z]{3}$/.test(currency.value.trim().toUpperCase())) invalid.currency = 'wish_currency_invalid'
  for (const link of links.value) {
    if (link.value.trim() && !validLink(link.value.trim())) invalid[`link-${link.id}`] = 'wish_links_invalid'
  }
  errors.value = invalid
  if (imageInvalid.value) invalid.image = imageInvalid.value
  failure.value = ''
  if (Object.keys(invalid).length) {
    await nextTick()
    form.value.querySelector('[aria-invalid="true"]')?.focus()
    return
  }
  saving.value = true
  cancelImport()
  try {
    await createWish({
      title: title.value.trim(), description: description.value.trim(),
      price: normalizedPrice || null, currency: normalizedPrice ? currency.value.trim().toUpperCase() : '',
      priority: priority.value, shop_links: links.value.map(link => link.value.trim()).filter(Boolean),
      list_ids: selectedLists.value,
    }, image.value)
    trackEvent('wish_created')
    saved.value = true
    window.location.assign(contextList.value && selectedLists.value.includes(contextList.value) ? `/my/lists/${contextList.value}?notice=wish-added` : '/my/wishes?notice=wish-added')
  } catch (error) {
    if (error.status === 401) { window.location.replace('/login'); return }
    const fields = { wish_title_invalid: 'title', wish_description_invalid: 'description', wish_price_invalid: 'price', wish_priority_invalid: 'priority' }
    if (fields[error.code]) errors.value = { [fields[error.code]]: error.code }
    else if (['wish_image_invalid', 'wish_image_too_large', 'wish_image_dimensions', 'wish_image_busy'].includes(error.code)) errors.value = { image: error.code }
    else failure.value = ['wish_links_invalid', 'list_selection_invalid'].includes(error.code) ? error.code : 'save_failed'
    await nextTick()
    form.value.querySelector('[aria-invalid="true"], [role="alert"]')?.focus()
  } finally { saving.value = false }
}
</script>

<template>
  <div class="new-wish-page" :aria-busy="checking">
    <div v-if="checking" class="page-loading"><UiSkeleton class="loading-title" /><UiSkeleton class="loading-form" /><p class="sr-only" role="status">{{ t('auth.checkingSession') }}</p></div>
    <UiCard v-else-if="unavailable" class="connection-error"><EmptyState icon="globe" :title="t('my.connectionTitle')" :description="t('auth.error.unavailable')"><UiButton @click="retry">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <template v-else-if="user">
      <UiButton class="back-link" variant="secondary" href="/my/wishes" :disabled="saving || saved"><AppIcon name="arrow" />{{ t('wishes.back') }}</UiButton>
      <header class="page-heading"><h1>{{ t('my.addWish') }}</h1><p>{{ t('wishes.formDescription') }}</p></header>
      <UiCard class="form-card">
        <form ref="form" novalidate :aria-busy="saving || saved" @submit.prevent="submit">
          <fieldset :disabled="saving || saved">
            <legend class="sr-only">{{ t('my.addWish') }}</legend>
            <section class="links-section" aria-labelledby="links-title">
              <h2 id="links-title">{{ t('wishes.shopLinks') }}</h2><p class="links-hint">{{ t('wishes.linksHint') }}</p>
              <div v-for="(link, index) in links" :key="link.id" class="link-row">
                <TextField :id="`wish-link-${link.id}`" v-model="link.value" :label="`${t('wishes.shopLink')} ${index + 1}`" visually-hidden-label :error="errors[`link-${link.id}`] ? t('wishes.error.wish_links_invalid') : ''" type="url" placeholder="https://" maxlength="2048" autocomplete="off" @blur="index === 0 && firstLinkBlur()" />
                <button class="link-icon import-link" type="button" :aria-label="`${t('wishes.import.action')} ${index + 1}`" :title="t('wishes.import.action')" :disabled="!validImportLink(link.value.trim()) || importingId === link.id" :aria-busy="importingId === link.id || undefined" @click="importLink(link.id)"><span v-if="importingId === link.id" class="spinner" aria-hidden="true"></span><AppIcon v-else name="download" /></button>
                <button v-if="links.length > 1" class="link-icon remove-link" type="button" :aria-label="`${t('wishes.removeLink')} ${index + 1}`" @click="removeLink(link.id)"><AppIcon name="close" /></button>
              </div>
              <div class="link-actions"><UiButton variant="secondary" :disabled="links.length >= 10" @click="addLink"><AppIcon name="plus" />{{ t('wishes.addLink') }}</UiButton></div>
              <p class="import-hint">{{ t('wishes.import.hint') }}</p>
              <p v-if="importingId !== null" class="sr-only" role="status">{{ t('wishes.import.loading') }}</p>
            </section>
            <TextField id="wish-title" v-model="title" :label="t('wishes.titleLabel')" :placeholder="t('wishes.titlePlaceholder')" :hint="t('wishes.titleHint')" :error="errors.title ? t(`wishes.error.${errors.title}`) : ''" required maxlength="400" autocomplete="off" @input="markDirty('title')" />
            <WishImageInput id="wish-image" v-model="image" :error="errors.image || ''" :disabled="saving || saved" @validation="imageInvalid = $event; delete errors.image" @manual-change="markDirty('image')" />
            <TextField id="wish-description" v-model="description" multiline rows="3" class="description-field" :label="t('wishes.descriptionLabel')" :hint="t('wishes.descriptionHint')" :error="errors.description ? t(`wishes.error.${errors.description}`) : ''" maxlength="10000" @input="markDirty('description')" />
            <div class="price-fields">
              <TextField id="wish-price" v-model="price" :label="t('wishes.priceLabel')" :hint="t('wishes.priceHint')" :error="errors.price ? t(`wishes.error.${errors.price}`) : ''" inputmode="decimal" placeholder="0.00" maxlength="13" @input="markDirty('price')" />
              <TextField id="wish-currency" v-model="currency" :label="t('wishes.currencyLabel')" :hint="t('wishes.currencyHint')" :error="errors.currency ? t(`wishes.error.${errors.currency}`) : ''" list="wish-currencies" maxlength="3" placeholder="USD" autocomplete="off" @input="markDirty('currency')" />
              <datalist id="wish-currencies"><option v-for="code in currencies" :key="code" :value="code" /></datalist>
            </div>
            <div class="select-field">
              <label for="wish-priority">{{ t('wishes.priorityLabel') }}</label>
              <select id="wish-priority" v-model="priority" :aria-invalid="errors.priority ? 'true' : undefined" :aria-describedby="errors.priority ? 'priority-error' : undefined"><option v-for="value in ['low', 'medium', 'high']" :key="value" :value="value">{{ t(`wishes.priority.${value}`) }}</option></select>
              <p v-if="errors.priority" id="priority-error" class="field-error">{{ t(`wishes.error.${errors.priority}`) }}</p>
            </div>
            <section aria-labelledby="wish-lists-title"><h2 id="wish-lists-title">{{ t('lists.wishLists') }}</h2><p class="list-muted">{{ t('lists.newWishHint') }}</p><p v-if="listsLoading" role="status">{{ t('lists.loading') }}</p><template v-else-if="listsError"><p class="list-error" role="alert">{{ t('lists.error.unavailable') }}</p><UiButton variant="secondary" @click="loadLists">{{ t('checkAgain') }}</UiButton></template><ListCheckboxes v-else-if="availableLists.length" v-model="selectedLists" :lists="availableLists" :disabled="saving || saved" /><p v-else class="list-muted">{{ t('lists.noLists') }}</p></section>
          </fieldset>
          <p v-if="failure" class="operation-error" role="alert" tabindex="-1">{{ t(`wishes.error.${failure}`) }}</p>
          <p v-if="contextMissing" class="list-error" role="alert">{{ t('lists.error.list_not_found') }}</p><UiButton v-if="contextList && (contextMissing || listsError)" variant="secondary" :disabled="saving || saved" @click="contextList = ''">{{ t('lists.continueInCollection') }}</UiButton><UiButton v-if="failure === 'list_selection_invalid'" variant="secondary" :disabled="saving || saved" @click="loadLists">{{ t('lists.reload') }}</UiButton>
          <div class="form-actions"><UiButton type="submit" :loading="saving || saved" :disabled="!!contextList && (listsLoading || listsError || contextMissing)">{{ t('wishes.save') }}</UiButton><UiButton :href="contextList && !contextMissing ? `/my/lists/${contextList}` : '/my/wishes'" variant="secondary" :disabled="saving || saved">{{ t('wishes.cancel') }}</UiButton></div>
        </form>
      </UiCard>
      <p class="privacy-note"><AppIcon name="lock" /><span>{{ t('wishes.privateNote') }}</span></p>
      <UiToast v-if="importNotice" :key="importNoticeSequence" :duration="importNotice === 'ok' ? 3000 : 6000" :variant="importNotice === 'ok' ? 'success' : 'info'" @dismiss="importNotice = ''">{{ t(`wishes.import.${importNotice}`) }}</UiToast>
    </template>
  </div>
</template>

<style scoped>
.new-wish-page { max-width: 760px; margin-inline: auto; min-width: 0; overflow-wrap: anywhere; }
.back-link { margin-bottom: var(--wx-space-6); }.back-link .icon { transform: rotate(180deg); }
.page-heading { margin-bottom: var(--wx-space-6); }h1 { margin: 0; font-size: 26px; line-height: 1.3; font-weight: 750; letter-spacing: -.03em; }
.page-heading p { margin: var(--wx-space-3) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.65; }
.form-card { padding: var(--wx-space-5) var(--wx-space-4); }
fieldset { display: grid; gap: var(--wx-space-5); min-width: 0; margin: 0; padding: 0; border: 0; }fieldset:disabled { opacity: .7; }
:deep(textarea.description-field) { min-height: 88px; }
.price-fields { display: grid; gap: var(--wx-space-4); min-width: 0; }.select-field { display: grid; gap: var(--wx-space-2); }
label, h2 { margin: 0; font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; }
select { width: 100%; min-width: 0; min-height: 48px; padding: var(--wx-space-3); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); color: var(--wx-text); background: var(--wx-surface); font-size: var(--wx-text-md); }
select:hover { border-color: var(--wx-secondary); }
.links-section { min-width: 0; padding-bottom: var(--wx-space-5); border-bottom: 1px solid var(--wx-border); }.links-hint { margin: var(--wx-space-1) 0 var(--wx-space-3); color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.65; }
.link-row { display: flex; align-items: flex-start; gap: var(--wx-space-2); margin-bottom: var(--wx-space-2); }.link-row > .field { flex: 1; }
.link-actions { display: flex; flex-wrap: wrap; gap: var(--wx-space-3); }.link-actions > * { max-width: 100%; }
.import-hint { margin: var(--wx-space-3) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.65; }
.link-icon { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; min-height: 48px; margin-top: 0; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); color: var(--wx-primary); background: var(--wx-surface); cursor: pointer; }
.link-icon:hover:not(:disabled) { background: var(--wx-background); border-color: var(--wx-secondary); }.link-icon:active:not(:disabled) { background: var(--wx-lavender); }.link-icon:focus-visible { outline: none; box-shadow: var(--wx-focus-ring); }.link-icon:disabled { opacity: .5; cursor: not-allowed; }.link-icon[aria-busy='true'] { cursor: wait; }.remove-link { color: var(--wx-text-muted); }.remove-link:hover:not(:disabled) { color: var(--wx-error); }
.spinner { width: 18px; height: 18px; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; animation: import-spin 700ms linear infinite; }@keyframes import-spin { to { transform: rotate(360deg); } }@media (prefers-reduced-motion: reduce) { .spinner { animation: none; } }
.form-actions { display: flex; flex-direction: column; gap: var(--wx-space-3); margin-top: var(--wx-space-8); padding-top: var(--wx-space-6); border-top: 1px solid var(--wx-border); }
.privacy-note { display: flex; align-items: flex-start; justify-content: center; gap: var(--wx-space-2); margin: var(--wx-space-6) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.65; }.privacy-note .icon { width: 16px; height: 18px; }
.operation-error, .field-error { color: var(--wx-error); font-size: var(--wx-text-sm); line-height: 1.6; }
.page-loading { display: grid; gap: var(--wx-space-6); }.loading-title { height: 32px; width: 70%; }.loading-form { height: 600px; border-radius: var(--wx-radius-card); }.connection-error { padding: var(--wx-space-10) var(--wx-space-5); }
@media (min-width: 48rem) { h1 { font-size: var(--wx-text-2xl); }.form-card { padding: var(--wx-space-8); }.price-fields { grid-template-columns: minmax(0, 2fr) minmax(0, 1fr); }.form-actions { flex-direction: row; } }
</style>
