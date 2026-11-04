// The fragment carries the capability without sending it in HTTP URLs or Referer.
export function sharedListPath(username, sharePath) {
  if (!/^[A-Za-z0-9_]{3,30}$/.test(username || '')) return ''
  const match = /^\/shared-list#([A-Za-z0-9_-]{43})$/.exec(sharePath || '')
  return match ? `/${encodeURIComponent(username)}/wishlist/shared#${match[1]}` : ''
}

export function isNamedSharedListPath(path) {
  return /^\/[A-Za-z0-9_]{3,30}\/wishlist\/shared$/.test(path)
}
