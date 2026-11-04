import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { previewWish } from '../api/wishes.js'
import { locale } from '../i18n.js'

export function validImportLink(raw) {
  try {
    const url = new URL(raw)
    return /^https?:$/.test(url.protocol) && !url.username && !url.password && raw.length <= 2048 && !/[\s\u0000-\u001f\u007f]/.test(raw)
  } catch { return false }
}

export function useWishImport(firstLink, fields, onApplied) {
  const status = ref('idle')
  const shopName = ref('')
  const dirty = new Set()
  const imported = new Set()
  let attempted = false
  let timer
  let request
  let version = 0
  const canImport = computed(() => validImportLink(firstLink.value.trim()))
  const loading = computed(() => status.value === 'loading')
  function markDirty(field) { dirty.add(field) }
  function cancel() {
    clearTimeout(timer)
    version++
    request?.abort()
    request = null
    if (loading.value) status.value = 'idle'
  }
  function importedImage(value) {
    if (!value || value.mime !== 'image/jpeg' || value.data_base64.length > 700000) return null
    const binary = atob(value.data_base64)
    return new File([Uint8Array.from(binary, ch => ch.charCodeAt(0))], 'wish-image.jpg', { type: 'image/jpeg' })
  }
  async function run() {
    if (!canImport.value || loading.value) return
    clearTimeout(timer)
    attempted = true
    const current = ++version
    const url = firstLink.value.trim()
    request = new AbortController()
    status.value = 'loading'
    shopName.value = ''
    try {
      const result = await previewWish(url, locale.value, request.signal)
      if (current !== version || url !== firstLink.value.trim()) return
      const image = importedImage(result.image)
      const applied = []
      if (!dirty.has('title') && result.title) { fields.title.value = result.title; applied.push('title') }
      if (!dirty.has('price') && !dirty.has('currency') && result.price && result.currency) {
        fields.price.value = result.price; fields.currency.value = result.currency
        applied.push('price', 'currency')
      }
      if (!dirty.has('image') && image) { fields.image.value = image; applied.push('image') }
      for (const field of applied) imported.add(field)
      onApplied?.(applied)
      shopName.value = result.shop_name || ''
      status.value = ['not_product', 'empty'].includes(result.status) ? result.status : applied.length ? result.status : 'kept'
    } catch (error) {
      if (current !== version) return
      if (error.status === 401) { status.value = 'session'; return }
      status.value = ['import_busy', 'rate_limited'].includes(error.code) ? 'busy' : 'failed'
    } finally { if (current === version) request = null }
  }
  watch(firstLink, () => {
    cancel()
    for (const field of imported) {
      if (!dirty.has(field)) fields[field].value = field === 'image' ? null : ''
    }
    imported.clear()
    status.value = 'idle'
    shopName.value = ''
    if (!attempted && canImport.value) timer = setTimeout(run, 800)
  }, { flush: 'sync' })
  function firstLinkBlur() { if (!attempted && canImport.value) run() }
  onBeforeUnmount(cancel)
  return { status, shopName, canImport, loading, markDirty, cancel, run, firstLinkBlur }
}
