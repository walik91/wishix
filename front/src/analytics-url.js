// Report actual application routes, but never pass unknown route segments as data.
const uuid = '[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}'
const username = '[A-Za-z0-9_]{3,30}'
const sharedList = new RegExp(`^(/u/${username}/wishlist/${uuid})(?:/[^/]+)?/?$`)
const dynamicRoutes = [
  new RegExp(`^/u/${username}(?:/(?:wishes|followers|following))?/?$`),
  new RegExp(`^/my/lists/${uuid}/?$`),
  new RegExp(`^/my/wishes/${uuid}/edit/?$`),
]
const staticRoutes = new Set(['/', '/login', '/register', '/forgot-password', '/google/finish', '/facebook/finish', '/my', '/my/profile', '/my/following', '/my/search', '/my/wishes', '/my/wishes/new', '/my/lists', '/my/lists/new', '/status'])

function decoded(value) {
  let result = value.replace(/\+/g, ' ')
  try {
    for (let i = 0; i < 3; i++) {
      const next = decodeURIComponent(result)
      if (next === result) break
      result = next
    }
  } catch { return null }
  return result
}

function safeQuery(search) {
  const sensitiveName = /(?:email|e-mail|mail|token|code|pass(?:word)?|secret|authorization|auth|session|state|otp|credential|api.?key)/i
  const email = /[^\s@]+@[^\s@]+\.[^\s@]+/i
  const safe = search.replace(/^\?/, '').split('&').filter(part => {
    if (!part) return false
    const [rawName, ...rawValue] = part.split('=')
    const name = decoded(rawName), value = decoded(rawValue.join('='))
    if (!name || value === null || sensitiveName.test(name) || email.test(name) || email.test(value)) return false
    // Nested URLs and free-text values can conceal sensitive parameters.
    if (sensitiveName.test(value)) return false
    if (/(?:^|[^A-Za-z0-9_-])[A-Za-z0-9_-]{43}(?:$|[^A-Za-z0-9_-])/.test(value)) return false
    return true
  })
  return safe.length ? '?' + safe.join('&') : ''
}

export function analyticsLocation(location) {
  const path = location.pathname
  const match = sharedList.exec(path)
  const cleanPath = match ? match[1] : staticRoutes.has(path.replace(/\/$/, '') || '/') || dynamicRoutes.some(route => route.test(path)) ? path : '/other'
  // Only the known non-secret wish anchor is accepted; arbitrary fragments may be tokens.
  const hash = new RegExp(`^#reserve-${uuid}$`).test(location.hash) ? location.hash : ''
  return location.origin + cleanPath + safeQuery(location.search) + hash
}
