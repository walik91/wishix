export function sharedListPath(username, sharePath) {
  if (!/^[A-Za-z0-9_]{3,30}$/.test(username || '')) return ''
  const match = /^\/([a-f0-9-]{36})(?:\/([A-Za-z0-9_-]{43}))?$/.exec(sharePath || '')
  return match ? `/u/${encodeURIComponent(username)}/wishlist${sharePath}` : ''
}

export function isNamedSharedListPath(path) {
  return /^\/u\/[A-Za-z0-9_]{3,30}\/wishlist\/[^/]+(?:\/[^/]+)?$/.test(path)
}
