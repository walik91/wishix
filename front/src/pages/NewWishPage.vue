<script setup>
import { computed, nextTick, ref, watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { trackEvent } from '../analytics.js'
import { useSession } from '../composables/useSession.js'
import { useUnsavedWish } from '../composables/useUnsavedWish.js'
import { createWish } from '../api/wishes.js'
import { useWishImport, validImportLink } from '../composables/useWishImport.js'
import { readWishCurrency, rememberWishCurrency, validateWishPrice } from '../wishPrice.js'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import UiToast from '../components/ui/UiToast.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import TextField from '../components/ui/TextField.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import WishImageInput from '../components/wishes/WishImageInput.vue'
import WishDiscardDialog from '../components/wishes/WishDiscardDialog.vue'
import WishPriceFields from '../components/wishes/WishPriceFields.vue'
import ListCheckboxes from '../components/lists/ListCheckboxes.vue'
import { listLists } from '../api/lists.js'

const { user, checking, unavailable, refresh } = useSession()
const title = ref('')
const description = ref('')
function limitDescription(event) {
  const limited = [...event.target.value].slice(0, 500).join('')
  event.target.value = limited
  description.value = limited
  markDirty('description')
}
const price = ref('')
const initialCurrency = computed(() => readWishCurrency(user.value?.id))
const currency = ref(initialCurrency.value)
watch(initialCurrency, code => { currency.value = code })
const priority = ref('medium')
const image = ref(null)
const imageInvalid = ref('')
let nextLinkID = 1
const links = ref([{ id: nextLinkID++, value: '' }])
const errors = ref({})
const saving = ref(false)
const saved = ref(false)
const failure = ref('')
const failureMessage = ref(null)
const form = ref(null)
const detailsOpen = ref(false)
const listPickerOpen = ref(false)
const availableLists = ref([]), selectedLists = ref([]), listsLoading = ref(true), listsError = ref(false)
const contextList = ref(new URLSearchParams(window.location.search).get('list') || '')
const initialList = ref('')
const contextMissing = computed(() => !!contextList.value && !listsLoading.value && !listsError.value && !availableLists.value.some(list => list.id === contextList.value))
const waitingForList = computed(() => !!contextList.value && (listsLoading.value || listsError.value || contextMissing.value))
const destination = computed(() => [
  t('my.allWishes'),
  ...availableLists.value.filter(list => selectedLists.value.includes(list.id)).map(list => list.title),
].join(', '))
const cancelURL = computed(() => contextList.value && !contextMissing.value ? `/my/lists/${encodeURIComponent(contextList.value)}` : '/my/wishes')
let listsInitialized = false
async function loadLists() {
  listsLoading.value = true; listsError.value = false
  try {
    availableLists.value = (await listLists()).lists
    if (!listsInitialized && contextList.value && availableLists.value.some(list => list.id === contextList.value)) {
      initialList.value = contextList.value
      selectedLists.value = [initialList.value]
    } else selectedLists.value = selectedLists.value.filter(id => availableLists.value.some(list => list.id === id))
    listsInitialized = true
  } catch (error) { if (error.status === 401) window.location.replace('/login'); else listsError.value = true }
  finally { listsLoading.value = false }
}
watch([user, checking, unavailable], ([account, pending, failed]) => { if (account && !pending && !failed) loadLists() }, { immediate: true })
const hasChanges = computed(() => {
  if (!user.value) return false
  const defaultList = availableLists.value.some(list => list.id === initialList.value) ? initialList.value : ''
  const listsChanged = defaultList ? selectedLists.value.length !== 1 || selectedLists.value[0] !== defaultList : selectedLists.value.length > 0
  return Boolean(title.value || description.value || price.value || currency.value !== initialCurrency.value || priority.value !== 'medium'
    || image.value || links.value.some(link => link.value) || listsChanged)
})
const { leaveDestination, leave } = useUnsavedWish(hasChanges)
const importNotice = ref(''), importNoticeSequence = ref(0)
const { activeId: importingId, markDirty, cancel: cancelImport, run: importLink, firstLinkBlur } = useWishImport(links, { title, description, price, currency, image }, applied => {
  for (const field of applied) delete errors.value[field]
}, result => {
  importNotice.value = result
  importNoticeSequence.value++
})
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
async function focusValidationError() {
  if (['image', 'description', 'price', 'currency', 'priority'].some(field => errors.value[field])) {
    detailsOpen.value = true
  }
  await nextTick()
  const target = form.value?.querySelector('[aria-invalid="true"]') || failureMessage.value
  target?.focus()
}
async function submit() {
  if (saving.value || saved.value || waitingForList.value) return
  const { price: normalizedPrice, currency: normalizedCurrency, errors: invalid } = validateWishPrice(price.value, currency.value)
  if (!title.value.trim() || [...title.value.trim()].length > 200) invalid.title = 'wish_title_invalid'
  if ([...description.value.trim()].length > 500) invalid.description = 'wish_description_invalid'
  for (const link of links.value) {
    if (link.value.trim() && !validImportLink(link.value.trim())) invalid[`link-${link.id}`] = 'wish_links_invalid'
  }
  if (imageInvalid.value) invalid.image = imageInvalid.value
  errors.value = invalid
  failure.value = ''
  if (Object.keys(invalid).length) {
    await focusValidationError()
    return
  }
  saving.value = true
  cancelImport()
  try {
    await createWish({
      title: title.value.trim(), description: description.value.trim(),
      price: normalizedPrice, currency: normalizedCurrency,
      priority: priority.value, shop_links: links.value.map(link => link.value.trim()).filter(Boolean),
      list_ids: selectedLists.value,
    }, image.value)
    trackEvent('wish_created')
    saved.value = true
    const returnURL = selectedLists.value.includes(contextList.value) ? cancelURL.value : '/my/wishes'
    leave(`${returnURL}?notice=wish-added`)
  } catch (error) {
    if (error.status === 401) { window.location.replace('/login'); return }
    const fields = { wish_title_invalid: 'title', wish_description_invalid: 'description', wish_price_invalid: 'price', wish_priority_invalid: 'priority' }
    if (fields[error.code]) errors.value = { [fields[error.code]]: error.code }
    else if (['wish_image_invalid', 'wish_image_too_large', 'wish_image_dimensions', 'wish_image_busy'].includes(error.code)) errors.value = { image: error.code }
    else failure.value = ['wish_links_invalid', 'list_selection_invalid'].includes(error.code) ? error.code : 'save_failed'
    if (failure.value === 'list_selection_invalid') listPickerOpen.value = true
  } finally { saving.value = false }
  if (!saved.value) await focusValidationError()
}
</script>

<template>
  <div class="new-wish-page" :aria-busy="checking">
    <div v-if="checking" class="page-loading"><UiSkeleton class="loading-title" /><UiSkeleton class="loading-form" /><p class="sr-only" role="status">{{ t('auth.checkingSession') }}</p></div>
    <UiCard v-else-if="unavailable" class="connection-error"><EmptyState icon="globe" :title="t('my.connectionTitle')" :description="t('auth.error.unavailable')"><UiButton @click="retry">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <template v-else-if="user">
      <div class="wish-heading">
        <UiButton class="back-link quiet-action" variant="ghost" :href="cancelURL" :disabled="saving || saved"><AppIcon name="arrow" /><span>{{ t(contextList && !contextMissing ? 'lists.backToList' : 'wishes.back') }}</span></UiButton>
        <h1>{{ t('my.addWish') }}</h1>
      </div>
      <UiCard class="form-card">
        <form ref="form" novalidate :aria-busy="saving || saved" @submit.prevent="submit">
          <fieldset :disabled="saving || saved">
            <legend class="sr-only">{{ t('my.addWish') }}</legend>
            <div class="links-section">
              <div v-for="(link, index) in links" :key="link.id" class="link-row">
                <TextField :id="`wish-link-${link.id}`" v-model="link.value" :label="index === 0 ? t('wishes.productLink') : `${t('wishes.shopLink')} ${index + 1}`" :hint="index === 0 ? t('wishes.import.hint') : ''" :error="errors[`link-${link.id}`] ? t('wishes.error.wish_links_invalid') : ''" type="url" placeholder="https://" maxlength="2048" autocomplete="off" @blur="index === 0 && firstLinkBlur()">
                  <button v-if="links.length > 1" class="remove-link" type="button" :aria-label="`${t('wishes.removeLink')} ${index + 1}`" @click="removeLink(link.id)"><AppIcon name="close" /></button>
                </TextField>
                <UiButton v-if="link.value.trim()" class="import-link quiet-action" variant="ghost" :aria-label="`${t('wishes.import.action')} ${index + 1}`" :disabled="!validImportLink(link.value.trim())" :loading="importingId === link.id" @click="importLink(link.id)">{{ t('wishes.import.action') }}</UiButton>
              </div>
              <p v-if="importingId !== null" class="sr-only" role="status">{{ t('wishes.import.loading') }}</p>
            </div>
            <TextField id="wish-title" v-model="title" :label="t('wishes.titleLabel')" :placeholder="t('wishes.titlePlaceholder')" :hint="t('wishes.titleHint')" :error="errors.title ? t(`wishes.error.${errors.title}`) : ''" required maxlength="400" autocomplete="off" @input="markDirty('title')" />
            <div class="save-section">
              <div id="wish-destination" class="destination" aria-live="polite">
                <p v-if="!waitingForList"><span>{{ t('wishes.saveDestination') }}</span> <strong>{{ destination }}</strong></p>
                <p v-else-if="listsLoading">{{ t('lists.loading') }}</p>
                <p v-else class="list-error" role="alert">{{ t(listsError ? 'lists.error.unavailable' : 'lists.error.list_not_found') }}</p>
              </div>
              <p v-if="failure" ref="failureMessage" class="operation-error" role="alert" tabindex="-1">{{ t(`wishes.error.${failure}`) }}</p>
              <div v-if="contextList && (contextMissing || listsError)" class="form-actions">
                <UiButton v-if="listsError" variant="secondary" @click="loadLists">{{ t('checkAgain') }}</UiButton>
                <UiButton variant="secondary" @click="contextList = ''">{{ t('lists.continueInCollection') }}</UiButton>
              </div>
              <div class="form-actions">
                <UiButton type="submit" :loading="saving || saved" :disabled="waitingForList" aria-describedby="wish-destination">{{ t('wishes.save') }}</UiButton>
                <UiButton :href="cancelURL" class="quiet-action" variant="ghost" :disabled="saving || saved">{{ t('wishes.cancel') }}</UiButton>
              </div>
            </div>
            <div class="optional-fields">
              <div>
                <UiButton class="quiet-action" variant="ghost" :disabled="links.length >= 10" @click="addLink"><AppIcon name="plus" />{{ t('wishes.addLink') }}</UiButton>
              </div>
              <div>
                <UiButton class="quiet-action" variant="ghost" :aria-expanded="detailsOpen" aria-controls="wish-details" @click="detailsOpen = !detailsOpen">
                  <AppIcon name="chevron" />{{ t('wishes.addDetails') }}
                </UiButton>
                <div v-show="detailsOpen" id="wish-details" class="details-fields">
                  <WishImageInput id="wish-image" v-model="image" compact :error="errors.image || ''" :disabled="saving || saved" @validation="imageInvalid = $event; delete errors.image" @manual-change="markDirty('image')" />
                  <TextField id="wish-description" v-model="description" multiline rows="3" class="description-field" :label="t('wishes.descriptionLabel')" :hint="t('wishes.descriptionHint')" :error="errors.description ? t(`wishes.error.${errors.description}`) : ''" @input="limitDescription" />
                  <WishPriceFields v-model:price="price" v-model:currency="currency" :errors="errors"
                    @price-input="delete errors.price; markDirty('price')"
                    @currency-input="delete errors.currency; markDirty('currency')"
                    @currency-selected="rememberWishCurrency(user.id, $event)" />
                  <div class="select-field">
                    <label for="wish-priority">{{ t('wishes.priorityLabel') }}</label>
                    <select id="wish-priority" v-model="priority" :aria-invalid="errors.priority ? 'true' : undefined" :aria-describedby="errors.priority ? 'priority-error' : undefined"><option v-for="value in ['low', 'medium', 'high']" :key="value" :value="value">{{ t(`wishes.priority.${value}`) }}</option></select>
                    <p v-if="errors.priority" id="priority-error" class="field-error">{{ t(`wishes.error.${errors.priority}`) }}</p>
                  </div>
                </div>
              </div>
              <div>
                <UiButton class="quiet-action" variant="ghost" :aria-expanded="listPickerOpen" aria-controls="wish-lists" @click="listPickerOpen = !listPickerOpen">
                  <AppIcon name="chevron" />{{ t('lists.wishLists') }}
                </UiButton>
                <div v-show="listPickerOpen" id="wish-lists">
                  <p class="list-muted">{{ t('lists.newWishHint') }}</p>
                  <p v-if="listsLoading" class="list-muted" role="status">{{ t('lists.loading') }}</p>
                  <template v-else-if="listsError">
                    <p class="list-error" role="alert">{{ t('lists.error.unavailable') }}</p>
                    <UiButton variant="secondary" @click="loadLists">{{ t('checkAgain') }}</UiButton>
                  </template>
                  <ListCheckboxes v-else-if="availableLists.length" v-model="selectedLists" :lists="availableLists" :disabled="saving || saved" />
                  <p v-else class="list-muted">{{ t('lists.noLists') }}</p>
                  <UiButton v-if="failure === 'list_selection_invalid' && !listsLoading && !listsError" variant="secondary" @click="loadLists">{{ t('lists.reload') }}</UiButton>
                </div>
              </div>
            </div>
          </fieldset>
        </form>
      </UiCard>
      <UiToast v-if="importNotice" :key="importNoticeSequence" :duration="importNotice === 'ok' ? 3000 : 6000" :variant="importNotice === 'ok' ? 'success' : 'info'" @dismiss="importNotice = ''">{{ t(`wishes.import.${importNotice}`) }}</UiToast>
    </template>
  </div>
  <WishDiscardDialog :open="!!leaveDestination" @update:open="leaveDestination = ''" @discard="leave(leaveDestination)" />
</template>

<style scoped>
.new-wish-page { max-width: 760px; margin-inline: auto; min-width: 0; overflow-wrap: anywhere; }
.wish-heading { margin-bottom: var(--wx-space-4); }
.quiet-action { min-height: 44px; gap: var(--wx-space-2); padding: var(--wx-space-2); }
.back-link { margin-bottom: var(--wx-space-2); }
.back-link .icon { transform: rotate(180deg); }
h1 { margin: 0; font-size: var(--wx-text-xl); line-height: 1.2; font-weight: 750; letter-spacing: -.03em; }
.form-card {
  --wish-field-border: color-mix(in srgb, var(--wx-text-muted) 65%, var(--wx-border));
  padding: var(--wx-space-4);
}
fieldset { display: grid; gap: var(--wx-space-4); min-width: 0; margin: 0; padding: 0; border: 0; }
fieldset:disabled { opacity: .7; }
.links-section :deep(.field) { gap: var(--wx-space-1); }
.links-section :deep(label) { color: var(--wx-text-muted); }
:deep(label[for="wish-title"]) { font-size: var(--wx-text-md); font-weight: 700; }
:deep(.control:not(.invalid)), :deep(select:not([aria-invalid="true"])) { border-color: var(--wish-field-border); }
:deep(.control:not(.invalid):hover), :deep(select:not([aria-invalid="true"]):hover) { border-color: var(--wx-text-muted); }
:deep(.control:has(> :is(input, textarea):focus-visible)), :deep(select:focus-visible) {
  outline: 2px solid var(--wx-focus);
  outline-offset: 2px;
  box-shadow: none;
}
:deep(.control.invalid), select[aria-invalid="true"] { border-color: var(--wx-error); }
:deep(textarea.description-field) { min-height: 88px; }
.select-field { display: grid; gap: var(--wx-space-2); }
label { font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; }
select { width: 100%; min-width: 0; min-height: 48px; padding: var(--wx-space-3); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); color: var(--wx-text); background: var(--wx-surface); font-size: var(--wx-text-md); }
.links-section, .save-section, .optional-fields, .details-fields { display: grid; gap: var(--wx-space-3); min-width: 0; }
.optional-fields { gap: 0; padding-top: var(--wx-space-2); border-top: 1px solid var(--wx-border); }
.details-fields { gap: var(--wx-space-5); margin-top: var(--wx-space-4); }
.optional-fields [aria-expanded="true"] .icon { transform: rotate(180deg); }
.link-row { display: grid; gap: var(--wx-space-2); min-width: 0; }
.import-link { justify-self: start; }
.remove-link { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; min-height: 46px; border: 0; border-inline-start: 1px solid var(--wx-border); border-start-end-radius: var(--wx-radius-md); border-end-end-radius: var(--wx-radius-md); color: var(--wx-text-muted); background: var(--wx-surface); cursor: pointer; }
.remove-link:hover:not(:disabled) { color: var(--wx-error); background: var(--wx-background); border-color: var(--wx-secondary); }
.remove-link:active:not(:disabled) { background: var(--wx-lavender); }
.form-actions { display: flex; flex-wrap: wrap; gap: var(--wx-space-3); }
.destination p { display: flex; flex-wrap: wrap; gap: var(--wx-space-1) var(--wx-space-2); margin: 0; font-size: var(--wx-text-sm); line-height: 1.5; }
.destination span { color: var(--wx-text-muted); }
.operation-error, .field-error { margin: 0; color: var(--wx-error); font-size: var(--wx-text-sm); line-height: 1.6; }
.page-loading { display: grid; gap: var(--wx-space-6); }.loading-title { height: 32px; width: 70%; }.loading-form { height: 600px; border-radius: var(--wx-radius-card); }.connection-error { padding: var(--wx-space-10) var(--wx-space-5); }
@media (max-width: 47.99rem) {
  .form-actions .primary { flex: 1; }
  .wish-heading { display: flex; align-items: center; gap: var(--wx-space-2); margin-bottom: var(--wx-space-3); }
  .back-link { min-width: 44px; margin: 0; }
  .back-link span { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
}
@media (max-width: 24rem) {
  .form-card { padding: var(--wx-space-2) var(--wx-space-3); }
  fieldset { gap: var(--wx-space-2); }
  .save-section { gap: var(--wx-space-1); }
  .form-actions { gap: var(--wx-space-2); }
  .form-actions .button { padding-inline: var(--wx-space-2); }
}
@media (min-width: 48rem) {
  h1 { font-size: var(--wx-text-2xl); }
  .form-card { padding: var(--wx-space-6); }
  fieldset { gap: var(--wx-space-5); }
}
</style>
