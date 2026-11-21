const uuid = '[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}'
const sharedList = new RegExp(`^(/u/[A-Za-z0-9_]{3,30}/wishlist/${uuid})/[^/]+(?=/|$)`)
const secretName = /^(?:email|e-?mail|token|access_?token|refresh_?token|share_?token|code|verification_?code|auth(?:orization)?_?code|otp|password|passwd|secret|client_?secret|authorization|session(?:_?id)?|state|credential|api_?key)$/i
const emailPattern = /[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/gi
const tokenPattern = /(^|[^A-Za-z0-9_-])([A-Za-z0-9_-]{43})(?=$|[^A-Za-z0-9_-])/g

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

function redact(value) {
  return value
    .replace(emailPattern, '')
    .replace(tokenPattern, '$1')
    // Also handle labelled secrets embedded in text or a nested URL.
    .replace(/\b((?:access[_-]?token|refresh[_-]?token|share[_-]?token|verification[_-]?code|authorization[_-]?code|code|otp|token|password|secret|state)[=:])[^&#\s]+/gi, '$1')
    .replace(/\b((?:verification[_-]?)?(?:code|otp)[ _-]+)\d{4,12}\b/gi, '$1')
}

function cleanPart(raw) {
  const value = decoded(raw)
  if (value === null) return ''
  const clean = redact(value)
  return clean === value ? raw : encodeURIComponent(clean)
}

function cleanPath(path) {
  // The final segment of this route is the share token, regardless of its format.
  path = path.replace(sharedList, '$1')
  const parts = path.split('/')
  for (let i = 0; i < parts.length; i++) {
    const name = decoded(parts[i])
    if (name && secretName.test(name) && i + 1 < parts.length) {
      parts.splice(i + 1, 1)
    } else {
      parts[i] = cleanPart(parts[i])
    }
  }
  return parts.join('/') || '/'
}

function cleanQuery(search) {
  if (!search) return ''
  const parts = search.slice(1).split('&').flatMap(part => {
    const at = part.indexOf('=')
    const rawName = at < 0 ? part : part.slice(0, at)
    const name = decoded(rawName)
    if (!name || secretName.test(name)) return []
    const cleanName = cleanPart(rawName)
    if (!cleanName) return []
    if (at < 0) return [cleanName]
    const rawValue = part.slice(at + 1)
    const cleanValue = cleanPart(rawValue)
    if (rawValue && !cleanValue) return []
    return [cleanName + '=' + cleanValue]
  })
  return parts.length ? '?' + parts.join('&') : ''
}

export function analyticsLocation(location) {
  const hash = location.hash ? cleanPart(location.hash.slice(1)) : ''
  return location.origin + cleanPath(location.pathname) + cleanQuery(location.search) + (hash ? '#' + hash : '')
}
