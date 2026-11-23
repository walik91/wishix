import { onBeforeUnmount, onMounted, ref, watchEffect } from 'vue'

export function useUnsavedWish(hasChanges) {
  const leaveDestination = ref('')
  const leaving = ref(false)

  function confirmLink(event) {
    if (leaving.value || !hasChanges.value || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    const link = event.target instanceof Element ? event.target.closest('a[href]') : null
    if (!link || link.hasAttribute('download') || (link.target && link.target.toLowerCase() !== '_self')) return
    const destination = new URL(link.href)
    if (!['http:', 'https:'].includes(destination.protocol)) return
    const current = new URL(window.location.href)
    if (destination.hash && destination.origin === current.origin && destination.pathname === current.pathname && destination.search === current.search) return
    event.preventDefault()
    leaveDestination.value = destination.href
  }

  function confirmUnload(event) {
    event.preventDefault()
    event.returnValue = true
  }

  watchEffect(onCleanup => {
    if (leaving.value || !hasChanges.value) return
    window.addEventListener('beforeunload', confirmUnload)
    onCleanup(() => window.removeEventListener('beforeunload', confirmUnload))
  }, { flush: 'sync' })

  function leave(destination) {
    leaving.value = true
    leaveDestination.value = ''
    window.location.assign(destination)
  }

  function restorePage(event) {
    // A document restored from the back/forward cache still holds the old form state.
    if (!event.persisted || (!leaving.value && !hasChanges.value)) return
    leaving.value = true
    window.location.reload()
  }

  onMounted(() => {
    document.addEventListener('click', confirmLink)
    window.addEventListener('pageshow', restorePage)
  })
  onBeforeUnmount(() => {
    document.removeEventListener('click', confirmLink)
    window.removeEventListener('pageshow', restorePage)
  })

  return { leaveDestination, leave }
}
