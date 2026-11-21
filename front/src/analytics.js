import { ref } from 'vue'
import { analyticsLocation } from './analytics-url.js'

// This is public configuration, not a credential. A saved denial disables Google.
export const measurementId = (import.meta.env.VITE_GA_MEASUREMENT_ID || '').trim()
export const analyticsConfigured = /^G-[A-Z0-9]+$/.test(measurementId)
const preferenceKey = 'wishix_analytics_consent_v1'
function readPreference() {
  try {
    const value = localStorage.getItem(preferenceKey)
    return value === 'granted' || value === 'denied' ? value : null
  } catch { return null }
}
export const analyticsConsent = ref(readPreference())
let started = false

// Titles remain categories so user-controlled page titles cannot enter GA.
export function pageCategory(path) {
  if (path === '/') return 'home'
  if (/^\/u\/[^/]+\/wishlist\//.test(path)) return 'shared_list'
  if (/^\/u\/[^/]+\/wishes\/?$/.test(path)) return 'public_collection'
  if (/^\/u\/[^/]+\/(followers|following)\/?$/.test(path)) return 'public_people'
  if (/^\/u\/[^/]+\/?$/.test(path)) return 'public_profile'
  if (/^\/my\/wishes\/[^/]+\/edit\/?$/.test(path)) return 'edit_wish'
  if (/^\/my\/lists\/new\/?$/.test(path)) return 'new_list'
  if (/^\/my\/lists\/[^/]+\/?$/.test(path)) return 'my_list'
  const pages = { '/login':'login', '/register':'register', '/forgot-password':'recovery', '/google/finish':'social_finish', '/facebook/finish':'social_finish', '/my':'dashboard', '/my/profile':'settings', '/my/following':'following', '/my/search':'search', '/my/wishes':'wishes', '/my/wishes/new':'new_wish', '/my/lists':'lists', '/status':'status' }
  return pages[path.replace(/\/+$/, '')] || 'other'
}
function safeReferrer() {
  try {
    const url = new URL(document.referrer)
    if (!['http:', 'https:'].includes(url.protocol) || url.origin === window.location.origin) return ''
    return url.origin + '/'
  } catch { return '' }
}
function context() {
  const category = pageCategory(window.location.pathname)
  return { page_location: analyticsLocation(window.location), page_title: category, page_referrer: safeReferrer() }
}
export function startAnalytics() {
  // Reservation management links contain secret tokens; no vendor script on that page.
  if (!analyticsConfigured || analyticsConsent.value === 'denied' || started || /^\/reservation\/?$/.test(window.location.pathname)) return
  started = true
  window.dataLayer = window.dataLayer || []
  window.gtag = function () { window.dataLayer.push(arguments) }
  window.gtag('set', { ...context(), send_page_view: false })
  window.gtag('js', new Date())
  window.gtag('config', measurementId, { ...context(), send_page_view: false, allow_google_signals: false, allow_ad_personalization_signals: false, transport_type: 'beacon' })
  window.gtag('event', 'page_view', context())
  const script = document.createElement('script')
  script.async = true
  script.referrerPolicy = 'no-referrer'
  script.src = 'https://www.googletagmanager.com/gtag/js?id=' + encodeURIComponent(measurementId)
  document.head.appendChild(script)
}
const allowedEvents = new Set(['sign_up_started', 'sign_up_completed', 'wish_created', 'list_created', 'share_link_copied', 'reservation_started', 'reservation_completed', 'shop_link_opened', 'people_search_completed'])
export function trackEvent(name) {
  if (!started || analyticsConsent.value === 'denied' || !allowedEvents.has(name)) return
  window.gtag('event', name, context())
}
function clearAnalyticsCookies() {
  const host = window.location.hostname.split('.')
  const domains = ['', ...host.map((_, index) => '; Domain=' + host.slice(index).join('.'))]
  for (const item of document.cookie.split(';')) {
    const name = item.trim().split('=')[0]
    if (!/^_ga(?:_|$)/.test(name)) continue
    for (const domain of domains) document.cookie = name + '=; Max-Age=0; Path=/' + domain + '; SameSite=Lax'
  }
}
window.addEventListener('storage', event => {
  if (event.key !== preferenceKey) return
  analyticsConsent.value = ['granted', 'denied'].includes(event.newValue) ? event.newValue : null
  if (analyticsConsent.value !== 'denied') startAnalytics()
  else if (started) {
    window['ga-disable-' + measurementId] = true
    clearAnalyticsCookies()
    window.location.reload()
  }
})
export function setAnalyticsConsent(value) {
  const next = value ? 'granted' : 'denied'
  let persisted = false
  try { localStorage.setItem(preferenceKey, next); persisted = true } catch { /* Preference lasts for this page only. */ }
  const wasStarted = started
  analyticsConsent.value = next
  if (value) {
    window['ga-disable-' + measurementId] = false
    startAnalytics()
  } else if (wasStarted) {
    window['ga-disable-' + measurementId] = true
    clearAnalyticsCookies()
    if (persisted) window.location.reload()
  }
}
