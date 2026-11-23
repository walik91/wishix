import { onBeforeUnmount, ref, watch } from 'vue'
import { previewWish } from '../api/wishes.js'
import { locale } from '../i18n.js'

export function validImportLink(raw) {
  try {
    const url = new URL(raw)
    return /^https?:$/.test(url.protocol) && !url.username && !url.password && raw.length <= 2048 && !/[\s\u0000-\u001f\u007f]/.test(raw)
  } catch { return false }
}

export function useWishImport(links, fields, onApplied, onResult) {
  const activeId = ref(null)
  const dirty = new Set()
  const imported = new Set()
  let sourceId = null
  let attempted = false
  let timer, request, activeURL
  let version = 0

  function markDirty(field) { dirty.add(field) }
  function cancel() {
    clearTimeout(timer)
    version++
    request?.abort()
    request = null
    activeId.value = null
    activeURL = ''
  }
  function clearImported() {
    for (const field of imported) {
      if (!dirty.has(field)) fields[field].value = field === 'image' ? null : ''
    }
    imported.clear()
    sourceId = null
  }
  function importedImage(value) {
    if (!value || value.mime !== 'image/jpeg' || value.data_base64.length > 700000) return null
    const binary = atob(value.data_base64)
    return new File([Uint8Array.from(binary, ch => ch.charCodeAt(0))], 'wish-image.jpg', { type: 'image/jpeg' })
  }
  async function run(id) {
    const link = links.value.find(item => item.id === id)
    const url = link?.value.trim() || ''
    if (!validImportLink(url)) return
    if (activeId.value === id && activeURL === url) return
    cancel()
    attempted = true
    const current = version
    activeId.value = id
    activeURL = url
    request = new AbortController()
    try {
      const result = await previewWish(url, locale.value, request.signal)
      if (current !== version || links.value.find(item => item.id === id)?.value.trim() !== url) return
      if (result.status === 'not_product') { onResult?.('not_product'); return }
      const image = importedImage(result.image)
      const hasData = result.title || result.description || (result.price && result.currency) || image
      if (!hasData) { onResult?.('failed'); return }
      clearImported()
      const applied = []
      if (!dirty.has('title') && result.title) { fields.title.value = result.title; applied.push('title') }
      if (!dirty.has('description') && result.description) { fields.description.value = result.description; applied.push('description') }
      if (!dirty.has('price') && !dirty.has('currency') && result.price && result.currency) {
        fields.price.value = result.price; fields.currency.value = result.currency
        applied.push('price', 'currency')
      }
      if (!dirty.has('image') && image) { fields.image.value = image; applied.push('image') }
      for (const field of applied) imported.add(field)
      sourceId = applied.length ? id : null
      onApplied?.(applied)
      onResult?.(applied.length ? result.status : 'kept')
    } catch (error) {
      if (current !== version) return
      onResult?.(error.status === 401 ? 'session' : ['import_busy', 'rate_limited'].includes(error.code) ? 'busy' : 'failed')
    } finally {
      if (current === version) { request = null; activeId.value = null; activeURL = '' }
    }
  }
  watch(() => links.value.map(({ id, value }) => ({ id, value })), (current, previous) => {
    const changed = id => id != null && current.find(link => link.id === id)?.value !== previous.find(link => link.id === id)?.value
    if (changed(activeId.value)) cancel()
    if (changed(sourceId)) clearImported()
    const firstChanged = current[0]?.id !== previous[0]?.id || current[0]?.value !== previous[0]?.value
    if (firstChanged && !attempted) {
      clearTimeout(timer)
      if (validImportLink(current[0]?.value.trim() || '')) timer = setTimeout(() => run(current[0].id), 800)
    }
  }, { flush: 'sync' })
  function firstLinkBlur() {
    if (!attempted && links.value[0]) run(links.value[0].id)
  }
  onBeforeUnmount(cancel)
  return { activeId, markDirty, cancel, run, firstLinkBlur }
}
