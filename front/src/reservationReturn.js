const storageKey = 'wishix_reservation_return'
const listPath = /^\/u\/[A-Za-z0-9_]{3,30}\/wishlist\/[a-fA-F0-9-]{36}(?:\/[A-Za-z0-9_-]{43})?$/
const collectionPath = /^\/u\/[A-Za-z0-9_]{3,30}\/wishes$/
const wishIDPattern = /^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$/

function validPath(path) {
  return typeof path === 'string' && (listPath.test(path) || collectionPath.test(path))
}

export function clearReservationReturn() {
  try { sessionStorage.removeItem(storageKey) } catch { /* Storage may be disabled. */ }
}

// Keep share capabilities out of login URLs and server logs. The return target
// is local to this tab, short-lived, single-use and checked again when read.
export function rememberReservationReturn(path, wishID) {
  if (!validPath(path)) return
  try { sessionStorage.setItem(storageKey, JSON.stringify({ path, wishID: listPath.test(path) && wishIDPattern.test(wishID) ? wishID : '', expires: Date.now() + 15 * 60 * 1000 })) } catch { /* Storage may be disabled. */ }
}

export function consumeReservationReturn() {
  try {
    const value = sessionStorage.getItem(storageKey)
    sessionStorage.removeItem(storageKey)
    const saved = JSON.parse(value)
    if (saved && Number.isFinite(saved.expires) && saved.expires > Date.now() && validPath(saved.path)) {
      return saved.path + (listPath.test(saved.path) && wishIDPattern.test(saved.wishID) ? `#reserve-${saved.wishID}` : '')
    }
  } catch { /* Invalid or unavailable storage falls back to the account page. */ }
  return '/my'
}
