import { onUnmounted, ref } from 'vue'
import { t } from '../i18n.js'

export function authError(error) {
  const key = `auth.error.${error.code || 'unavailable'}`
  const text = t(key)
  return text === key ? t('auth.error.unavailable') : text
}

export function useAuthForm() {
  const busy = ref(false)
  const error = ref('')
  const remaining = ref(0)
  let deadline = 0
  const timer = window.setInterval(() => {
    remaining.value = Math.max(0, Math.ceil((deadline - Date.now()) / 1000))
  }, 500)
  onUnmounted(() => window.clearInterval(timer))

  function cooldown(seconds = 60) {
    deadline = Date.now() + seconds * 1000
    remaining.value = seconds
  }

  async function run(action) {
    if (busy.value) return
    error.value = ''
    busy.value = true
    try { await action() }
    catch (failure) { error.value = authError(failure) }
    finally { busy.value = false }
  }

  return { busy, error, remaining, cooldown, run }
}
