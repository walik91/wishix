<script setup>
import { nextTick, ref, watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { getWish, updateWish } from '../api/wishes.js'
import { validImportLink } from '../composables/useWishImport.js'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import TextField from '../components/ui/TextField.vue'
import AppIcon from '../components/ui/AppIcon.vue'

const props = defineProps({ id: { type: String, required: true } })
const { user, checking, unavailable, refresh } = useSession()
const title = ref('')
const description = ref('')
const price = ref('')
const currency = ref('')
const priority = ref('medium')
let nextLinkID = 1
const links = ref([{ id: nextLinkID++, value: '' }])
const errors = ref({})
const saving = ref(false)
const saved = ref(false)
const failure = ref('')
const form = ref(null)
const loading = ref(true), loadError = ref(''), editVersion = ref('')
const listID = new URLSearchParams(window.location.search).get('list') || ''
const returnPath = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(listID) ? '/my/lists/' + listID : '/my/wishes'
async function loadWish() {
  loading.value = true; loadError.value = ''; failure.value = ''; errors.value = {}
  try {
    const { wish } = await getWish(props.id)
    editVersion.value = wish.edit_version
    title.value = wish.title; description.value = wish.description; price.value = wish.price || ''
    currency.value = wish.currency; priority.value = wish.priority
    links.value = (wish.shop_links.length ? wish.shop_links : ['']).map(value => ({ id: nextLinkID++, value }))
  } catch (error) {
    if (error.status === 401) window.location.replace('/login')
    else loadError.value = error.code === 'wish_not_found' ? 'not_found' : 'load_failed'
  } finally { loading.value = false }
}
watch([user, checking, unavailable], ([account, pending, failed]) => { if (account && !pending && !failed) loadWish() }, { immediate: true })
const currencies = ['EUR', 'USD', 'GBP', 'MDL', 'RON', 'UAH', 'RUB', 'CHF', 'CAD', 'AUD', 'JPY']
watchEffect(() => { document.title = `${t('wishes.edit')} · ${t('appName')}` })

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
  failure.value = ''
  if (Object.keys(invalid).length) {
    await nextTick()
    form.value.querySelector('[aria-invalid="true"]')?.focus()
    return
  }
  saving.value = true
  try {
    await updateWish(props.id, {
      title: title.value.trim(), description: description.value.trim(),
      price: normalizedPrice || null, currency: normalizedPrice ? currency.value.trim().toUpperCase() : '',
      priority: priority.value, shop_links: links.value.map(link => link.value.trim()).filter(Boolean),
      edit_version: editVersion.value,
    })
    saved.value = true
    window.location.assign(`${returnPath}?notice=wish-updated`)
  } catch (error) {
    if (error.status === 401) { window.location.replace('/login'); return }
    const fields = { wish_title_invalid: 'title', wish_description_invalid: 'description', wish_price_invalid: 'price', wish_priority_invalid: 'priority' }
    if (fields[error.code]) errors.value = { [fields[error.code]]: error.code }
    else failure.value = ['wish_links_invalid', 'wish_not_found', 'wish_conflict'].includes(error.code) ? error.code : 'save_failed'
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
      <div v-if="loading" class="page-loading"><UiSkeleton class="loading-title" /><UiSkeleton class="loading-form" /><p class="sr-only" role="status">{{ t('wishes.loading') }}</p></div>
      <UiCard v-else-if="loadError" class="connection-error"><EmptyState :title="t(`wishes.editError.${loadError}`)"><UiButton v-if="loadError !== 'not_found'" @click="loadWish">{{ t('checkAgain') }}</UiButton><UiButton :href="returnPath" variant="secondary">{{ t('wishes.back') }}</UiButton></EmptyState></UiCard>
      <template v-else>
      <UiButton class="back-link" variant="secondary" :href="returnPath" :disabled="saving || saved"><AppIcon name="arrow" />{{ t('wishes.back') }}</UiButton>
      <header class="page-heading"><h1>{{ t('wishes.edit') }}</h1><p>{{ t('wishes.editDescription') }}</p></header>
      <UiCard class="form-card">
        <form ref="form" novalidate :aria-busy="saving || saved" @submit.prevent="submit">
          <fieldset :disabled="saving || saved">
            <legend class="sr-only">{{ t('wishes.edit') }}</legend>
            <section class="links-section" aria-labelledby="links-title">
              <h2 id="links-title">{{ t('wishes.shopLinks') }}</h2><p class="links-hint">{{ t('wishes.linksHint') }}</p>
              <div v-for="(link, index) in links" :key="link.id" class="link-row">
                <TextField :id="`wish-link-${link.id}`" v-model="link.value" :label="`${t('wishes.shopLink')} ${index + 1}`" :error="errors[`link-${link.id}`] ? t('wishes.error.wish_links_invalid') : ''" type="url" placeholder="https://" maxlength="2048" autocomplete="off" />
                <button v-if="links.length > 1" class="remove-link" type="button" :aria-label="`${t('wishes.removeLink')} ${index + 1}`" @click="removeLink(link.id)"><AppIcon name="close" /></button>
              </div>
              <div class="link-actions"><UiButton variant="secondary" :disabled="links.length >= 10" @click="addLink"><AppIcon name="plus" />{{ t('wishes.addLink') }}</UiButton></div>
            </section>
            <TextField id="wish-title" v-model="title" :label="t('wishes.titleLabel')" :placeholder="t('wishes.titlePlaceholder')" :hint="t('wishes.titleHint')" :error="errors.title ? t(`wishes.error.${errors.title}`) : ''" required maxlength="400" autocomplete="off" />
            <TextField id="wish-description" v-model="description" multiline rows="4" :label="t('wishes.descriptionLabel')" :hint="t('wishes.descriptionHint')" :error="errors.description ? t(`wishes.error.${errors.description}`) : ''" maxlength="10000" />
            <div class="price-fields">
              <TextField id="wish-price" v-model="price" :label="t('wishes.priceLabel')" :hint="t('wishes.priceHint')" :error="errors.price ? t(`wishes.error.${errors.price}`) : ''" inputmode="decimal" placeholder="0.00" maxlength="13" />
              <TextField id="wish-currency" v-model="currency" :label="t('wishes.currencyLabel')" :hint="t('wishes.currencyHint')" :error="errors.currency ? t(`wishes.error.${errors.currency}`) : ''" list="wish-currencies" maxlength="3" placeholder="USD" autocomplete="off" />
              <datalist id="wish-currencies"><option v-for="code in currencies" :key="code" :value="code" /></datalist>
            </div>
            <div class="select-field">
              <label for="wish-priority">{{ t('wishes.priorityLabel') }}</label>
              <select id="wish-priority" v-model="priority" :aria-invalid="errors.priority ? 'true' : undefined" :aria-describedby="errors.priority ? 'priority-error' : undefined"><option v-for="value in ['low', 'medium', 'high']" :key="value" :value="value">{{ t(`wishes.priority.${value}`) }}</option></select>
              <p v-if="errors.priority" id="priority-error" class="field-error">{{ t(`wishes.error.${errors.priority}`) }}</p>
            </div>
          </fieldset>
          <p v-if="failure" class="operation-error" role="alert" tabindex="-1">{{ t(`wishes.error.${failure}`) }}</p>
          <UiButton v-if="failure === 'wish_conflict'" variant="secondary" @click="loadWish">{{ t('wishes.reloadEdit') }}</UiButton>
          <div class="form-actions"><UiButton type="submit" :loading="saving || saved" >{{ t('wishes.save') }}</UiButton><UiButton :href="returnPath" variant="secondary" :disabled="saving || saved">{{ t('wishes.cancel') }}</UiButton></div>
        </form>
      </UiCard>
      <p class="privacy-note"><AppIcon name="lock" /><span>{{ t('wishes.privateNote') }}</span></p>
      </template>
    </template>
  </div>
</template>

<style scoped>
.new-wish-page { max-width: 760px; margin-inline: auto; min-width: 0; overflow-wrap: anywhere; }
.back-link { margin-bottom: var(--wx-space-6); }.back-link .icon { transform: rotate(180deg); }
.page-heading { margin-bottom: var(--wx-space-6); }h1 { margin: 0; font-size: 26px; line-height: 1.3; font-weight: 750; letter-spacing: -.03em; }
.page-heading p { margin: var(--wx-space-3) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-sm); line-height: 1.65; }
.form-card { padding: var(--wx-space-5) var(--wx-space-4); }
fieldset { display: grid; gap: var(--wx-space-6); min-width: 0; margin: 0; padding: 0; border: 0; }fieldset:disabled { opacity: .7; }
.price-fields { display: grid; gap: var(--wx-space-5); min-width: 0; }.select-field { display: grid; gap: var(--wx-space-2); }
label, h2 { margin: 0; font-size: var(--wx-text-sm); font-weight: 600; line-height: 1.5; }
select { width: 100%; min-width: 0; min-height: 48px; padding: var(--wx-space-3); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); color: var(--wx-text); background: var(--wx-surface); font-size: var(--wx-text-md); }
select:hover { border-color: var(--wx-secondary); }
.links-section { min-width: 0; padding-bottom: var(--wx-space-6); border-bottom: 1px solid var(--wx-border); }.links-hint { margin: var(--wx-space-2) 0 var(--wx-space-5); color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.65; }
.link-row { display: flex; align-items: flex-start; gap: var(--wx-space-2); margin-bottom: var(--wx-space-4); }.link-row > .field { flex: 1; }
.link-actions { display: flex; flex-wrap: wrap; gap: var(--wx-space-3); }.link-actions > * { max-width: 100%; }
.import-hint { margin: var(--wx-space-3) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.65; }
.import-status { margin-top: var(--wx-space-4); padding: var(--wx-space-3) var(--wx-space-4); border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); background: var(--wx-background); color: var(--wx-text); font-size: var(--wx-text-sm); line-height: 1.65; }.import-status p { margin: 0; }.import-status .import-shop { margin-top: var(--wx-space-2); color: var(--wx-text-muted); font-size: var(--wx-text-xs); }.import-status a { display: inline-flex; align-items: center; min-height: 44px; color: var(--wx-primary); }
.remove-link { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; min-height: 48px; margin-top: 29px; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-md); color: var(--wx-text-muted); background: var(--wx-surface); cursor: pointer; }.remove-link:hover { color: var(--wx-error); background: var(--wx-background); }.remove-link:disabled { cursor: wait; }
.form-actions { display: flex; flex-direction: column; gap: var(--wx-space-3); margin-top: var(--wx-space-8); padding-top: var(--wx-space-6); border-top: 1px solid var(--wx-border); }
.privacy-note { display: flex; align-items: flex-start; justify-content: center; gap: var(--wx-space-2); margin: var(--wx-space-6) 0 0; color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.65; }.privacy-note .icon { width: 16px; height: 18px; }
.operation-error, .field-error { color: var(--wx-error); font-size: var(--wx-text-sm); line-height: 1.6; }
.page-loading { display: grid; gap: var(--wx-space-6); }.loading-title { height: 32px; width: 70%; }.loading-form { height: 600px; border-radius: var(--wx-radius-card); }.connection-error { padding: var(--wx-space-10) var(--wx-space-5); }
@media (min-width: 48rem) { h1 { font-size: var(--wx-text-2xl); }.form-card { padding: var(--wx-space-8); }.price-fields { grid-template-columns: minmax(0, 2fr) minmax(0, 1fr); }.form-actions { flex-direction: row; } }
</style>
