import { ref, watch, watchEffect } from 'vue'
import en from './locales/en.json'
import ru from './locales/ru.json'
import ro from './locales/ro.json'

const messages = { en, ru, ro }
const localeCookie = 'wishix_locale'
const cookieMaxAge = 60 * 60 * 24 * 365

function isSupportedLocale(code) {
  return Object.hasOwn(messages, code)
}

function readLocaleCookie() {
  try {
    const cookie = document.cookie.split(';').map(part => part.trim())
      .find(part => part.startsWith(`${localeCookie}=`))
    const value = cookie ? decodeURIComponent(cookie.slice(localeCookie.length + 1)) : null
    return isSupportedLocale(value) ? value : null
  } catch {
    return null
  }
}

function saveLocale(value) {
  try {
    const secure = window.location.protocol === 'https:' ? '; Secure' : ''
    document.cookie = `${localeCookie}=${encodeURIComponent(value)}; Path=/; Max-Age=${cookieMaxAge}; SameSite=Lax${secure}`
  } catch {
    // Keep the language switch usable even if the browser denies cookie access.
  }
}

function getInitialLocale() {
  const saved = readLocaleCookie()
  if (saved) return saved

  const browserLanguages = [...(navigator.languages ?? []), navigator.language]
  return browserLanguages
    .map(language => language?.toLowerCase().split('-')[0])
    .find(isSupportedLocale) ?? 'en'
}

export const locale = ref(getInitialLocale())
export const languages = Object.entries(messages).map(([code, texts]) => ({
  code,
  name: texts.languageName,
}))

export function t(key) {
  return messages[locale.value]?.[key] ?? en[key] ?? key
}

watch(locale, value => {
  if (!isSupportedLocale(value)) return
  saveLocale(value)
}, { flush: 'sync' })

watchEffect(() => {
  document.documentElement.lang = locale.value
  document.title = t('appName')
})

