import { onMounted, ref, watch } from 'vue'

const headingPathKey = 'wishix:profile-heading-path'

export function rememberProfileDestination(event) {
  if (event.defaultPrevented || event.button !== 0 || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return false
  try {
    sessionStorage.setItem(headingPathKey, event.currentTarget.pathname)
  } catch {
    // The link still works when storage is unavailable.
  }
  return true
}

export function usePublicProfileFocus(loading) {
  const heading = ref(null)
  onMounted(() => {
    let path
    try {
      path = sessionStorage.getItem(headingPathKey)
      sessionStorage.removeItem(headingPathKey)
    } catch { return }
    const navigation = performance.getEntriesByType('navigation')[0]
    if (path !== location.pathname || location.hash || (navigation && navigation.type !== 'navigate')) return

    function focusHeading() {
      if (document.activeElement !== document.body) return
      const target = heading.value ?? document.getElementById('main-content')
      target?.focus()
    }

    if (loading?.value) watch(loading, focusHeading, { once: true, flush: 'post' })
    else focusHeading()
  })
  return heading
}
